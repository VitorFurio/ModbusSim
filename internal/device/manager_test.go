package device

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"modbussim/internal/register"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	devDir, _ := os.MkdirTemp("", "devices_*")
	verDir, _ := os.MkdirTemp("", "versions_*")
	t.Cleanup(func() {
		os.RemoveAll(devDir)
		os.RemoveAll(verDir)
	})
	return NewManager(devDir, verDir)
}

// ─── LoadAll ─────────────────────────────────────────────────────────────────

func TestLoadAllEmpty(t *testing.T) {
	m := newTestManager(t)
	if err := m.LoadAll(); err != nil {
		t.Fatalf("LoadAll on empty dir: %v", err)
	}
	if len(m.List()) != 0 {
		t.Errorf("expected 0 devices, got %d", len(m.List()))
	}
}

func TestLoadAllWithFile(t *testing.T) {
	m := newTestManager(t)

	yaml := `version: "1"
id: sensor
name: Sensor Hub
modbus_addr: ":5020"
registers: []
`
	os.WriteFile(filepath.Join(m.devicesDir, "sensor.yaml"), []byte(yaml), 0644)

	if err := m.LoadAll(); err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	devs := m.List()
	if len(devs) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devs))
	}
	if devs[0].ID != "sensor" {
		t.Errorf("ID = %q, want %q", devs[0].ID, "sensor")
	}
}

func TestLoadAllSkipsBadFile(t *testing.T) {
	m := newTestManager(t)
	os.WriteFile(filepath.Join(m.devicesDir, "bad.yaml"), []byte("key: [unclosed"), 0644)

	if err := m.LoadAll(); err != nil {
		t.Fatalf("LoadAll should not fail on bad file: %v", err)
	}
	// bad file is skipped, manager is empty
	if len(m.List()) != 0 {
		t.Errorf("expected 0 devices after bad file, got %d", len(m.List()))
	}
}

// ─── Migrate ─────────────────────────────────────────────────────────────────

func TestMigrateCreatesDefault(t *testing.T) {
	m := newTestManager(t)
	if err := m.Migrate(""); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	devs := m.List()
	if len(devs) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devs))
	}
	if devs[0].ID != "default" {
		t.Errorf("ID = %q, want default", devs[0].ID)
	}
	// YAML file should be persisted
	if _, err := os.Stat(filepath.Join(m.devicesDir, "default.yaml")); err != nil {
		t.Error("default.yaml not found on disk")
	}
}

func TestMigrateNoOpWhenDevicesExist(t *testing.T) {
	m := newTestManager(t)
	m.Create(CreateRequest{Name: "Existing"})

	if err := m.Migrate(""); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if len(m.List()) != 1 {
		t.Errorf("expected 1 device, got %d", len(m.List()))
	}
}

// ─── Create ──────────────────────────────────────────────────────────────────

func TestCreate(t *testing.T) {
	m := newTestManager(t)
	d, err := m.Create(CreateRequest{Name: "My Device"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if d.Name() != "My Device" {
		t.Errorf("Name = %q, want %q", d.Name(), "My Device")
	}
	if d.ID() == "" {
		t.Error("ID should not be empty")
	}
	// File should be on disk
	if _, err := os.Stat(filepath.Join(m.devicesDir, d.ID()+".yaml")); err != nil {
		t.Errorf("device file not found: %v", err)
	}
}

func TestCreateEmptyNameFails(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.Create(CreateRequest{Name: ""}); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCreateAutoPort(t *testing.T) {
	m := newTestManager(t)
	d1, _ := m.Create(CreateRequest{Name: "D1"})
	d2, _ := m.Create(CreateRequest{Name: "D2"})

	p1 := d1.ModbusAddr()
	p2 := d2.ModbusAddr()
	if p1 == p2 {
		t.Errorf("auto ports should differ, both got %q", p1)
	}
}

func TestCreateManualPort(t *testing.T) {
	m := newTestManager(t)
	d, _ := m.Create(CreateRequest{Name: "D1", ModbusAddr: ":9999"})
	if d.ModbusAddr() != ":9999" {
		t.Errorf("ModbusAddr = %q, want :9999", d.ModbusAddr())
	}
}

func TestCreateDuplicateIDSuffix(t *testing.T) {
	m := newTestManager(t)
	d1, _ := m.Create(CreateRequest{Name: "sensor"})
	d2, _ := m.Create(CreateRequest{Name: "sensor"})
	if d1.ID() == d2.ID() {
		t.Errorf("duplicate IDs: %q", d1.ID())
	}
}

// ─── Get ─────────────────────────────────────────────────────────────────────

func TestGetExisting(t *testing.T) {
	m := newTestManager(t)
	m.Create(CreateRequest{Name: "Dev"})
	devs := m.List()
	id := devs[0].ID

	d, ok := m.Get(id)
	if !ok {
		t.Fatal("Get returned false for existing device")
	}
	if d.ID() != id {
		t.Errorf("Get ID = %q, want %q", d.ID(), id)
	}
}

func TestGetMissing(t *testing.T) {
	m := newTestManager(t)
	if _, ok := m.Get("ghost"); ok {
		t.Fatal("Get returned true for missing device")
	}
}

// ─── Update ──────────────────────────────────────────────────────────────────

func TestUpdate(t *testing.T) {
	m := newTestManager(t)
	d, _ := m.Create(CreateRequest{Name: "Old"})
	id := d.ID()

	if err := m.Update(id, UpdateRequest{Name: "New", Description: "desc"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ := m.Get(id)
	if got.Name() != "New" {
		t.Errorf("Name = %q, want New", got.Name())
	}
}

func TestUpdateNotFound(t *testing.T) {
	m := newTestManager(t)
	if err := m.Update("ghost", UpdateRequest{Name: "X"}); err == nil {
		t.Fatal("expected error updating missing device")
	}
}

// ─── Delete ──────────────────────────────────────────────────────────────────

func TestDelete(t *testing.T) {
	m := newTestManager(t)
	d, _ := m.Create(CreateRequest{Name: "Dev"})
	id := d.ID()

	if err := m.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := m.Get(id); ok {
		t.Error("device still found after Delete")
	}
	if _, err := os.Stat(filepath.Join(m.devicesDir, id+".yaml")); !os.IsNotExist(err) {
		t.Error("device YAML file should be removed after Delete")
	}
}

func TestDeleteNotFound(t *testing.T) {
	m := newTestManager(t)
	if err := m.Delete("ghost"); err == nil {
		t.Fatal("expected error deleting missing device")
	}
}

// ─── Versions ────────────────────────────────────────────────────────────────

func TestSaveAndListVersions(t *testing.T) {
	m := newTestManager(t)
	d, _ := m.Create(CreateRequest{Name: "Dev"})
	id := d.ID()

	path, err := m.SaveVersion(id)
	if err != nil {
		t.Fatalf("SaveVersion: %v", err)
	}
	if path == "" {
		t.Error("SaveVersion returned empty path")
	}

	versions, err := m.ListVersions(id)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Errorf("expected 1 version, got %d", len(versions))
	}
}

func TestLoadVersion(t *testing.T) {
	m := newTestManager(t)
	d, _ := m.Create(CreateRequest{Name: "Dev"})
	id := d.ID()

	// Add a register before saving
	d.Engine().Add(register.Register{
		ID: "r1", Name: "R1", Address: 0, DataType: register.TypeUint16,
		Signal: register.Signal{Kind: register.SignalConstant, Value: 42},
	})
	path, _ := m.SaveVersion(id)

	// Remove the register
	d.Engine().Remove("r1")
	if len(d.Engine().List()) != 0 {
		t.Fatal("expected empty engine after remove")
	}

	// Load version back
	if err := m.LoadVersion(id, path); err != nil {
		t.Fatalf("LoadVersion: %v", err)
	}
	regs := d.Engine().List()
	if len(regs) != 1 || regs[0].ID != "r1" {
		t.Errorf("LoadVersion: expected r1, got %v", regs)
	}
}

// ─── Export / Import ─────────────────────────────────────────────────────────

func TestExportImport(t *testing.T) {
	m := newTestManager(t)
	src, _ := m.Create(CreateRequest{Name: "Src"})
	src.Engine().Add(register.Register{
		ID: "r1", Name: "R1", Address: 0, DataType: register.TypeUint16,
		Signal: register.Signal{Kind: register.SignalConstant, Value: 7},
	})

	data, err := m.ExportDevice(src.ID())
	if err != nil {
		t.Fatalf("ExportDevice: %v", err)
	}

	dst, _ := m.Create(CreateRequest{Name: "Dst"})
	if err := m.ImportDevice(dst.ID(), data); err != nil {
		t.Fatalf("ImportDevice: %v", err)
	}
	regs := dst.Engine().List()
	if len(regs) != 1 || regs[0].ID != "r1" {
		t.Errorf("ImportDevice: expected r1, got %v", regs)
	}
}

// ─── sanitizeID ──────────────────────────────────────────────────────────────

func TestSanitizeID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"My Device", "my_device"},
		{"PLC 300!", "plc_300"},
		{"  spaces  ", "spaces"},
		{"", ""},
		{"already_ok", "already_ok"},
		{"CamelCase", "camelcase"},
	}
	for _, c := range cases {
		got := sanitizeID(c.in)
		if got != c.want {
			t.Errorf("sanitizeID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ─── nextPort ────────────────────────────────────────────────────────────────

func TestNextPortDefault(t *testing.T) {
	m := newTestManager(t)
	if p := m.nextPort(); p != ":5020" {
		t.Errorf("nextPort = %q, want :5020", p)
	}
}

func TestNextPortIncrement(t *testing.T) {
	m := newTestManager(t)
	m.Create(CreateRequest{Name: "D1", ModbusAddr: ":5020"})
	m.Create(CreateRequest{Name: "D2", ModbusAddr: ":5021"})
	if p := m.nextPort(); p != ":5022" {
		t.Errorf("nextPort = %q, want :5022", p)
	}
}

// ─── StartAll ────────────────────────────────────────────────────────────────

func TestStartAll(t *testing.T) {
	m := newTestManager(t)
	m.Create(CreateRequest{Name: "D1", ModbusAddr: ":15200"})
	m.Create(CreateRequest{Name: "D2", ModbusAddr: ":15201"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.StartAll(ctx)

	for _, info := range m.List() {
		d, _ := m.Get(info.ID)
		if d.GetStatus() != StatusRunning {
			t.Errorf("device %q should be running after StartAll", info.ID)
		}
		d.Stop()
	}
}

// ─── List ordering ───────────────────────────────────────────────────────────

func TestListSortedByID(t *testing.T) {
	m := newTestManager(t)
	m.Create(CreateRequest{Name: "zzz"})
	m.Create(CreateRequest{Name: "aaa"})
	m.Create(CreateRequest{Name: "mmm"})

	list := m.List()
	for i := 1; i < len(list); i++ {
		if strings.Compare(list[i-1].ID, list[i].ID) > 0 {
			t.Errorf("List not sorted: %q > %q", list[i-1].ID, list[i].ID)
		}
	}
}
