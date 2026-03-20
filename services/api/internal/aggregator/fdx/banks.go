package fdx

import (
	"fmt"
	"os"
)

// BankDef describes an FDX-capable financial institution.
type BankDef struct {
	InstitutionID   string   // matches the DB institution id, e.g. "chase"
	Name            string
	AuthURL         string   // OAuth 2.0 authorization endpoint
	TokenURL        string   // OAuth 2.0 token endpoint
	APIURL          string   // FDX data API base (e.g. "https://api.chase.com/fdx/v6")
	FDXVersion      string   // "5.0" or "6.0"
	Scopes          []string
	UsePKCE         bool
	clientIDKey     string   // env var name for client ID
	clientSecretKey string   // env var name for client secret
}

func (b *BankDef) ClientID() string     { return os.Getenv(b.clientIDKey) }
func (b *BankDef) ClientSecret() string { return os.Getenv(b.clientSecretKey) }
func (b *BankDef) Configured() bool     { return b.ClientID() != "" }

var defaultScopes = []string{"openid", "accounts", "transactions", "customer_info"}

var registry = []BankDef{
	// Top 10 US banks by deposit volume — institution IDs match the DB seed in migrations/002.
	// All OAuth endpoints are configurable via env vars; defaults are best-effort from
	// each bank's public developer documentation.
	{
		InstitutionID:   "chase",
		Name:            "Chase",
		AuthURL:         envOr("FDX_CHASE_AUTH_URL", "https://auth.chase.com/oauth2/authorize"),
		TokenURL:        envOr("FDX_CHASE_TOKEN_URL", "https://auth.chase.com/oauth2/token"),
		APIURL:          envOr("FDX_CHASE_API_URL", "https://api.chase.com/fdx/v6"),
		FDXVersion:      "6.0",
		Scopes:          defaultScopes,
		UsePKCE:         true,
		clientIDKey:     "FDX_CHASE_CLIENT_ID",
		clientSecretKey: "FDX_CHASE_CLIENT_SECRET",
	},
	{
		InstitutionID:   "bofa",
		Name:            "Bank of America",
		AuthURL:         envOr("FDX_BOFA_AUTH_URL", "https://api.bankofamerica.com/oauth2/authorize"),
		TokenURL:        envOr("FDX_BOFA_TOKEN_URL", "https://api.bankofamerica.com/oauth2/token"),
		APIURL:          envOr("FDX_BOFA_API_URL", "https://api.bankofamerica.com/fdx/v6"),
		FDXVersion:      "6.0",
		Scopes:          defaultScopes,
		UsePKCE:         true,
		clientIDKey:     "FDX_BOFA_CLIENT_ID",
		clientSecretKey: "FDX_BOFA_CLIENT_SECRET",
	},
	{
		InstitutionID:   "wellsfargo",
		Name:            "Wells Fargo",
		AuthURL:         envOr("FDX_WF_AUTH_URL", "https://api.wellsfargo.com/oauth2/v1/authorize"),
		TokenURL:        envOr("FDX_WF_TOKEN_URL", "https://api.wellsfargo.com/oauth2/v1/token"),
		APIURL:          envOr("FDX_WF_API_URL", "https://api.wellsfargo.com/fdx/v6"),
		FDXVersion:      "6.0",
		Scopes:          defaultScopes,
		UsePKCE:         true,
		clientIDKey:     "FDX_WF_CLIENT_ID",
		clientSecretKey: "FDX_WF_CLIENT_SECRET",
	},
	{
		InstitutionID:   "capitalonebank",
		Name:            "Capital One",
		AuthURL:         envOr("FDX_C1_AUTH_URL", "https://api.capitalone.com/oauth2/authorize"),
		TokenURL:        envOr("FDX_C1_TOKEN_URL", "https://api.capitalone.com/oauth2/token"),
		APIURL:          envOr("FDX_C1_API_URL", "https://api.capitalone.com/fdx/v6"),
		FDXVersion:      "6.0",
		Scopes:          defaultScopes,
		UsePKCE:         true,
		clientIDKey:     "FDX_C1_CLIENT_ID",
		clientSecretKey: "FDX_C1_CLIENT_SECRET",
	},
	{
		InstitutionID:   "usbank",
		Name:            "U.S. Bank",
		AuthURL:         envOr("FDX_USB_AUTH_URL", "https://api.usbank.com/oauth2/authorize"),
		TokenURL:        envOr("FDX_USB_TOKEN_URL", "https://api.usbank.com/oauth2/token"),
		APIURL:          envOr("FDX_USB_API_URL", "https://api.usbank.com/fdx/v6"),
		FDXVersion:      "6.0",
		Scopes:          defaultScopes,
		UsePKCE:         true,
		clientIDKey:     "FDX_USB_CLIENT_ID",
		clientSecretKey: "FDX_USB_CLIENT_SECRET",
	},
	{
		InstitutionID:   "citibank",
		Name:            "Citi",
		AuthURL:         envOr("FDX_CITI_AUTH_URL", "https://api.citi.com/gcb/oauth2/authorize"),
		TokenURL:        envOr("FDX_CITI_TOKEN_URL", "https://api.citi.com/gcb/oauth2/token"),
		APIURL:          envOr("FDX_CITI_API_URL", "https://api.citi.com/fdx/v6"),
		FDXVersion:      "6.0",
		Scopes:          defaultScopes,
		UsePKCE:         true,
		clientIDKey:     "FDX_CITI_CLIENT_ID",
		clientSecretKey: "FDX_CITI_CLIENT_SECRET",
	},
	{
		InstitutionID:   "pnc",
		Name:            "PNC Bank",
		AuthURL:         envOr("FDX_PNC_AUTH_URL", "https://api.pnc.com/oauth2/authorize"),
		TokenURL:        envOr("FDX_PNC_TOKEN_URL", "https://api.pnc.com/oauth2/token"),
		APIURL:          envOr("FDX_PNC_API_URL", "https://api.pnc.com/fdx/v6"),
		FDXVersion:      "6.0",
		Scopes:          defaultScopes,
		UsePKCE:         true,
		clientIDKey:     "FDX_PNC_CLIENT_ID",
		clientSecretKey: "FDX_PNC_CLIENT_SECRET",
	},
	{
		InstitutionID:   "tdbank",
		Name:            "TD Bank",
		AuthURL:         envOr("FDX_TD_AUTH_URL", "https://api.td.com/oauth2/authorize"),
		TokenURL:        envOr("FDX_TD_TOKEN_URL", "https://api.td.com/oauth2/token"),
		APIURL:          envOr("FDX_TD_API_URL", "https://api.td.com/fdx/v6"),
		FDXVersion:      "6.0",
		Scopes:          defaultScopes,
		UsePKCE:         true,
		clientIDKey:     "FDX_TD_CLIENT_ID",
		clientSecretKey: "FDX_TD_CLIENT_SECRET",
	},
	{
		InstitutionID:   "truist",
		Name:            "Truist",
		AuthURL:         envOr("FDX_TRUIST_AUTH_URL", "https://api.truist.com/oauth2/authorize"),
		TokenURL:        envOr("FDX_TRUIST_TOKEN_URL", "https://api.truist.com/oauth2/token"),
		APIURL:          envOr("FDX_TRUIST_API_URL", "https://api.truist.com/fdx/v6"),
		FDXVersion:      "6.0",
		Scopes:          defaultScopes,
		UsePKCE:         true,
		clientIDKey:     "FDX_TRUIST_CLIENT_ID",
		clientSecretKey: "FDX_TRUIST_CLIENT_SECRET",
	},
	{
		InstitutionID:   "regions",
		Name:            "Regions Bank",
		AuthURL:         envOr("FDX_REGIONS_AUTH_URL", "https://api.regions.com/oauth2/authorize"),
		TokenURL:        envOr("FDX_REGIONS_TOKEN_URL", "https://api.regions.com/oauth2/token"),
		APIURL:          envOr("FDX_REGIONS_API_URL", "https://api.regions.com/fdx/v6"),
		FDXVersion:      "6.0",
		Scopes:          defaultScopes,
		UsePKCE:         true,
		clientIDKey:     "FDX_REGIONS_CLIENT_ID",
		clientSecretKey: "FDX_REGIONS_CLIENT_SECRET",
	},
}

// Lookup returns a pointer to the BankDef for the given institution ID,
// or an error if the institution is not in the FDX registry.
func Lookup(institutionID string) (*BankDef, error) {
	for i := range registry {
		if registry[i].InstitutionID == institutionID {
			return &registry[i], nil
		}
	}
	return nil, fmt.Errorf("fdx: institution %q not in registry", institutionID)
}

// ConfiguredBanks returns all banks that have at least a client ID set in the environment.
func ConfiguredBanks() []*BankDef {
	var out []*BankDef
	for i := range registry {
		if registry[i].Configured() {
			out = append(out, &registry[i])
		}
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
