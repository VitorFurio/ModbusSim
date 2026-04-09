package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"modbussim/internal/api"
	"modbussim/internal/config"
	"modbussim/internal/frontend"
	"modbussim/internal/modbus"
	"modbussim/internal/register"
)

func main() {
	cfgPath := flag.String("config", "", "path to YAML config file (optional)")
	versDir := flag.String("versions", "./configs", "directory for saved config versions")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Load or use default config.
	var cfg *config.Config
	if *cfgPath != "" {
		var err error
		cfg, err = config.Load(*cfgPath)
		if err != nil {
			slog.Error("failed to load config", "err", err)
			os.Exit(1)
		}
	} else {
		cfg = config.Default()
	}

	// Build engine.
	eng := register.NewEngine()
	for _, r := range cfg.Registers {
		if _, err := eng.Add(r); err != nil {
			slog.Warn("skipping register", "id", r.ID, "err", err)
		}
	}

	// Start engine.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	eng.Start(ctx)

	// Start Modbus TCP server.
	mbSrv := modbus.New(cfg.ModbusAddr, eng)
	if err := mbSrv.Start(); err != nil {
		slog.Error("failed to start modbus server", "err", err)
		os.Exit(1)
	}
	defer mbSrv.Stop()

	fmt.Println()
	fmt.Println("  ┌─────────────────────────────────────────────────────────┐")
	fmt.Println("  │             ModbusSim — Modbus TCP Simulator            │")
	fmt.Println("  └─────────────────────────────────────────────────────────┘")
	fmt.Printf("  Modbus TCP : %s\n", cfg.ModbusAddr)
	fmt.Printf("  Admin HTTP : http://localhost%s\n", cfg.AdminAddr)
	fmt.Println()
	fmt.Println("  Press Ctrl+C to stop.")
	fmt.Println()

	// Start HTTP+WS API server (blocks until ctx cancelled).
	apiSrv := api.NewServer(cfg.AdminAddr, eng, cfg, *versDir, frontend.FS())
	if err := apiSrv.Start(ctx); err != nil && err != http.ErrServerClosed {
		slog.Error("api server error", "err", err)
	}
}
