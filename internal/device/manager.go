package device

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"modbussim/internal/config"
	"modbussim/internal/register"
)

// DeviceFile is the on-disk YAML structure for a device.
type DeviceFile struct {
	Version     string              `yaml:"version"`
	ID          string              `yaml:"id"`
	Name        string              `yaml:"name"`
	Description string              `yaml:"description,omitempty"`
	ModbusAddr  string              `yaml:"modbus_addr"`
	Registers   []register.Register `yaml:"registers,omitempty"`
}

// VersionInfo describes a saved device snapshot.
type VersionInfo struct {
	Path     string    `json:"path"`
	Filename string    `json:"filename"`
	SavedAt  time.Time `json:"saved_at"`
	Name     string    `json:"name"`
	RegCount int       `json:"reg_count"`
}

// CreateRequest is the payload for creating a new device.
type CreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ModbusAddr  string `json:"modbus_addr,omitempty"`
}

// UpdateRequest is the payload for updating device metadata.
type UpdateRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	ModbusAddr  string `json:"modbus_addr,omitempty"`
}

// Manager owns the full device lifecycle: load from disk, CRUD, start/stop.
type Manager struct {
	mu          sync.RWMutex
	devicesDir  string
	versionsDir string
	devices     map[string]*Device
}

// NewManager creates a Manager for the given directories.
func NewManager(devicesDir, versionsDir string) *Manager {
	return &Manager{
		devicesDir:  devicesDir,
		versionsDir: versionsDir,
		devices:     make(map[string]*Device),
	}
}

// LoadAll reads every *.yaml in devicesDir and loads each as a stopped device.
func (m *Manager) LoadAll() error {
	if err := os.MkdirAll(m.devicesDir, 0755); err != nil {
		return fmt.Errorf("mkdir devices %s: %w", m.devicesDir, err)
	}

	entries, err := os.ReadDir(m.devicesDir)
	if err != nil {
		return fmt.Errorf("read devices dir %s: %w", m.devicesDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(m.devicesDir, entry.Name())
		if err := m.loadFile(path); err != nil {
			slog.Warn("device: skip file", "path", path, "err", err)
		}
	}
	return nil
}

func (m *Manager) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var df DeviceFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		return err
	}

	if df.ID == "" {
		df.ID = strings.TrimSuffix(filepath.Base(path), ".yaml")
	}
	if df.Name == "" {
		df.Name = df.ID
	}
	if df.ModbusAddr == "" {
		df.ModbusAddr = ":5020"
	}

	d := New(df.ID, df.Name, df.Description, df.ModbusAddr)
	for _, r := range df.Registers {
		if _, err := d.engine.Add(r); err != nil {
			slog.Warn("device: skip register", "device", df.ID, "reg", r.ID, "err", err)
		}
	}

	m.mu.Lock()
	m.devices[df.ID] = d
	m.mu.Unlock()
	return nil
}

// Migrate handles the startup migration:
//   - If devices/ is non-empty, it is a no-op.
//   - If legacyCfgPath is provided, imports it as the first device.
//   - Otherwise, creates a "default" device with example registers.
func (m *Manager) Migrate(legacyCfgPath string) error {
	m.mu.RLock()
	count := len(m.devices)
	m.mu.RUnlock()

	if count > 0 {
		return nil
	}

	var df DeviceFile
	if legacyCfgPath != "" {
		cfg, err := config.Load(legacyCfgPath)
		if err != nil {
			return fmt.Errorf("migrate: load %s: %w", legacyCfgPath, err)
		}
		id := sanitizeID(cfg.Name)
		if id == "" {
			id = "default"
		}
		df = DeviceFile{
			Version:     "1",
			ID:          id,
			Name:        cfg.Name,
			Description: cfg.Description,
			ModbusAddr:  cfg.ModbusAddr,
			Registers:   cfg.Registers,
		}
	} else {
		cfg := config.Default()
		df = DeviceFile{
			Version:    "1",
			ID:         "default",
			Name:       "Default",
			ModbusAddr: cfg.ModbusAddr,
			Registers:  cfg.Registers,
		}
	}

	return m.persistAndLoad(df)
}

// persistAndLoad writes the DeviceFile to disk and loads it into the manager.
func (m *Manager) persistAndLoad(df DeviceFile) error {
	if err := os.MkdirAll(m.devicesDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(m.devicesDir, df.ID+".yaml")
	data, err := yaml.Marshal(df)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	return m.loadFile(path)
}

// StartAll starts all loaded devices, logging but not aborting on failure.
func (m *Manager) StartAll(ctx context.Context) {
	m.mu.RLock()
	devs := make([]*Device, 0, len(m.devices))
	for _, d := range m.devices {
		devs = append(devs, d)
	}
	m.mu.RUnlock()

	for _, d := range devs {
		if err := d.Start(ctx); err != nil {
			slog.Error("device: start failed", "id", d.ID(), "err", err)
		} else {
			slog.Info("device: started", "id", d.ID(), "modbus_addr", d.ModbusAddr())
		}
	}
}

// List returns a stable-sorted snapshot of all device infos.
func (m *Manager) List() []Info {
	m.mu.RLock()
	infos := make([]Info, 0, len(m.devices))
	for _, d := range m.devices {
		infos = append(infos, d.Info())
	}
	m.mu.RUnlock()

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].ID < infos[j].ID
	})
	return infos
}

// Get returns the Device for a given id.
func (m *Manager) Get(id string) (*Device, bool) {
	m.mu.RLock()
	d, ok := m.devices[id]
	m.mu.RUnlock()
	return d, ok
}

// Create persists a new device and registers it in the manager.
func (m *Manager) Create(req CreateRequest) (*Device, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("name required")
	}

	id := sanitizeID(req.Name)
	if id == "" {
		id = "device"
	}

	m.mu.Lock()
	base := id
	for i := 2; ; i++ {
		if _, exists := m.devices[id]; !exists {
			break
		}
		id = fmt.Sprintf("%s_%d", base, i)
	}
	m.mu.Unlock()

	addr := req.ModbusAddr
	if addr == "" {
		addr = m.nextPort()
	}

	df := DeviceFile{
		Version:     "1",
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		ModbusAddr:  addr,
	}

	if err := m.persistAndLoad(df); err != nil {
		return nil, err
	}

	d, _ := m.Get(id)
	return d, nil
}

// Update changes a device's name, description, or Modbus address.
// If the address changes while the device is running, it is stopped first.
func (m *Manager) Update(id string, req UpdateRequest) error {
	d, ok := m.Get(id)
	if !ok {
		return fmt.Errorf("device %q not found", id)
	}

	addrChanged := req.ModbusAddr != "" && req.ModbusAddr != d.ModbusAddr()
	if addrChanged && d.GetStatus() == StatusRunning {
		d.Stop()
	}

	if req.Name != "" {
		d.SetName(req.Name)
	}
	if req.Description != "" {
		d.SetDescription(req.Description)
	}
	if req.ModbusAddr != "" {
		d.SetModbusAddr(req.ModbusAddr)
	}

	return m.persistDevice(d)
}

// Delete stops the device, removes its YAML file, and deregisters it.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	d, ok := m.devices[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("device %q not found", id)
	}
	delete(m.devices, id)
	m.mu.Unlock()

	d.Stop()
	os.Remove(filepath.Join(m.devicesDir, id+".yaml"))
	return nil
}

// SaveVersion writes a timestamped snapshot of the device to configs/{id}/.
func (m *Manager) SaveVersion(deviceID string) (string, error) {
	d, ok := m.Get(deviceID)
	if !ok {
		return "", fmt.Errorf("device %q not found", deviceID)
	}

	versDir := filepath.Join(m.versionsDir, deviceID)
	if err := os.MkdirAll(versDir, 0755); err != nil {
		return "", err
	}

	name := sanitizeID(d.Name())
	if name == "" {
		name = "snapshot"
	}
	filename := fmt.Sprintf("%s_%s.yaml", time.Now().Format("20060102_150405"), name)
	path := filepath.Join(versDir, filename)

	df := DeviceFile{
		Version:     "1",
		ID:          d.ID(),
		Name:        d.Name(),
		Description: d.Description(),
		ModbusAddr:  d.ModbusAddr(),
		Registers:   d.Engine().List(),
	}

	data, err := yaml.Marshal(df)
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, data, 0644)
}

// ListVersions returns saved snapshots for a device, newest first.
func (m *Manager) ListVersions(deviceID string) ([]VersionInfo, error) {
	if _, ok := m.Get(deviceID); !ok {
		return nil, fmt.Errorf("device %q not found", deviceID)
	}

	versDir := filepath.Join(m.versionsDir, deviceID)
	if err := os.MkdirAll(versDir, 0755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(versDir)
	if err != nil {
		return nil, err
	}

	versions := make([]VersionInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(versDir, entry.Name())
		info, _ := entry.Info()

		v := VersionInfo{
			Path:     path,
			Filename: entry.Name(),
			SavedAt:  info.ModTime(),
		}
		if df, err := readDeviceFile(path); err == nil {
			v.Name = df.Name
			v.RegCount = len(df.Registers)
		}
		versions = append(versions, v)
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].SavedAt.After(versions[j].SavedAt)
	})
	return versions, nil
}

// LoadVersion replaces the device's registers from a saved snapshot.
func (m *Manager) LoadVersion(deviceID, path string) error {
	d, ok := m.Get(deviceID)
	if !ok {
		return fmt.Errorf("device %q not found", deviceID)
	}

	df, err := readDeviceFile(path)
	if err != nil {
		return err
	}

	eng := d.Engine()
	for _, r := range eng.List() {
		eng.Remove(r.ID)
	}
	for _, r := range df.Registers {
		if _, err := eng.Add(r); err != nil {
			slog.Warn("device: load version skip register", "device", deviceID, "reg", r.ID, "err", err)
		}
	}
	return nil
}

// ExportDevice returns the current device config as YAML bytes.
func (m *Manager) ExportDevice(deviceID string) ([]byte, error) {
	d, ok := m.Get(deviceID)
	if !ok {
		return nil, fmt.Errorf("device %q not found", deviceID)
	}

	df := DeviceFile{
		Version:     "1",
		ID:          d.ID(),
		Name:        d.Name(),
		Description: d.Description(),
		ModbusAddr:  d.ModbusAddr(),
		Registers:   d.Engine().List(),
	}
	return yaml.Marshal(df)
}

// ImportDevice replaces the device's registers from raw YAML bytes.
func (m *Manager) ImportDevice(deviceID string, data []byte) error {
	d, ok := m.Get(deviceID)
	if !ok {
		return fmt.Errorf("device %q not found", deviceID)
	}

	var df DeviceFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		return err
	}

	eng := d.Engine()
	for _, r := range eng.List() {
		eng.Remove(r.ID)
	}
	for _, r := range df.Registers {
		if _, err := eng.Add(r); err != nil {
			slog.Warn("device: import skip register", "device", deviceID, "reg", r.ID, "err", err)
		}
	}
	return nil
}

// persistDevice writes the current device state to its YAML file.
func (m *Manager) persistDevice(d *Device) error {
	df := DeviceFile{
		Version:     "1",
		ID:          d.ID(),
		Name:        d.Name(),
		Description: d.Description(),
		ModbusAddr:  d.ModbusAddr(),
		Registers:   d.Engine().List(),
	}
	data, err := yaml.Marshal(df)
	if err != nil {
		return err
	}
	path := filepath.Join(m.devicesDir, d.ID()+".yaml")
	return os.WriteFile(path, data, 0644)
}

// nextPort returns the next available Modbus port starting from 5020.
func (m *Manager) nextPort() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	next := 5020
	for _, d := range m.devices {
		addr := d.ModbusAddr()
		if strings.HasPrefix(addr, ":") {
			if p, err := strconv.Atoi(addr[1:]); err == nil && p >= next {
				next = p + 1
			}
		}
	}
	return fmt.Sprintf(":%d", next)
}

var reNonAlpha = regexp.MustCompile(`[^a-z0-9_-]+`)

func sanitizeID(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "_")
	s = reNonAlpha.ReplaceAllString(s, "")
	return strings.Trim(s, "_-")
}

func readDeviceFile(path string) (*DeviceFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var df DeviceFile
	if err := yaml.Unmarshal(data, &df); err != nil {
		return nil, err
	}
	return &df, nil
}
