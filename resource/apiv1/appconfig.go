package apiv1

import (
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/rweb"
)

// AppConfig is the boot payload for the mobile app: everything the client
// needs before it can render its first screen, fetched once at startup.
//
// It lives in apiv1 (not a resource package) because it is cross-resource,
// site-level configuration — the same reason the feed aggregate lives outside
// the individual resources. The handler reads only from config.Options, so it
// touches no database and works even when the DB is down (useful: the app can
// still learn the church name and show a branded error state).
//
// Contract notes (mirrors the discipline of the other /api/v1 DTOs):
//   - snake_case keys, stable once shipped — the Flutter side maps these
//     directly into a model with non-null fields.
//   - giving_contacts serializes as [] when unset, never null.
//   - Only the Stripe *publishable* key is exposed; it is designed to be
//     public (it can only create client-side tokens, never charges).
type AppConfig struct {
	ChurchName           string      `json:"church_name"`
	Theme                string      `json:"theme"`
	StripePublishableKey string      `json:"stripe_publishable_key"`
	GivingContacts       []string    `json:"giving_contacts"`
	Features             AppFeatures `json:"features"`
	ServerVersion        string      `json:"server_version"`
}

// AppFeatures are per-site capability flags so one app binary can serve any
// church on this platform: the client shows/hides whole sections based on
// what the site is actually configured for, instead of hitting endpoints
// that would 500 for a site without (say) Stripe keys.
type AppFeatures struct {
	// Giving requires both Stripe keys: the publishable key for the client
	// SDK and the private key server-side for create-intent. Either missing
	// means the flow cannot complete, so advertise it only when whole.
	Giving bool `json:"giving"`
	// SermonAudio tracks whether the site has media storage (IDrive e2)
	// configured — without it /sermon-audio/* cannot serve files.
	SermonAudio bool `json:"sermon_audio"`
	// Chat and PrayerWall ship with the server and need only the login the
	// app already has, so they are advertised unconditionally — the flags
	// exist so a future per-site opt-out is a config change, not an API
	// contract change.
	Chat       bool `json:"chat"`
	PrayerWall bool `json:"prayer_wall"`
}

// APIAppConfigRWeb handles GET /api/v1/app-config.
// Public and unauthenticated by design: the app needs this before any login,
// and nothing in it is secret.
func APIAppConfigRWeb(ctx rweb.Context) error {
	opts := config.Options

	contacts := opts.GivingContacts
	if contacts == nil {
		contacts = []string{}
	}

	return ctx.WriteJSON(AppConfig{
		// CopyrightOwner is the site's plain-text name (banner_inner_html is
		// HTML and unusable as a label in a native UI).
		ChurchName:           opts.CopyrightOwner,
		Theme:                opts.Theme,
		StripePublishableKey: opts.Stripe.PubKey,
		GivingContacts:       contacts,
		Features: AppFeatures{
			Giving:      opts.Stripe.PubKey != "" && opts.Stripe.PrivKey != "",
			SermonAudio: opts.IDrive.Enabled,
			Chat:        true,
			PrayerWall:  true,
		},
		ServerVersion: config.Version,
	})
}
