// Package main is the entry point for the WhatsApp bridge.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/config"
	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/health"
	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/state"
	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/store"
	"github.com/hiteshgupta/whatsapp-bridge-v2/pkg/api"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to config file")
	logLevel   = flag.String("log-level", "", "Log level (debug, info, warn, error)")
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Override log level from flag if provided
	if *logLevel != "" {
		cfg.LogLevel = *logLevel
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid config: %v\n", err)
		os.Exit(1)
	}

	// Setup logging
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	if cfg.LogFormat == "text" {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	logger := slog.New(handler)
	slog.SetDefault(logger)

	logger.Info("WhatsApp Bridge V2 starting",
		"config", *configPath,
		"log_level", cfg.LogLevel,
	)

	// Initialize store
	storeDB, err := store.NewSQLiteStore(cfg.StorePath)
	if err != nil {
		logger.Error("Failed to initialize store", "error", err)
		os.Exit(1)
	}
	defer storeDB.Close()

	// Initialize state machine
	sm := state.NewMachine()

	// Initialize health monitor
	hm := health.NewMonitor(cfg, sm)
	hm.Start()
	defer hm.Stop()

	// Initialize MCP server
	mcpServer := api.NewMCPServer(storeDB, sm, hm)
	_ = mcpServer // Will be used for MCP protocol handling

	logger.Info("Bridge initialized",
		"store_path", cfg.StorePath,
		"state", sm.MustState(),
	)

	// Wait for shutdown signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		logger.Info("Received shutdown signal", "signal", sig)
	case <-ctx.Done():
		logger.Info("Context cancelled")
	}

	// Graceful shutdown
	if err := sm.Fire(context.Background(), state.TriggerShutdown); err != nil {
		logger.Warn("Failed to transition to shutting down", "error", err)
	}

	logger.Info("WhatsApp Bridge V2 stopped")
}
