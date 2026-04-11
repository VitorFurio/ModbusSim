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
	"modbussim/internal/config"
	"modbussim/internal/frontend"
	"modbussim/internal/modbus"
	"modbussim/internal/register"
)

func main() {
	defaultVersionsDir := defaultConfigsDir()

	cfgPath := flag.String("config", "", "path to YAML config file (optional)")
	versDir := flag.String("versions", defaultVersionsDir, "directory for saved config versions")
	logPath := flag.String("log", defaultLogPath(), "path to log file (empty = disable)")
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
		"versions_dir", *versDir,
		"log_file", *logPath,
	)

	// ── Load config ────────────────────────────────────────────────────────────
	var cfg *config.Config
	if *cfgPath != "" {
		var err error
		cfg, err = config.Load(*cfgPath)
		if err != nil {
			slog.Error("failed to load config", "err", err)
			os.Exit(1)
		}
		slog.Info("config loaded", "path", *cfgPath)
	} else {
		cfg = config.Default()
		slog.Info("config using defaults")
	}

	// ── Build engine ───────────────────────────────────────────────────────────
	eng := register.NewEngine()
	for _, r := range cfg.Registers {
		if _, err := eng.Add(r); err != nil {
			slog.Warn("skipping register", "id", r.ID, "err", err)
		}
	}
	slog.Info("engine ready", "registers", len(eng.List()))

	// ── Start engine ───────────────────────────────────────────────────────────
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	eng.Start(ctx)

	// ── Start Modbus TCP server ────────────────────────────────────────────────
	mbSrv := modbus.New(cfg.ModbusAddr, eng)
	if err := mbSrv.Start(); err != nil {
		slog.Error("failed to start modbus server", "err", err)
		os.Exit(1)
	}
	defer mbSrv.Stop()

	// ── Banner ─────────────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("  ┌─────────────────────────────────────────────────────────┐")
	fmt.Println("  │             ModbusSim — Modbus TCP Simulator            │")
	fmt.Println("  └─────────────────────────────────────────────────────────┘")
	fmt.Printf("  Modbus TCP : %s\n", cfg.ModbusAddr)
	fmt.Printf("  Admin HTTP : http://localhost%s\n", cfg.AdminAddr)
	if *logPath != "" {
		fmt.Printf("  Log file   : %s\n", *logPath)
	}
	fmt.Println()
	fmt.Println("  Press Ctrl+C to stop.")
	fmt.Println()

	// ── Start HTTP+WS API server ───────────────────────────────────────────────
	apiSrv := api.NewServer(cfg.AdminAddr, eng, cfg, *versDir, frontend.FS())
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
			// Can't open log file — continue with stdout only.
			fmt.Fprintf(os.Stderr, "warning: cannot open log file %s: %v\n", logPath, err)
		} else {
			w = io.MultiWriter(os.Stdout, f)
			// Write a separator so multiple runs are easy to distinguish.
			fmt.Fprintf(f, "\n--- ModbusSim started at %s ---\n", time.Now().Format(time.RFC3339))
		}
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
}

// defaultLogPath returns a log file path next to the binary.
func defaultLogPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "modbussim.log"
	}
	return filepath.Join(filepath.Dir(exe), "modbussim.log")
}

// defaultConfigsDir returns a configs directory path next to the running binary.
func defaultConfigsDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "./configs"
	}
	return filepath.Join(filepath.Dir(exe), "configs")
}
