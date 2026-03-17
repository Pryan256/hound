package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hound-fi/api/internal/config"
	"github.com/hound-fi/api/internal/database"
	"github.com/hound-fi/api/internal/server"
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

	srv := server.New(cfg, db, log)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      srv,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("starting server", zap.String("port", cfg.Port), zap.String("env", cfg.Env))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatal("forced shutdown", zap.Error(err))
	}

	log.Info("server stopped")
}
