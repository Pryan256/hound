package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hound-fi/api/internal/aggregator"
	"github.com/hound-fi/api/internal/aggregator/akoya"
	"github.com/hound-fi/api/internal/aggregator/finicity"
	"github.com/hound-fi/api/internal/aggregator/sandbox"
	"github.com/hound-fi/api/internal/config"
	"github.com/hound-fi/api/internal/database"
	"github.com/hound-fi/api/internal/encryption"
	"github.com/hound-fi/api/internal/ratelimit"
	"github.com/hound-fi/api/internal/refresh"
	"github.com/hound-fi/api/internal/server"
	"github.com/hound-fi/api/internal/webhook"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()

	var log *zap.Logger
	var err error
	if cfg.Env == "production" {
		log, err = zap.NewProduction()
	} else {
		log, err = zap.NewDevelopment()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		log.Fatal("failed to run migrations", zap.Error(err))
	}

	// Build the aggregator router once — shared by the HTTP server and the refresher.
	// Sandbox is always registered; it only activates for ins_sandbox institutions.
	agg := aggregator.NewRouterWithLogger(log,
		sandbox.New(),
		akoya.New(cfg.Akoya),
		finicity.New(cfg.Finicity),
	)

	// Encryptor — also shared with the refresher (needs to decrypt/re-encrypt tokens).
	enc, err := encryption.New(cfg.EncryptionKey)
	if err != nil {
		log.Fatal("failed to init encryptor", zap.Error(err))
	}

	// Webhook dispatcher — shared by the HTTP server and the token refresher.
	webhooks := webhook.New(db, enc, log)

	// Rate limiter — Redis-backed fixed-window, fails open if Redis is down.
	limiter, err := ratelimit.New(cfg.RedisURL)
	if err != nil {
		log.Warn("rate limiter unavailable, proceeding without rate limiting", zap.Error(err))
		limiter = nil
	}
	if limiter != nil {
		defer limiter.Close()
	}

	srv := server.New(cfg, db, agg, webhooks, limiter, log)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      srv,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Root context — cancelled on SIGINT/SIGTERM to trigger graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start background jobs.
	go webhooks.Start(ctx)
	go refresh.New(db, agg, enc, log, webhooks).Start(ctx)

	// Start HTTP server.
	go func() {
		log.Info("starting server", zap.String("port", cfg.Port), zap.String("env", cfg.Env))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	// Block until signal received.
	<-ctx.Done()

	log.Info("shutting down server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatal("forced shutdown", zap.Error(err))
	}

	log.Info("server stopped")
}
