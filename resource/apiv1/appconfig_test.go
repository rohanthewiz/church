package apiv1

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/rweb"
)

// withConfig installs a test EnvConfig and restores the prior one afterward.
// config.Options is process-global (set once by InitConfig in production), so
// tests must save/restore rather than assume a clean slate.
func withConfig(t *testing.T, cfg *config.EnvConfig) {
	t.Helper()
	prev := config.Options
	config.Options = cfg
	t.Cleanup(func() { config.Options = prev })
}

func appConfigServer() *rweb.Server {
	s := rweb.NewServer(rweb.ServerOptions{})
	s.Get("/api/v1/app-config", APIAppConfigRWeb)
	return s
}

// The app hard-maps this payload at boot; key names, nesting, and the
// never-null contacts array are contract.
func TestAppConfigContract(t *testing.T) {
	cfg := &config.EnvConfig{}
	cfg.Theme = "cobalt"
	cfg.CopyrightOwner = "Community Church"
	cfg.Stripe.PubKey = "pk_test_123"
	cfg.Stripe.PrivKey = "sk_test_456"
	cfg.IDrive.Enabled = true
	cfg.GivingContacts = []string{"treasurer@example.org"}
	withConfig(t, cfg)

	resp := appConfigServer().Request("GET", "/api/v1/app-config", nil, nil)
	if resp.Status() != 200 {
		t.Fatalf("status = %d, want 200", resp.Status())
	}

	var got struct {
		ChurchName           string   `json:"church_name"`
		Theme                string   `json:"theme"`
		StripePublishableKey string   `json:"stripe_publishable_key"`
		GivingContacts       []string `json:"giving_contacts"`
		Features             struct {
			Giving      bool `json:"giving"`
			SermonAudio bool `json:"sermon_audio"`
		} `json:"features"`
		ServerVersion string `json:"server_version"`
	}
	if err := json.Unmarshal(resp.Body(), &got); err != nil {
		t.Fatalf("response is not JSON: %v\nbody: %s", err, resp.Body())
	}

	if got.ChurchName != "Community Church" {
		t.Errorf("church_name = %q", got.ChurchName)
	}
	if got.Theme != "cobalt" {
		t.Errorf("theme = %q", got.Theme)
	}
	if got.StripePublishableKey != "pk_test_123" {
		t.Errorf("stripe_publishable_key = %q", got.StripePublishableKey)
	}
	if !got.Features.Giving || !got.Features.SermonAudio {
		t.Errorf("features = %+v, want both true", got.Features)
	}
	if len(got.GivingContacts) != 1 || got.GivingContacts[0] != "treasurer@example.org" {
		t.Errorf("giving_contacts = %v", got.GivingContacts)
	}

	// The private key must never appear anywhere in the payload.
	if strings.Contains(string(resp.Body()), "sk_test_456") {
		t.Errorf("stripe private key leaked into app-config: %s", resp.Body())
	}
}

// Giving requires BOTH Stripe keys; a site with only the publishable key
// configured (server can't create intents) must not advertise the feature.
func TestAppConfigGivingRequiresBothKeys(t *testing.T) {
	cfg := &config.EnvConfig{}
	cfg.Stripe.PubKey = "pk_test_123" // PrivKey empty
	withConfig(t, cfg)

	resp := appConfigServer().Request("GET", "/api/v1/app-config", nil, nil)
	var got struct {
		Features struct {
			Giving bool `json:"giving"`
		} `json:"features"`
	}
	if err := json.Unmarshal(resp.Body(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Features.Giving {
		t.Error("features.giving = true with no private key configured")
	}
}

// Unset giving_contacts must serialize as [], not null — the Dart client
// iterates the list without a null check (same rule as the list envelopes).
func TestAppConfigContactsNeverNull(t *testing.T) {
	withConfig(t, &config.EnvConfig{})

	resp := appConfigServer().Request("GET", "/api/v1/app-config", nil, nil)
	if !strings.Contains(string(resp.Body()), `"giving_contacts":[]`) {
		t.Errorf("giving_contacts should be [] when unset, body: %s", resp.Body())
	}
}
