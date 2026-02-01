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
	"github.com/hiteshgupta/whatsapp-bridge-v2/pkg/mcp"
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

	var logHandler slog.Handler
	if cfg.LogFormat == "text" {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	logger := slog.New(logHandler)
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

	// Initialize API handler (bridge will be set when whatsmeow client is ready)
	handler := api.NewHandler(storeDB, hm, nil, sm)

	// Initialize MCP server with stdio transport
	mcpServer := mcp.NewServer(os.Stdin, os.Stdout, handler, logger)

	logger.Info("Bridge initialized",
		"store_path", cfg.StorePath,
		"state", sm.MustState(),
	)

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run MCP server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- mcpServer.Run(ctx)
	}()

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigChan:
		logger.Info("Received shutdown signal", "signal", sig)
		cancel() // Signal server to stop
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			logger.Error("MCP server error", "error", err)
		}
	}

	// Graceful shutdown
	if err := sm.Fire(context.Background(), state.TriggerShutdown); err != nil {
		logger.Warn("Failed to transition to shutting down", "error", err)
	}

	logger.Info("WhatsApp Bridge V2 stopped")
}
