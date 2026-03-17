package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/hound-fi/api/internal/aggregator"
	"github.com/hound-fi/api/internal/config"
	"github.com/hound-fi/api/internal/database"
	"go.uber.org/zap"
)

type Handler struct {
	db  *database.DB
	agg *aggregator.Router
	log *zap.Logger
	cfg *config.Config
}

func New(db *database.DB, agg *aggregator.Router, log *zap.Logger, cfg *config.Config) *Handler {
	return &Handler{db: db, agg: agg, log: log, cfg: cfg}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{
		"error_code":    code,
		"error_message": message,
	})
}
