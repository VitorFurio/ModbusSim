package api

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"modbussim/internal/config"
	"modbussim/internal/register"
)

// Engine is the interface the API server needs.
type Engine interface {
	List() []register.Register
	Get(id string) (register.Register, bool)
	Add(r register.Register) (string, error)
	Update(id string, r register.Register) error
	Remove(id string) error
	Subscribe() <-chan []register.RegisterSnapshot
	Unsubscribe(ch <-chan []register.RegisterSnapshot)
}

// Server is the HTTP+WebSocket API server.
type Server struct {
	addr    string
	eng     Engine
	cfg     *config.Config
	versDir string
	static  fs.FS
	mu      sync.RWMutex

	// WS hub
	hubMu   sync.Mutex
	clients map[*wsClient]struct{}
}

// NewServer creates a new API server.
func NewServer(addr string, eng Engine, cfg *config.Config, versDir string, static fs.FS) *Server {
	return &Server{
		addr:    addr,
		eng:     eng,
		cfg:     cfg,
		versDir: versDir,
		static:  static,
		clients: make(map[*wsClient]struct{}),
	}
}

// Start begins listening. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Register routes.
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

	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/", s.handleSPA)

	srv := &http.Server{
		Addr:    s.addr,
		Handler: mux,
	}

	// Start WS broadcast loop.
	go s.broadcastLoop(ctx)

	// Shutdown on ctx cancel.
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx)
	}()

	slog.Info("api server started", "addr", s.addr)
	return srv.ListenAndServe()
}

// ─── helpers ────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ─── Register handlers ───────────────────────────────────────────────────────

func (s *Server) handleListRegisters(w http.ResponseWriter, r *http.Request) {
	regs := s.eng.List()
	writeJSON(w, http.StatusOK, regs)
}

func (s *Server) handleCreateRegister(w http.ResponseWriter, r *http.Request) {
	var reg register.Register
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id, err := s.eng.Add(reg)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	got, _ := s.eng.Get(id)
	writeJSON(w, http.StatusCreated, got)
}

func (s *Server) handleUpdateRegister(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var reg register.Register
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.eng.Update(id, reg); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	got, _ := s.eng.Get(id)
	writeJSON(w, http.StatusOK, got)
}

func (s *Server) handleDeleteRegister(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.eng.Remove(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Config handlers ─────────────────────────────────────────────────────────

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cfg := *s.cfg
	s.mu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"name":        cfg.Name,
		"description": cfg.Description,
		"modbus_addr": cfg.ModbusAddr,
		"admin_addr":  cfg.AdminAddr,
	})
}

func (s *Server) handleSaveConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	s.mu.Lock()
	if body.Name != "" {
		s.cfg.Name = body.Name
	}
	if body.Description != "" {
		s.cfg.Description = body.Description
	}
	s.cfg.Registers = s.eng.List()
	cfgCopy := *s.cfg
	s.mu.Unlock()

	path, err := config.Save(&cfgCopy, s.versDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}

// ─── Version handlers ─────────────────────────────────────────────────────────

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := config.ListVersions(s.versDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

func (s *Server) handleLoadVersion(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Path == "" {
		writeError(w, http.StatusBadRequest, "path required")
		return
	}
	cfg, err := config.Load(body.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Remove all existing registers.
	existing := s.eng.List()
	for _, reg := range existing {
		s.eng.Remove(reg.ID)
	}

	// Add new registers.
	for _, reg := range cfg.Registers {
		if _, err := s.eng.Add(reg); err != nil {
			slog.Warn("load version: skip register", "id", reg.ID, "err", err)
		}
	}

	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "loaded"})
}

func (s *Server) handleExportVersion(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cfgCopy := *s.cfg
	s.mu.RUnlock()
	cfgCopy.Registers = s.eng.List()

	data, err := config.Export(&cfgCopy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Content-Disposition", `attachment; filename="modbussim.yaml"`)
	w.Write(data)
}

func (s *Server) handleImportVersion(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg, err := config.Import(data)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Replace all registers.
	existing := s.eng.List()
	for _, reg := range existing {
		s.eng.Remove(reg.ID)
	}
	for _, reg := range cfg.Registers {
		if _, err := s.eng.Add(reg); err != nil {
			slog.Warn("import: skip register", "id", reg.ID, "err", err)
		}
	}

	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "imported"})
}

// ─── SPA fallback ─────────────────────────────────────────────────────────────

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if s.static == nil {
		http.Error(w, "frontend not embedded", http.StatusNotFound)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// Try to open the file.
	f, err := s.static.Open(path)
	if err != nil {
		// Fallback to index.html for SPA routing.
		path = "index.html"
		f, err = s.static.Open(path)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	}
	defer f.Close()

	// Set content type explicitly so browsers (especially Safari) handle it correctly.
	switch {
	case strings.HasSuffix(path, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case strings.HasSuffix(path, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(path, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case strings.HasSuffix(path, ".svg"):
		w.Header().Set("Content-Type", "image/svg+xml")
	case strings.HasSuffix(path, ".ico"):
		w.Header().Set("Content-Type", "image/x-icon")
	case strings.HasSuffix(path, ".woff2"):
		w.Header().Set("Content-Type", "font/woff2")
	case strings.HasSuffix(path, ".woff"):
		w.Header().Set("Content-Type", "font/woff")
	}

	data, err := io.ReadAll(f)
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

// ─── WebSocket ────────────────────────────────────────────────────────────────

type wsClient struct {
	send chan []byte
	done chan struct{}
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	// Minimal WebSocket handshake without external deps.
	conn, err := upgradeWS(w, r)
	if err != nil {
		slog.Warn("ws upgrade failed", "err", err)
		return
	}
	defer conn.Close()

	client := &wsClient{
		send: make(chan []byte, 10),
		done: make(chan struct{}),
	}

	s.hubMu.Lock()
	s.clients[client] = struct{}{}
	s.hubMu.Unlock()

	defer func() {
		s.hubMu.Lock()
		delete(s.clients, client)
		s.hubMu.Unlock()
		close(client.done)
	}()

	// Send initial snapshot.
	regs := s.eng.List()
	snapshots := make([]map[string]any, 0, len(regs))
	for _, reg := range regs {
		snapshots = append(snapshots, map[string]any{
			"id":         reg.ID,
			"value":      reg.Value,
			"updated_at": reg.UpdatedAt,
			"history":    [30]float64{},
		})
	}
	initial, _ := json.Marshal(map[string]any{
		"type":      "snapshot",
		"registers": snapshots,
	})
	if err := wsWriteText(conn, initial); err != nil {
		return
	}

	// Read goroutine (to detect client disconnect).
	go func() {
		buf := make([]byte, 1024)
		for {
			if _, err := conn.Read(buf); err != nil {
				close(client.send) // signal writer to stop
				return
			}
		}
	}()

	// Write loop.
	for msg := range client.send {
		if err := wsWriteText(conn, msg); err != nil {
			return
		}
	}
}

// broadcastLoop subscribes to engine updates and fans out to WS clients.
func (s *Server) broadcastLoop(ctx context.Context) {
	ch := s.eng.Subscribe()
	defer s.eng.Unsubscribe(ch)

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	var lastSnapshots []register.RegisterSnapshot

	for {
		select {
		case <-ctx.Done():
			return
		case snaps, ok := <-ch:
			if ok {
				lastSnapshots = snaps
			}
		case <-ticker.C:
			if lastSnapshots == nil {
				continue
			}
			payload := make([]map[string]any, 0, len(lastSnapshots))
			for _, snap := range lastSnapshots {
				payload = append(payload, map[string]any{
					"id":         snap.ID,
					"value":      snap.Value,
					"updated_at": snap.UpdatedAt,
					"history":    snap.History,
				})
			}
			msg, err := json.Marshal(map[string]any{
				"type":      "snapshot",
				"registers": payload,
			})
			if err != nil {
				continue
			}
			s.hubMu.Lock()
			for client := range s.clients {
				select {
				case client.send <- msg:
				default:
					// slow client, drop
				}
			}
			s.hubMu.Unlock()
		}
	}
}
