package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"modbussim/internal/register"
)

// ─── Default ─────────────────────────────────────────────────────────────────

func TestDefaultNotNil(t *testing.T) {
	cfg := Default()
	if cfg == nil {
		t.Fatal("Default() returned nil")
	}
}

func TestDefaultHasRegisters(t *testing.T) {
	cfg := Default()
	if len(cfg.Registers) == 0 {
		t.Error("Default config should have at least one register")
	}
}

func TestDefaultAddresses(t *testing.T) {
	cfg := Default()
	for i, r := range cfg.Registers {
		if r.Name == "" {
			t.Errorf("register[%d] has empty name", i)
		}
		if r.ID == "" {
			t.Errorf("register[%d] has empty ID", i)
		}
	}
}

// ─── Load ─────────────────────────────────────────────────────────────────────

func TestLoadValidFile(t *testing.T) {
	dir := t.TempDir()
	content := `
version: "1"
name: test
modbus_addr: ":5020"
admin_addr: ":7070"
registers:
  - id: r1
    name: Reg1
    address: 0
    data_type: uint16
    signal:
      kind: constant
      value: 42
`
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Name != "test" {
		t.Errorf("Name = %q, want %q", cfg.Name, "test")
	}
	if len(cfg.Registers) != 1 {
		t.Errorf("len(Registers) = %d, want 1", len(cfg.Registers))
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte("key: [unclosed bracket"), 0644)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

// ─── Save ─────────────────────────────────────────────────────────────────────

func TestSaveCreatesFile(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()
	path, err := Save(cfg, dir)
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("saved file does not exist: %s", path)
	}
}

func TestSaveFilenameContainsName(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()
	cfg.Name = "myconfig"
	path, err := Save(cfg, dir)
	if err != nil {
		t.Fatalf("Save error: %v", err)
	}
	if !strings.Contains(filepath.Base(path), "myconfig") {
		t.Errorf("filename %q does not contain config name", filepath.Base(path))
	}
}

func TestSaveSpecialCharsInName(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()
	cfg.Name = "my config!@#"
	path, err := Save(cfg, dir)
	if err != nil {
		t.Fatalf("Save with special chars failed: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
}

func TestSaveEmptyName(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()
	cfg.Name = ""
	path, err := Save(cfg, dir)
	if err != nil {
		t.Fatalf("Save with empty name failed: %v", err)
	}
	if !strings.Contains(filepath.Base(path), "config") {
		t.Errorf("filename %q should contain fallback 'config'", filepath.Base(path))
	}
}

// ─── ListVersions ─────────────────────────────────────────────────────────────

func TestListVersionsEmpty(t *testing.T) {
	dir := t.TempDir()
	versions, err := ListVersions(dir)
	if err != nil {
		t.Fatalf("ListVersions error: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(versions))
	}
}

func TestListVersionsReturnsSaved(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()
	if _, err := Save(cfg, dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	versions, err := ListVersions(dir)
	if err != nil {
		t.Fatalf("ListVersions error: %v", err)
	}
	if len(versions) != 1 {
		t.Errorf("expected 1 version, got %d", len(versions))
	}
	if versions[0].RegCount != len(cfg.Registers) {
		t.Errorf("RegCount = %d, want %d", versions[0].RegCount, len(cfg.Registers))
	}
}

func TestListVersionsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()
	Save(cfg, dir)
	// Small sleep to ensure different mod times.
	// We just check it doesn't panic and returns sorted.
	Save(cfg, dir)

	versions, err := ListVersions(dir)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	for i := 1; i < len(versions); i++ {
		if versions[i].SavedAt.After(versions[i-1].SavedAt) {
			t.Errorf("versions not sorted newest-first at index %d", i)
		}
	}
}

func TestListVersionsIgnoresNonYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "data.json"), []byte("{}"), 0644)

	versions, err := ListVersions(dir)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions (non-yaml ignored), got %d", len(versions))
	}
}

// ─── Export / Import ─────────────────────────────────────────────────────────

func TestExportImportRoundtrip(t *testing.T) {
	original := Default()
	data, err := Export(original)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	loaded, err := Import(data)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if loaded.Name != original.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, original.Name)
	}
	if len(loaded.Registers) != len(original.Registers) {
		t.Errorf("len(Registers) = %d, want %d", len(loaded.Registers), len(original.Registers))
	}
}

func TestImportInvalidYAML(t *testing.T) {
	if _, err := Import([]byte("key: [unclosed bracket")); err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestExportContainsRegisterData(t *testing.T) {
	cfg := &Config{
		Name:       "exporttest",
		ModbusAddr: ":5020",
		AdminAddr:  ":7070",
		Registers: []register.Register{
			{ID: "r1", Name: "Sensor", Address: 0, DataType: register.TypeUint16},
		},
	}
	data, err := Export(cfg)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if !strings.Contains(string(data), "Sensor") {
		t.Error("exported YAML should contain register name 'Sensor'")
	}
}
