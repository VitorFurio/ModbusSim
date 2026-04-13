package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"modbussim/internal/device"
	"modbussim/internal/register"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func newTestManager(t *testing.T) *device.Manager {
	t.Helper()
	devDir, _ := os.MkdirTemp("", "api_dev_*")
	verDir, _ := os.MkdirTemp("", "api_ver_*")
	t.Cleanup(func() {
		os.RemoveAll(devDir)
		os.RemoveAll(verDir)
	})
	return device.NewManager(devDir, verDir)
}

func newTestServer(t *testing.T) (*Server, *device.Manager) {
	t.Helper()
	mgr := newTestManager(t)
	ctx := context.Background()
	srv := NewServer(":0", mgr, ctx, nil)
	return srv, mgr
}

func newTestServerWithDevice(t *testing.T, req device.CreateRequest) (*Server, *device.Manager, string) {
	t.Helper()
	srv, mgr := newTestServer(t)
	d, err := mgr.Create(req)
	if err != nil {
		t.Fatalf("create device: %v", err)
	}
	return srv, mgr, d.ID()
}

func buildMux(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/devices", s.handleListDevices)
	mux.HandleFunc("POST /api/devices", s.handleCreateDevice)
	mux.HandleFunc("GET /api/devices/{id}", s.handleGetDevice)
	mux.HandleFunc("PUT /api/devices/{id}", s.handleUpdateDevice)
	mux.HandleFunc("DELETE /api/devices/{id}", s.handleDeleteDevice)
	mux.HandleFunc("POST /api/devices/{id}/start", s.handleStartDevice)
	mux.HandleFunc("POST /api/devices/{id}/stop", s.handleStopDevice)
	mux.HandleFunc("GET /api/devices/{id}/registers", s.handleListRegisters)
	mux.HandleFunc("POST /api/devices/{id}/registers", s.handleCreateRegister)
	mux.HandleFunc("PUT /api/devices/{id}/registers/{rid}", s.handleUpdateRegister)
	mux.HandleFunc("DELETE /api/devices/{id}/registers/{rid}", s.handleDeleteRegister)
	mux.HandleFunc("POST /api/devices/{id}/versions/save", s.handleSaveVersion)
	mux.HandleFunc("GET /api/devices/{id}/versions", s.handleListVersions)
	mux.HandleFunc("POST /api/devices/{id}/versions/load", s.handleLoadVersion)
	mux.HandleFunc("GET /api/devices/{id}/versions/export", s.handleExportVersion)
	mux.HandleFunc("POST /api/devices/{id}/versions/import", s.handleImportVersion)
	return mux
}

func doRequest(t *testing.T, s *Server, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	buildMux(s).ServeHTTP(w, req)
	return w
}

// ─── GET /api/devices ────────────────────────────────────────────────────────

func TestListDevicesEmpty(t *testing.T) {
	srv, _ := newTestServer(t)
	w := doRequest(t, srv, "GET", "/api/devices", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var list []device.Info
	json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestListDevicesReturnsAll(t *testing.T) {
	srv, mgr := newTestServer(t)
	mgr.Create(device.CreateRequest{Name: "D1"})
	mgr.Create(device.CreateRequest{Name: "D2"})

	w := doRequest(t, srv, "GET", "/api/devices", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var list []device.Info
	json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 2 {
		t.Errorf("expected 2 devices, got %d", len(list))
	}
}

// ─── POST /api/devices ───────────────────────────────────────────────────────

func TestCreateDevice(t *testing.T) {
	srv, _ := newTestServer(t)
	w := doRequest(t, srv, "POST", "/api/devices", map[string]string{"name": "PLC"})
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
	var info device.Info
	json.NewDecoder(w.Body).Decode(&info)
	if info.Name != "PLC" {
		t.Errorf("Name = %q, want PLC", info.Name)
	}
}

func TestCreateDeviceBadJSON(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/devices", bytes.NewBufferString("not-json"))
	w := httptest.NewRecorder()
	srv.handleCreateDevice(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestCreateDeviceEmptyName(t *testing.T) {
	srv, _ := newTestServer(t)
	w := doRequest(t, srv, "POST", "/api/devices", map[string]string{"name": ""})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ─── GET /api/devices/{id} ───────────────────────────────────────────────────

func TestGetDevice(t *testing.T) {
	srv, _, id := newTestServerWithDevice(t, device.CreateRequest{Name: "Dev"})
	w := doRequest(t, srv, "GET", "/api/devices/"+id, nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var info device.Info
	json.NewDecoder(w.Body).Decode(&info)
	if info.ID != id {
		t.Errorf("ID = %q, want %q", info.ID, id)
	}
}

func TestGetDeviceNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	w := doRequest(t, srv, "GET", "/api/devices/ghost", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ─── PUT /api/devices/{id} ───────────────────────────────────────────────────

func TestUpdateDevice(t *testing.T) {
	srv, _, id := newTestServerWithDevice(t, device.CreateRequest{Name: "Old"})
	w := doRequest(t, srv, "PUT", "/api/devices/"+id, map[string]string{"name": "New"})
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var info device.Info
	json.NewDecoder(w.Body).Decode(&info)
	if info.Name != "New" {
		t.Errorf("Name = %q, want New", info.Name)
	}
}

// ─── DELETE /api/devices/{id} ────────────────────────────────────────────────

func TestDeleteDevice(t *testing.T) {
	srv, _, id := newTestServerWithDevice(t, device.CreateRequest{Name: "Dev"})
	w := doRequest(t, srv, "DELETE", "/api/devices/"+id, nil)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestDeleteDeviceNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	w := doRequest(t, srv, "DELETE", "/api/devices/ghost", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ─── POST /api/devices/{id}/start and /stop ───────────────────────────────────

func TestStartStopDevice(t *testing.T) {
	srv, _, id := newTestServerWithDevice(t, device.CreateRequest{
		Name: "Dev", ModbusAddr: ":15300",
	})

	w := doRequest(t, srv, "POST", "/api/devices/"+id+"/start", nil)
	if w.Code != http.StatusOK {
		t.Errorf("start status = %d, want 200", w.Code)
	}
	var info device.Info
	json.NewDecoder(w.Body).Decode(&info)
	if info.Status != device.StatusRunning {
		t.Errorf("status = %q after start, want running", info.Status)
	}

	w = doRequest(t, srv, "POST", "/api/devices/"+id+"/stop", nil)
	if w.Code != http.StatusOK {
		t.Errorf("stop status = %d, want 200", w.Code)
	}
	var info2 device.Info
	json.NewDecoder(w.Body).Decode(&info2)
	if info2.Status != device.StatusStopped {
		t.Errorf("status = %q after stop, want stopped", info2.Status)
	}
}

// ─── GET /api/devices/{id}/registers ────────────────────────────────────────

func TestListRegistersEmpty(t *testing.T) {
	srv, _, id := newTestServerWithDevice(t, device.CreateRequest{Name: "Dev"})
	w := doRequest(t, srv, "GET", "/api/devices/"+id+"/registers", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var regs []register.Register
	json.NewDecoder(w.Body).Decode(&regs)
	if len(regs) != 0 {
		t.Errorf("expected empty registers, got %d", len(regs))
	}
}

func TestCreateAndListRegisters(t *testing.T) {
	srv, _, id := newTestServerWithDevice(t, device.CreateRequest{Name: "Dev"})

	body := map[string]any{
		"id": "r1", "name": "Sensor", "address": 0, "data_type": "uint16",
		"signal": map[string]any{"kind": "constant", "value": 42},
	}
	w := doRequest(t, srv, "POST", "/api/devices/"+id+"/registers", body)
	if w.Code != http.StatusCreated {
		t.Errorf("create status = %d, want 201", w.Code)
	}

	w = doRequest(t, srv, "GET", "/api/devices/"+id+"/registers", nil)
	var regs []register.Register
	json.NewDecoder(w.Body).Decode(&regs)
	if len(regs) != 1 {
		t.Errorf("expected 1 register, got %d", len(regs))
	}
}

func TestUpdateRegister(t *testing.T) {
	srv, mgr, id := newTestServerWithDevice(t, device.CreateRequest{Name: "Dev"})
	d, _ := mgr.Get(id)
	d.Engine().Add(register.Register{
		ID: "r1", Name: "Old", Address: 0, DataType: register.TypeUint16,
		Signal: register.Signal{Kind: register.SignalConstant},
	})

	body := map[string]any{
		"name": "New", "address": 0, "data_type": "uint16",
		"signal": map[string]any{"kind": "constant", "value": 99},
	}
	w := doRequest(t, srv, "PUT", "/api/devices/"+id+"/registers/r1", body)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestDeleteRegister(t *testing.T) {
	srv, mgr, id := newTestServerWithDevice(t, device.CreateRequest{Name: "Dev"})
	d, _ := mgr.Get(id)
	d.Engine().Add(register.Register{
		ID: "r1", Name: "R1", Address: 0, DataType: register.TypeUint16,
		Signal: register.Signal{Kind: register.SignalConstant},
	})

	w := doRequest(t, srv, "DELETE", "/api/devices/"+id+"/registers/r1", nil)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestDeleteRegisterNotFound(t *testing.T) {
	srv, _, id := newTestServerWithDevice(t, device.CreateRequest{Name: "Dev"})
	w := doRequest(t, srv, "DELETE", "/api/devices/"+id+"/registers/ghost", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ─── Versions ────────────────────────────────────────────────────────────────

func TestSaveAndListVersionsAPI(t *testing.T) {
	srv, _, id := newTestServerWithDevice(t, device.CreateRequest{Name: "Dev"})

	w := doRequest(t, srv, "POST", "/api/devices/"+id+"/versions/save", nil)
	if w.Code != http.StatusOK {
		t.Errorf("save status = %d, want 200", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["path"] == "" {
		t.Error("expected non-empty path")
	}

	w = doRequest(t, srv, "GET", "/api/devices/"+id+"/versions", nil)
	if w.Code != http.StatusOK {
		t.Errorf("list status = %d, want 200", w.Code)
	}
}

func TestLoadVersionInvalidPath(t *testing.T) {
	srv, _, id := newTestServerWithDevice(t, device.CreateRequest{Name: "Dev"})
	w := doRequest(t, srv, "POST", "/api/devices/"+id+"/versions/load",
		map[string]string{"path": "/nonexistent.yaml"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestLoadVersionMissingPath(t *testing.T) {
	srv, _, id := newTestServerWithDevice(t, device.CreateRequest{Name: "Dev"})
	w := doRequest(t, srv, "POST", "/api/devices/"+id+"/versions/load",
		map[string]string{})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestExportVersion(t *testing.T) {
	srv, _, id := newTestServerWithDevice(t, device.CreateRequest{Name: "Dev"})
	w := doRequest(t, srv, "GET", "/api/devices/"+id+"/versions/export", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/yaml" {
		t.Errorf("Content-Type = %q, want application/yaml", ct)
	}
}

func TestImportVersionValid(t *testing.T) {
	srv, _, id := newTestServerWithDevice(t, device.CreateRequest{Name: "Dev"})
	yamlData := `version: "1"
id: dev
name: Dev
modbus_addr: ":5020"
registers: []
`
	req := httptest.NewRequest("POST", "/api/devices/"+id+"/versions/import",
		bytes.NewBufferString(yamlData))
	w := httptest.NewRecorder()
	buildMux(srv).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestImportVersionInvalid(t *testing.T) {
	srv, _, id := newTestServerWithDevice(t, device.CreateRequest{Name: "Dev"})
	req := httptest.NewRequest("POST", "/api/devices/"+id+"/versions/import",
		bytes.NewBufferString("key: [unclosed"))
	w := httptest.NewRecorder()
	buildMux(srv).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ─── SPA fallback ─────────────────────────────────────────────────────────────

func TestSPAFallbackNoStatic(t *testing.T) {
	srv, _ := newTestServer(t)
	req := httptest.NewRequest("GET", "/some/route", nil)
	w := httptest.NewRecorder()
	srv.handleSPA(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when no static FS", w.Code)
	}
}

// ─── broadcastLoop ───────────────────────────────────────────────────────────

func TestBroadcastLoopRunsUntilContextCancel(t *testing.T) {
	srv, mgr := newTestServer(t)
	d, _ := mgr.Create(device.CreateRequest{Name: "Dev"})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		srv.broadcastLoop(ctx, d)
		close(done)
	}()

	select {
	case <-done:
		// broadcastLoop exited when ctx was cancelled — correct.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("broadcastLoop did not exit after context cancel")
	}
}

func TestBroadcastLoopNilCtx(t *testing.T) {
	srv, mgr := newTestServer(t)
	d, _ := mgr.Create(device.CreateRequest{Name: "Dev"})
	// must not block or panic with nil ctx
	done := make(chan struct{})
	go func() {
		srv.broadcastLoop(nil, d)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("broadcastLoop with nil ctx should return immediately")
	}
}
