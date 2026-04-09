package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"modbussim/internal/register"
)

// Config is the top-level configuration structure.
type Config struct {
	Version     string              `yaml:"version"`
	Name        string              `yaml:"name"`
	Description string              `yaml:"description,omitempty"`
	ModbusAddr  string              `yaml:"modbus_addr"`
	AdminAddr   string              `yaml:"admin_addr"`
	Registers   []register.Register `yaml:"registers"`
}

// VersionInfo describes a saved config snapshot.
type VersionInfo struct {
	Path     string    `json:"path"`
	Filename string    `json:"filename"`
	SavedAt  time.Time `json:"saved_at"`
	Name     string    `json:"name"`
	RegCount int       `json:"reg_count"`
}

// Default returns a Config with example registers.
func Default() *Config {
	return &Config{
		Version:    "1",
		Name:       "default",
		ModbusAddr: ":5020",
		AdminAddr:  ":7070",
		Registers: []register.Register{
			{
				ID:       "temperature",
				Name:     "Temperature",
				Address:  0,
				DataType: register.TypeFloat32,
				Unit:     "°C",
				Signal: register.Signal{
					Kind:      register.SignalSine,
					Amplitude: 5,
					Period:    30,
					Offset:    25,
					Min:       15,
					Max:       35,
				},
			},
			{
				ID:       "pressure",
				Name:     "Pressure",
				Address:  2,
				DataType: register.TypeFloat32,
				Unit:     "bar",
				Signal: register.Signal{
					Kind: register.SignalRamp,
					Rate: 0.5,
					Min:  1.0,
					Max:  10.0,
				},
			},
			{
				ID:       "humidity",
				Name:     "Humidity",
				Address:  4,
				DataType: register.TypeFloat32,
				Unit:     "%",
				Signal: register.Signal{
					Kind:        register.SignalRandomWalk,
					StepMaxWalk: 1.0,
					Min:         40,
					Max:         80,
				},
			},
		},
	}
}

// Load reads a Config from a YAML file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &cfg, nil
}

// Save writes cfg to dir as a timestamped YAML file and returns the path.
func Save(cfg *Config, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	now := time.Now()
	name := cfg.Name
	if name == "" {
		name = "config"
	}
	// Sanitize name for filename.
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, name)

	filename := fmt.Sprintf("%s_%s.yaml", now.Format("20060102_150405"), name)
	path := filepath.Join(dir, filename)

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("write config %s: %w", path, err)
	}
	return path, nil
}

// ListVersions returns all saved config versions in dir, newest first.
func ListVersions(dir string) ([]VersionInfo, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	var versions []VersionInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		cfg, err := Load(path)
		if err != nil {
			// Still include but with minimal info.
			versions = append(versions, VersionInfo{
				Path:     path,
				Filename: entry.Name(),
				SavedAt:  info.ModTime(),
			})
			continue
		}

		versions = append(versions, VersionInfo{
			Path:     path,
			Filename: entry.Name(),
			SavedAt:  info.ModTime(),
			Name:     cfg.Name,
			RegCount: len(cfg.Registers),
		})
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].SavedAt.After(versions[j].SavedAt)
	})

	return versions, nil
}

// Export marshals cfg to YAML bytes.
func Export(cfg *Config) ([]byte, error) {
	return yaml.Marshal(cfg)
}

// Import parses YAML bytes into a Config.
func Import(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}
