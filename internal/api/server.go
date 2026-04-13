package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"modbussim/internal/device"
	"modbussim/internal/register"
)

// Server is the HTTP+WebSocket API server.
type Server struct {
	addr   string
	mgr    *device.Manager
	appCtx context.Context
	static fs.FS

	// WS hub: device ID -> set of clients
	hubMu   sync.Mutex
	clients map[string]map[*wsClient]struct{}
}

// NewServer creates a new API server.
func NewServer(addr string, mgr *device.Manager, appCtx context.Context, static fs.FS) *Server {
	return &Server{
		addr:    addr,
		mgr:     mgr,
		appCtx:  appCtx,
		static:  static,
		clients: make(map[string]map[*wsClient]struct{}),
	}
}

// Start begins listening. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// Device management
	mux.HandleFunc("GET /api/devices", s.handleListDevices)
	mux.HandleFunc("POST /api/devices", s.handleCreateDevice)
	mux.HandleFunc("GET /api/devices/{id}", s.handleGetDevice)
	mux.HandleFunc("PUT /api/devices/{id}", s.handleUpdateDevice)
	mux.HandleFunc("DELETE /api/devices/{id}", s.handleDeleteDevice)
	mux.HandleFunc("POST /api/devices/{id}/start", s.handleStartDevice)
	mux.HandleFunc("POST /api/devices/{id}/stop", s.handleStopDevice)

	// Per-device registers
	mux.HandleFunc("GET /api/devices/{id}/registers", s.handleListRegisters)
	mux.HandleFunc("POST /api/devices/{id}/registers", s.handleCreateRegister)
	mux.HandleFunc("PUT /api/devices/{id}/registers/{rid}", s.handleUpdateRegister)
	mux.HandleFunc("DELETE /api/devices/{id}/registers/{rid}", s.handleDeleteRegister)

	// Per-device versions
	mux.HandleFunc("POST /api/devices/{id}/versions/save", s.handleSaveVersion)
	mux.HandleFunc("GET /api/devices/{id}/versions", s.handleListVersions)
	mux.HandleFunc("POST /api/devices/{id}/versions/load", s.handleLoadVersion)
	mux.HandleFunc("GET /api/devices/{id}/versions/export", s.handleExportVersion)
	mux.HandleFunc("POST /api/devices/{id}/versions/import", s.handleImportVersion)

	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/", s.handleSPA)

	srv := &http.Server{
		Addr:    s.addr,
		Handler: loggingMiddleware(recoveryMiddleware(mux)),
		ConnState: func(conn net.Conn, state http.ConnState) {
			slog.Debug("conn", "remote", conn.RemoteAddr(), "state", state.String())
		},
		ErrorLog: slog.NewLogLogger(slog.Default().Handler(), slog.LevelWarn),
	}

	ln, err := net.Listen("tcp4", s.addr)
	if err != nil {
		ln, err = net.Listen("tcp", s.addr)
		if err != nil {
			return err
		}
	}

	// Launch broadcast loops for all currently running devices.
	for _, info := range s.mgr.List() {
		d, ok := s.mgr.Get(info.ID)
		if ok && d.GetStatus() == device.StatusRunning {
			go s.broadcastLoop(d.RunCtx(), d)
		}
	}

	// Shutdown on ctx cancel.
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx)
	}()

	slog.Info("api server started", "addr", ln.Addr().String(), "network", ln.Addr().Network())
	return srv.Serve(ln)
}

// deviceFromRequest extracts the device from the path value "id".
func (s *Server) deviceFromRequest(w http.ResponseWriter, r *http.Request) (*device.Device, bool) {
	id := r.PathValue("id")
	d, ok := s.mgr.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("device %q not found", id))
	}
	return d, ok
}

// ─── Device handlers ─────────────────────────────────────────────────────────

func (s *Server) handleListDevices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.mgr.List())
}

func (s *Server) handleCreateDevice(w http.ResponseWriter, r *http.Request) {
	var req device.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	d, err := s.mgr.Create(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, d.Info())
}

func (s *Server) handleGetDevice(w http.ResponseWriter, r *http.Request) {
	d, ok := s.deviceFromRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, d.Info())
}

func (s *Server) handleUpdateDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req device.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.mgr.Update(id, req); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	d, _ := s.mgr.Get(id)
	writeJSON(w, http.StatusOK, d.Info())
}

func (s *Server) handleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.mgr.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStartDevice(w http.ResponseWriter, r *http.Request) {
	d, ok := s.deviceFromRequest(w, r)
	if !ok {
		return
	}
	if err := d.Start(s.appCtx); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	go s.broadcastLoop(d.RunCtx(), d)
	writeJSON(w, http.StatusOK, d.Info())
}

func (s *Server) handleStopDevice(w http.ResponseWriter, r *http.Request) {
	d, ok := s.deviceFromRequest(w, r)
	if !ok {
		return
	}
	d.Stop()
	writeJSON(w, http.StatusOK, d.Info())
}

// ─── Register handlers ───────────────────────────────────────────────────────

func (s *Server) handleListRegisters(w http.ResponseWriter, r *http.Request) {
	d, ok := s.deviceFromRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, d.Engine().List())
}

func (s *Server) handleCreateRegister(w http.ResponseWriter, r *http.Request) {
	d, ok := s.deviceFromRequest(w, r)
	if !ok {
		return
	}
	var reg register.Register
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id, err := d.Engine().Add(reg)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	got, _ := d.Engine().Get(id)
	writeJSON(w, http.StatusCreated, got)
}

func (s *Server) handleUpdateRegister(w http.ResponseWriter, r *http.Request) {
	d, ok := s.deviceFromRequest(w, r)
	if !ok {
		return
	}
	rid := r.PathValue("rid")
	var reg register.Register
	if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := d.Engine().Update(rid, reg); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	got, _ := d.Engine().Get(rid)
	writeJSON(w, http.StatusOK, got)
}

func (s *Server) handleDeleteRegister(w http.ResponseWriter, r *http.Request) {
	d, ok := s.deviceFromRequest(w, r)
	if !ok {
		return
	}
	rid := r.PathValue("rid")
	if err := d.Engine().Remove(rid); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Version handlers ─────────────────────────────────────────────────────────

func (s *Server) handleSaveVersion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	path, err := s.mgr.SaveVersion(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	versions, err := s.mgr.ListVersions(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, versions)
}

func (s *Server) handleLoadVersion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Path == "" {
		writeError(w, http.StatusBadRequest, "path required")
		return
	}
	if err := s.mgr.LoadVersion(id, body.Path); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "loaded"})
}

func (s *Server) handleExportVersion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	data, err := s.mgr.ExportDevice(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	d, _ := s.mgr.Get(id)
	filename := d.ID() + ".yaml"
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Write(data)
}

func (s *Server) handleImportVersion(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.mgr.ImportDevice(id, data); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
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

	f, err := s.static.Open(path)
	if err != nil {
		path = "index.html"
		f, err = s.static.Open(path)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	}
	defer f.Close()

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

// handleWS upgrades to WebSocket and streams snapshots for the requested device.
// The device is specified via ?device=<id> query parameter.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device")
	dev, ok := s.mgr.Get(deviceID)
	if !ok {
		http.Error(w, fmt.Sprintf("device %q not found", deviceID), http.StatusNotFound)
		return
	}

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
	if s.clients[deviceID] == nil {
		s.clients[deviceID] = make(map[*wsClient]struct{})
	}
	s.clients[deviceID][client] = struct{}{}
	s.hubMu.Unlock()

	defer func() {
		s.hubMu.Lock()
		delete(s.clients[deviceID], client)
		s.hubMu.Unlock()
		close(client.done)
	}()

	// Send initial snapshot.
	regs := dev.Engine().List()
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

	// Read goroutine handles PING/CLOSE frames.
	go func() {
		for {
			opcode, payload, err := wsReadFrame(conn)
			if err != nil {
				close(client.send)
				return
			}
			switch opcode {
			case 0x9: // PING
				_ = wsWriteControl(conn, 0xA, payload)
			case 0x8: // CLOSE
				_ = wsWriteControl(conn, 0x8, payload)
				close(client.send)
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

// broadcastLoop subscribes to a device's engine and fans snapshots out to
// all WebSocket clients connected to that device.
// It runs until ctx is cancelled (i.e. the device stops).
func (s *Server) broadcastLoop(ctx context.Context, dev *device.Device) {
	if ctx == nil {
		return
	}
	deviceID := dev.ID()
	eng := dev.Engine()
	ch := eng.Subscribe()
	defer eng.Unsubscribe(ch)

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
			for client := range s.clients[deviceID] {
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

// ─── Middleware ───────────────────────────────────────────────────────────────

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		elapsed := time.Since(start)

		level := slog.LevelDebug
		if r.URL.Path != "/ws" {
			level = slog.LevelInfo
		}
		slog.Log(r.Context(), level, "http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", elapsed.Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijack not supported")
	}
	return hj.Hijack()
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("handler panic", "path", r.URL.Path, "panic", fmt.Sprintf("%v", rec))
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
