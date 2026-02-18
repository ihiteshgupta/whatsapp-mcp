// Package main is the entry point for the WhatsApp bridge.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/config"
	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/health"
	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/state"
	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/store"
	"github.com/hiteshgupta/whatsapp-bridge-v2/internal/whatsapp"
	"github.com/hiteshgupta/whatsapp-bridge-v2/pkg/api"
	"github.com/hiteshgupta/whatsapp-bridge-v2/pkg/mcp"
	"github.com/mdp/qrterminal/v3"
	"github.com/skip2/go-qrcode"
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
		logHandler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	} else {
		logHandler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
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

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize WhatsApp client
	waConfig := &whatsapp.Config{
		StorePath: cfg.SessionPath,
		StateMgr:  sm,
	}
	waClient, err := whatsapp.NewClient(ctx, waConfig, logger)
	if err != nil {
		logger.Error("Failed to create WhatsApp client", "error", err)
		os.Exit(1)
	}
	defer waClient.Disconnect()

	// Get QR channel before connecting
	qrChan := waClient.GetQRChannel()

	// Connect to WhatsApp in background
	go func() {
		if err := waClient.Connect(ctx); err != nil {
			logger.Error("WhatsApp connection error", "error", err)
		}
	}()

	// Handle QR codes in background - save to file and print to stderr
	qrFilePath := filepath.Join(filepath.Dir(cfg.StorePath), "qrcode.png")
	go func() {
		for qr := range qrChan {
			// Save QR code as PNG image file
			if err := qrcode.WriteFile(qr, qrcode.Medium, 256, qrFilePath); err == nil {
				logger.Info("QR code saved to file - open this file to scan",
					"path", qrFilePath,
				)
				fmt.Fprintf(os.Stderr, "\n╔══════════════════════════════════════════════════════╗\n")
				fmt.Fprintf(os.Stderr, "║  QR CODE SAVED - Open this file to scan with phone:  ║\n")
				fmt.Fprintf(os.Stderr, "║  %s\n", qrFilePath)
				fmt.Fprintf(os.Stderr, "╚══════════════════════════════════════════════════════╝\n\n")
			} else {
				logger.Error("Failed to save QR code to file", "error", err)
			}

			// Also print to stderr for terminals that support it
			fmt.Fprintln(os.Stderr, "╔══════════════════════════════════════════╗")
			fmt.Fprintln(os.Stderr, "║  Scan this QR code with WhatsApp Mobile  ║")
			fmt.Fprintln(os.Stderr, "╚══════════════════════════════════════════╝")
			qrterminal.GenerateHalfBlock(qr, qrterminal.L, os.Stderr)
			fmt.Fprintln(os.Stderr, "")
		}
	}()

	// Initialize API handler with WhatsApp client
	handler := api.NewHandler(storeDB, hm, waClient, sm)

	// Initialize MCP server with stdio transport
	mcpServer := mcp.NewServer(os.Stdin, os.Stdout, handler, logger)

	logger.Info("Bridge initialized",
		"store_path", cfg.StorePath,
		"session_path", cfg.SessionPath,
		"state", sm.MustState(),
	)

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
