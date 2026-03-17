package models

type Institution struct {
	ID           string   `json:"institution_id"`
	Name         string   `json:"name"`
	LogoURL      string   `json:"logo,omitempty"`
	PrimaryColor string   `json:"primary_color,omitempty"`
	URL          string   `json:"url,omitempty"`
	Products     []string `json:"products"`
	Status       string   `json:"status"`
	OAuthOnly    bool     `json:"oauth"`
}
