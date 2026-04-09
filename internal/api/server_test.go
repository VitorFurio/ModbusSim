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

	"modbussim/internal/config"
	"modbussim/internal/register"
)

// ─── Stub Engine ─────────────────────────────────────────────────────────────

type stubEngine struct {
	registers []register.Register
}

func (e *stubEngine) List() []register.Register { return e.registers }

func (e *stubEngine) Get(id string) (register.Register, bool) {
	for _, r := range e.registers {
		if r.ID == id {
			return r, true
		}
	}
	return register.Register{}, false
}

func (e *stubEngine) Add(r register.Register) (string, error) {
	if r.ID == "" {
		r.ID = r.Name
	}
	e.registers = append(e.registers, r)
	return r.ID, nil
}

func (e *stubEngine) Update(id string, r register.Register) error {
	for i, reg := range e.registers {
		if reg.ID == id {
			r.ID = id
			e.registers[i] = r
			return nil
		}
	}
	return &notFoundErr{id}
}

func (e *stubEngine) Remove(id string) error {
	for i, r := range e.registers {
		if r.ID == id {
			e.registers = append(e.registers[:i], e.registers[i+1:]...)
			return nil
		}
	}
	return &notFoundErr{id}
}

func (e *stubEngine) Subscribe() <-chan []register.RegisterSnapshot {
	ch := make(chan []register.RegisterSnapshot, 1)
	return ch
}

func (e *stubEngine) Unsubscribe(_ <-chan []register.RegisterSnapshot) {}

type notFoundErr struct{ id string }

func (e *notFoundErr) Error() string { return "register " + e.id + " not found" }

// ─── Test helpers ─────────────────────────────────────────────────────────────

func newTestServer(eng Engine) *Server {
	cfg := config.Default()
	dir, _ := os.MkdirTemp("", "api_test_*")
	return NewServer(":0", eng, cfg, dir, nil)
}

func doRequest(t *testing.T, srv *Server, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody bytes.Buffer
	if body != nil {
		json.NewEncoder(&reqBody).Encode(body)
	}
	req := httptest.NewRequest(method, path, &reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux := buildMux(srv)
	mux.ServeHTTP(w, req)
	return w
}

// buildMux replicates the route registration from Start() for unit testing.
func buildMux(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/registers", s.handleListRegisters)
	mux.HandleFunc("POST /api/registers", s.handleCreateRegister)
	mux.HandleFunc("PUT /api/registers/{id}", s.handleUpdateRegister)
	mux.HandleFunc("DELETE /api/registers/{id}", s.handleDeleteRegister)
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("POST /api/config/save", s.handleSaveConfig)
	mux.HandleFunc("GET /api/versions", s.handleListVersions)
	mux.HandleFunc("POST /api/versions/load", s.handleLoadVersion)
	mux.HandleFunc("GET /api/versions/export", s.handleExportVersion)
	mux.HandleFunc("POST /api/versions/import", s.handleImportVersion)
	return mux
}

// ─── GET /api/registers ───────────────────────────────────────────────────────

func TestListRegistersEmpty(t *testing.T) {
	srv := newTestServer(&stubEngine{})
	w := doRequest(t, srv, "GET", "/api/registers", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var regs []register.Register
	json.NewDecoder(w.Body).Decode(&regs)
	if len(regs) != 0 {
		t.Errorf("expected empty list, got %d", len(regs))
	}
}

func TestListRegistersReturnsAll(t *testing.T) {
	eng := &stubEngine{registers: []register.Register{
		{ID: "r1", Name: "R1"},
		{ID: "r2", Name: "R2"},
	}}
	srv := newTestServer(eng)
	w := doRequest(t, srv, "GET", "/api/registers", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var regs []register.Register
	json.NewDecoder(w.Body).Decode(&regs)
	if len(regs) != 2 {
		t.Errorf("expected 2 registers, got %d", len(regs))
	}
}

// ─── POST /api/registers ─────────────────────────────────────────────────────

func TestCreateRegister(t *testing.T) {
	eng := &stubEngine{}
	srv := newTestServer(eng)
	body := map[string]interface{}{
		"id": "r1", "name": "Sensor", "address": 0, "data_type": "uint16",
		"signal": map[string]interface{}{"kind": "constant", "value": 42},
	}
	w := doRequest(t, srv, "POST", "/api/registers", body)
	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", w.Code)
	}
}

func TestCreateRegisterBadJSON(t *testing.T) {
	srv := newTestServer(&stubEngine{})
	req := httptest.NewRequest("POST", "/api/registers", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleCreateRegister(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ─── PUT /api/registers/{id} ─────────────────────────────────────────────────

func TestUpdateRegister(t *testing.T) {
	eng := &stubEngine{registers: []register.Register{
		{ID: "r1", Name: "Old", Address: 0, DataType: register.TypeUint16},
	}}
	srv := newTestServer(eng)
	body := map[string]interface{}{
		"name": "New", "address": 0, "data_type": "uint16",
		"signal": map[string]interface{}{"kind": "constant", "value": 99},
	}
	w := doRequest(t, srv, "PUT", "/api/registers/r1", body)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestUpdateRegisterNotFound(t *testing.T) {
	srv := newTestServer(&stubEngine{})
	body := map[string]interface{}{"name": "X", "address": 0, "data_type": "uint16"}
	w := doRequest(t, srv, "PUT", "/api/registers/ghost", body)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ─── DELETE /api/registers/{id} ──────────────────────────────────────────────

func TestDeleteRegister(t *testing.T) {
	eng := &stubEngine{registers: []register.Register{{ID: "r1", Name: "R1"}}}
	srv := newTestServer(eng)
	w := doRequest(t, srv, "DELETE", "/api/registers/r1", nil)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestDeleteRegisterNotFound(t *testing.T) {
	srv := newTestServer(&stubEngine{})
	w := doRequest(t, srv, "DELETE", "/api/registers/ghost", nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ─── GET /api/config ─────────────────────────────────────────────────────────

func TestGetConfig(t *testing.T) {
	srv := newTestServer(&stubEngine{})
	w := doRequest(t, srv, "GET", "/api/config", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if _, ok := body["modbus_addr"]; !ok {
		t.Error("response missing modbus_addr field")
	}
	if _, ok := body["admin_addr"]; !ok {
		t.Error("response missing admin_addr field")
	}
}

// ─── POST /api/config/save ───────────────────────────────────────────────────

func TestSaveConfig(t *testing.T) {
	srv := newTestServer(&stubEngine{})
	body := map[string]string{"name": "saved", "description": "test save"}
	w := doRequest(t, srv, "POST", "/api/config/save", body)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["path"] == "" {
		t.Error("expected non-empty path in response")
	}
}

// ─── GET /api/versions ───────────────────────────────────────────────────────

func TestListVersionsEmpty(t *testing.T) {
	srv := newTestServer(&stubEngine{})
	w := doRequest(t, srv, "GET", "/api/versions", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// ─── POST /api/versions/load ─────────────────────────────────────────────────

func TestLoadVersionInvalidPath(t *testing.T) {
	srv := newTestServer(&stubEngine{})
	body := map[string]string{"path": "/nonexistent/path.yaml"}
	w := doRequest(t, srv, "POST", "/api/versions/load", body)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestLoadVersionMissingPath(t *testing.T) {
	srv := newTestServer(&stubEngine{})
	w := doRequest(t, srv, "POST", "/api/versions/load", map[string]string{})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ─── GET /api/versions/export ────────────────────────────────────────────────

func TestExportVersion(t *testing.T) {
	srv := newTestServer(&stubEngine{})
	w := doRequest(t, srv, "GET", "/api/versions/export", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/yaml" {
		t.Errorf("Content-Type = %q, want application/yaml", ct)
	}
}

// ─── POST /api/versions/import ───────────────────────────────────────────────

func TestImportVersionValid(t *testing.T) {
	srv := newTestServer(&stubEngine{})
	yamlData := `
version: "1"
name: imported
modbus_addr: ":5020"
admin_addr: ":7070"
registers: []
`
	req := httptest.NewRequest("POST", "/api/versions/import", bytes.NewBufferString(yamlData))
	w := httptest.NewRecorder()
	buildMux(srv).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestImportVersionInvalid(t *testing.T) {
	srv := newTestServer(&stubEngine{})
	req := httptest.NewRequest("POST", "/api/versions/import", bytes.NewBufferString("key: [unclosed"))
	w := httptest.NewRecorder()
	buildMux(srv).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ─── SPA fallback ─────────────────────────────────────────────────────────────

func TestSPAFallbackNoStatic(t *testing.T) {
	srv := newTestServer(&stubEngine{}) // static=nil
	req := httptest.NewRequest("GET", "/some/route", nil)
	w := httptest.NewRecorder()
	srv.handleSPA(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when no static FS", w.Code)
	}
}

// ─── broadcastLoop ───────────────────────────────────────────────────────────

func TestBroadcastLoopRunsUntilContextCancel(t *testing.T) {
	srv := newTestServer(&stubEngine{})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		srv.broadcastLoop(ctx)
		close(done)
	}()

	select {
	case <-done:
		// broadcastLoop exited when ctx was cancelled — correct.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("broadcastLoop did not exit after context cancel")
	}
}
