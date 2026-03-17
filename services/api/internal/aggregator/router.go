package aggregator

import (
	"context"
	"fmt"
	"time"

	"github.com/hound-fi/api/internal/models"
	"go.uber.org/zap"
)

// Router selects the appropriate Provider for each request.
// Priority: Akoya (OAuth-only, CFPB 1033 aligned) → Finicity (fallback)
type Router struct {
	providers []Provider
	log       *zap.Logger
}

func NewRouter(providers ...Provider) *Router {
	return &Router{providers: providers}
}

// NewRouter accepts a logger too
func NewRouterWithLogger(log *zap.Logger, providers ...Provider) *Router {
	return &Router{providers: providers, log: log}
}

// providerFor returns the registered provider matching the item's provider field.
func (r *Router) providerFor(item *models.Item) (Provider, error) {
	for _, p := range r.providers {
		if p.Name() == item.Provider {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no provider registered for: %s", item.Provider)
}

func (r *Router) GetAccounts(ctx context.Context, item *models.Item) ([]models.Account, error) {
	p, err := r.providerFor(item)
	if err != nil {
		return nil, err
	}
	return p.GetAccounts(ctx, item)
}

func (r *Router) GetAccountBalances(ctx context.Context, item *models.Item) ([]models.Account, error) {
	p, err := r.providerFor(item)
	if err != nil {
		return nil, err
	}
	return p.GetAccountBalances(ctx, item)
}

func (r *Router) GetTransactions(ctx context.Context, item *models.Item, start, end time.Time, count, offset int) (*models.TransactionsResponse, error) {
	p, err := r.providerFor(item)
	if err != nil {
		return nil, err
	}
	return p.GetTransactions(ctx, item, start, end, count, offset)
}

func (r *Router) GetIdentity(ctx context.Context, item *models.Item) ([]models.Account, error) {
	p, err := r.providerFor(item)
	if err != nil {
		return nil, err
	}
	return p.GetIdentity(ctx, item)
}

func (r *Router) RevokeItem(ctx context.Context, item *models.Item) error {
	p, err := r.providerFor(item)
	if err != nil {
		return err
	}
	return p.RevokeItem(ctx, item)
}

// SelectProvider picks the best provider for a new institution connection.
// Called during the Link flow to determine which backend to use.
func (r *Router) SelectProvider(ctx context.Context, institutionID string) (Provider, error) {
	for _, p := range r.providers {
		if p.Supports(ctx, institutionID) {
			if r.log != nil {
				r.log.Info("selected provider", zap.String("provider", p.Name()), zap.String("institution", institutionID))
			}
			return p, nil
		}
	}
	return nil, fmt.Errorf("no provider supports institution: %s", institutionID)
}
