// Command zvec-httpd is a lightweight HTTP service for managing one or more
// zvec collections/tables with HTTP Basic authentication.
//
// Usage:
//
//	zvec-httpd -config config.json
//
// It also provides a helper to generate config password hashes:
//
//	zvec-httpd -hash-password "mysecret"
//
// The service is intentionally dependency-free (Go standard library only).
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/oliveagle/zvec-go/server"
)

func main() {
	configPath := flag.String("config", "config.json", "path to the JSON configuration file")
	logLevel := flag.String("log-level", "info", "log level: debug, info, warn, or error")
	hashPassword := flag.String("hash-password", "", "print the sha256:<hex> hash of the given password and exit")
	flag.Parse()

	if *hashPassword != "" {
		sum := sha256.Sum256([]byte(*hashPassword))
		fmt.Printf("sha256:%s\n", hex.EncodeToString(sum[:]))
		return
	}

	logger := newLogger(*logLevel)
	slog.SetDefault(logger)

	cfg, err := server.LoadConfig(*configPath)
	if err != nil {
		logger.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	srv, err := server.New(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize server", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout())
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown error", "err", err)
		}
	case err := <-errCh:
		if err != nil {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}
}

func newLogger(level string) *slog.Logger {
	lvl := slog.LevelInfo
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}
