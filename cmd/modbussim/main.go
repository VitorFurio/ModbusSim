package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"modbussim/internal/api"
	"modbussim/internal/device"
	"modbussim/internal/frontend"
)

func main() {
	defaultDevicesDir := defaultDir("devices")
	defaultVersionsDir := defaultDir("configs")

	cfgPath  := flag.String("config", "", "legacy YAML config to auto-import as first device")
	devDir   := flag.String("devices", defaultDevicesDir, "directory for device YAML files")
	versDir  := flag.String("versions", defaultVersionsDir, "root directory for version history")
	adminAddr := flag.String("admin", ":7070", "HTTP admin server address")
	logPath  := flag.String("log", defaultLogPath(), "path to log file (empty = disable)")
	flag.Parse()

	// ── Setup logging ──────────────────────────────────────────────────────────
	setupLogging(*logPath)

	// ── Startup diagnostics ────────────────────────────────────────────────────
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	slog.Info("startup",
		"os", runtime.GOOS,
		"arch", runtime.GOARCH,
		"go", runtime.Version(),
		"exe", exe,
		"cwd", cwd,
		"devices_dir", *devDir,
		"versions_dir", *versDir,
		"log_file", *logPath,
	)

	// ── App context ────────────────────────────────────────────────────────────
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// ── Device manager ─────────────────────────────────────────────────────────
	mgr := device.NewManager(*devDir, *versDir)

	if err := mgr.LoadAll(); err != nil {
		slog.Error("failed to load devices", "err", err)
		os.Exit(1)
	}

	if err := mgr.Migrate(*cfgPath); err != nil {
		slog.Error("failed to migrate config", "err", err)
		os.Exit(1)
	}

	mgr.StartAll(ctx)

	// ── Banner ─────────────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("  ┌─────────────────────────────────────────────────────────┐")
	fmt.Println("  │             ModbusSim — Modbus TCP Simulator            │")
	fmt.Println("  └─────────────────────────────────────────────────────────┘")
	for _, d := range mgr.List() {
		fmt.Printf("  Device %-20s  Modbus TCP %s\n", d.Name, d.ModbusAddr)
	}
	fmt.Printf("  Admin HTTP : http://localhost%s\n", *adminAddr)
	if *logPath != "" {
		fmt.Printf("  Log file   : %s\n", *logPath)
	}
	fmt.Println()
	fmt.Println("  Press Ctrl+C to stop.")
	fmt.Println()

	// ── Start HTTP+WS API server ───────────────────────────────────────────────
	apiSrv := api.NewServer(*adminAddr, mgr, ctx, frontend.FS())
	if err := apiSrv.Start(ctx); err != nil && err != http.ErrServerClosed {
		slog.Error("api server error", "err", err)
	}

	slog.Info("shutdown complete")
}

// setupLogging configures slog to write to stdout and optionally to a file.
func setupLogging(logPath string) {
	var w io.Writer = os.Stdout

	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot open log file %s: %v\n", logPath, err)
		} else {
			w = io.MultiWriter(os.Stdout, f)
			fmt.Fprintf(f, "\n--- ModbusSim started at %s ---\n", time.Now().Format(time.RFC3339))
		}
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
}

// defaultLogPath returns a log file path next to the running binary.
func defaultLogPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "modbussim.log"
	}
	return filepath.Join(filepath.Dir(exe), "modbussim.log")
}

// defaultDir returns a path relative to the running binary for the given name.
func defaultDir(name string) string {
	exe, err := os.Executable()
	if err != nil {
		return "./" + name
	}
	return filepath.Join(filepath.Dir(exe), name)
}
