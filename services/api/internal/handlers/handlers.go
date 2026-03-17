package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hound-fi/api/internal/aggregator"
	"github.com/hound-fi/api/internal/config"
	"github.com/hound-fi/api/internal/database"
	"github.com/hound-fi/api/internal/encryption"
	"github.com/hound-fi/api/internal/models"
	"go.uber.org/zap"
)

type Handler struct {
	db  *database.DB
	agg *aggregator.Router
	log *zap.Logger
	cfg *config.Config
	enc *encryption.Encryptor
}

func New(db *database.DB, agg *aggregator.Router, log *zap.Logger, cfg *config.Config) *Handler {
	enc, err := encryption.New(cfg.EncryptionKey)
	if err != nil {
		// EncryptionKey is validated at startup; panic here is intentional
		panic(fmt.Sprintf("failed to init encryptor: %v", err))
	}
	return &Handler{db: db, agg: agg, log: log, cfg: cfg, enc: enc}
}

// decryptItem replaces item.ProviderItemID with the decrypted access token so
// the aggregator can use it as a bearer token. The item is mutated in place.
func (h *Handler) decryptItem(item *models.Item) error {
	plain, err := h.enc.Decrypt(item.ProviderItemID)
	if err != nil {
		return fmt.Errorf("decrypt provider token: %w", err)
	}
	item.ProviderItemID = plain
	return nil
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
