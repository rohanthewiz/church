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

// theme_colors resolves from the built-in palette by theme name, with
// per-field config overrides winning; unknown themes fall back to cobalt.
func TestAppConfigThemeColors(t *testing.T) {
	cfg := &config.EnvConfig{}
	cfg.Theme = "sanctuary"
	cfg.Mobile.ThemeColors.Secondary = "#123456" // override just one field
	withConfig(t, cfg)

	resp := appConfigServer().Request("GET", "/api/v1/app-config", nil, nil)
	var got struct {
		ThemeColors struct {
			Primary   string `json:"primary"`
			Secondary string `json:"secondary"`
			Surface   string `json:"surface"`
		} `json:"theme_colors"`
	}
	if err := json.Unmarshal(resp.Body(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ThemeColors.Primary != "#8d545c" {
		t.Errorf("primary = %q, want sanctuary's #8d545c", got.ThemeColors.Primary)
	}
	if got.ThemeColors.Secondary != "#123456" {
		t.Errorf("secondary = %q, want the config override", got.ThemeColors.Secondary)
	}
	if got.ThemeColors.Surface != "#f7f4ed" {
		t.Errorf("surface = %q, want sanctuary's #f7f4ed", got.ThemeColors.Surface)
	}
}

// Giving metadata defaults: suggested amounts never null, USD/US fallbacks.
func TestAppConfigGivingDefaults(t *testing.T) {
	withConfig(t, &config.EnvConfig{})

	resp := appConfigServer().Request("GET", "/api/v1/app-config", nil, nil)
	var got struct {
		Giving struct {
			SuggestedAmountsCents []int64 `json:"suggested_amounts_cents"`
			CountryCode           string  `json:"country_code"`
			Currency              string  `json:"currency"`
		} `json:"giving"`
	}
	if err := json.Unmarshal(resp.Body(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Giving.SuggestedAmountsCents) == 0 {
		t.Error("suggested_amounts_cents should default to a non-empty list")
	}
	if got.Giving.CountryCode != "US" || got.Giving.Currency != "usd" {
		t.Errorf("country/currency = %q/%q, want US/usd defaults",
			got.Giving.CountryCode, got.Giving.Currency)
	}
}

// ptr is the one-line helper the location tests need: config.MobileLocation
// holds *float64 so that an unset key is distinguishable from 0, which means
// every test that sets one has to take an address.
func ptr(v float64) *float64 { return &v }

// location is the payload the app reads to decide whether to draw a map, so
// both halves are contract: the flag, and the fields it guards.
func TestAppConfigLocation(t *testing.T) {
	cfg := &config.EnvConfig{}
	cfg.CopyrightOwner = "Community Church"
	cfg.Mobile.Location.Latitude = ptr(30.2672)
	cfg.Mobile.Location.Longitude = ptr(-97.7431)
	cfg.Mobile.Location.Aliases = []string{"Fellowship Hall"}
	withConfig(t, cfg)

	got := decodeLocation(t)
	if !got.Configured {
		t.Fatalf("configured = false, want true: %+v", got)
	}
	if got.Latitude != 30.2672 || got.Longitude != -97.7431 {
		t.Errorf("coordinates = %v, %v", got.Latitude, got.Longitude)
	}
	// An unset label falls back to the site's own name rather than to "",
	// so a configured map always has something to be announced as.
	if got.Label != "Community Church" {
		t.Errorf("label = %q, want the site name as the fallback", got.Label)
	}
	if len(got.Aliases) != 1 || got.Aliases[0] != "Fellowship Hall" {
		t.Errorf("aliases = %v", got.Aliases)
	}
}

// An unconfigured site must say so outright. The app cannot infer it: 0,0 is
// the Gulf of Guinea, not an absence — see the Location type.
func TestAppConfigLocationUnsetIsNotZeroZero(t *testing.T) {
	withConfig(t, &config.EnvConfig{})

	got := decodeLocation(t)
	if got.Configured {
		t.Error("configured = true with nothing configured")
	}
	if !strings.Contains(string(locationBody(t)), `"aliases":[]`) {
		t.Error("aliases should be [] when unset, never null")
	}
}

// Half a coordinate is a half-typed config, not a point.
func TestAppConfigLocationNeedsBothCoordinates(t *testing.T) {
	cfg := &config.EnvConfig{}
	cfg.Mobile.Location.Latitude = ptr(30.2672) // no longitude
	withConfig(t, cfg)

	if decodeLocation(t).Configured {
		t.Error("configured = true with only a latitude")
	}
}

// Zero is a real place and must survive as one — the whole reason the config
// fields are pointers. Longitude 0 is the Greenwich meridian; latitude 0 is
// the equator, and they cross on land in Ghana's own waters and in Ecuador.
func TestAppConfigLocationZeroIsAPlace(t *testing.T) {
	cfg := &config.EnvConfig{}
	cfg.Mobile.Location.Latitude = ptr(0)
	cfg.Mobile.Location.Longitude = ptr(0)
	withConfig(t, cfg)

	if !decodeLocation(t).Configured {
		t.Error("configured = false for an explicitly configured 0,0")
	}
}

// Out of range is a typo. Reporting it as unconfigured is the only answer
// that cannot draw a map of somewhere else — see resolveLocation.
func TestAppConfigLocationRejectsOutOfRange(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lat, lng float64
	}{
		{"latitude past the pole", 300, -97.7431},
		{"latitude just past 90", 90.1, 0},
		{"longitude past the antimeridian", 30.2672, 181},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.EnvConfig{}
			cfg.Mobile.Location.Latitude = ptr(tc.lat)
			cfg.Mobile.Location.Longitude = ptr(tc.lng)
			withConfig(t, cfg)

			if decodeLocation(t).Configured {
				t.Errorf("configured = true for %v, %v", tc.lat, tc.lng)
			}
		})
	}
}

func locationBody(t *testing.T) []byte {
	t.Helper()
	resp := appConfigServer().Request("GET", "/api/v1/app-config", nil, nil)
	if resp.Status() != 200 {
		t.Fatalf("status = %d, want 200", resp.Status())
	}
	return resp.Body()
}

// decodeLocation reads the location block back through keys spelled out here
// rather than through the apiv1.Location struct, which is what makes these
// contract tests: a field renamed on both sides still compiles and still
// round-trips, and would pass a test that reused the server's own type. These
// literals are the app's copy of the contract, standing in for it.
func decodeLocation(t *testing.T) locationWire {
	t.Helper()
	var got struct {
		Location locationWire `json:"location"`
	}
	body := locationBody(t)
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("response is not JSON: %v\nbody: %s", err, body)
	}
	return got.Location
}

// locationWire is the shape the mobile app declares for this block. Keep it
// spelled out, not aliased to Location.
type locationWire struct {
	Configured bool     `json:"configured"`
	Latitude   float64  `json:"latitude"`
	Longitude  float64  `json:"longitude"`
	Label      string   `json:"label"`
	Aliases    []string `json:"aliases"`
}
