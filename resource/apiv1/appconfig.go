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
	ThemeColors          ThemeColors `json:"theme_colors"`
	LogoURL              string      `json:"logo_url"`
	StripePublishableKey string      `json:"stripe_publishable_key"`
	GivingContacts       []string    `json:"giving_contacts"`
	Giving               GivingCfg   `json:"giving"`
	Features             AppFeatures `json:"features"`
	ServerVersion        string      `json:"server_version"`
}

// ThemeColors gives the app real colors to theme with — the bare theme *name*
// is a CSS class the app can do nothing with. Values are hex strings.
// Site config (mobile.theme_colors) overrides; otherwise the framework's
// built-in palette for the theme name applies (the palettes the sites share —
// see the site stylus _styl/themes/*.styl); unknown themes get cobalt's.
type ThemeColors struct {
	Primary   string `json:"primary"`   // structural (app bars, nav)
	Secondary string `json:"secondary"` // accent (buttons, highlights)
	Surface   string `json:"surface"`   // page/card background
}

// GivingCfg is the giving-sheet metadata for the mobile payment flow.
// suggested_amounts_cents is never null; merchant ids empty = wallets off.
type GivingCfg struct {
	SuggestedAmountsCents []int64 `json:"suggested_amounts_cents"`
	AppleMerchantID       string  `json:"apple_merchant_id"`
	GooglePayMerchantID   string  `json:"google_pay_merchant_id"`
	CountryCode           string  `json:"country_code"`
	Currency              string  `json:"currency"`
}

// builtinThemeColors mirrors the anchor colors of the six shared site themes
// (primary = theme-nav-bgcolor, secondary = theme-accent-bgcolor,
// surface = theme-page-bgcolor). Kept here rather than derived because the
// compiled site CSS is not parseable at runtime and the palettes are stable.
var builtinThemeColors = map[string]ThemeColors{
	"cobalt":     {Primary: "#9eaac2", Secondary: "#6794c3", Surface: "#f9f9f9"},
	"sanctuary":  {Primary: "#8d545c", Secondary: "#6b2531", Surface: "#f7f4ed"},
	"fellowship": {Primary: "#55a49d", Secondary: "#e2725b", Surface: "#f8f6f2"},
	"graphite":   {Primary: "#565d64", Secondary: "#3a4046", Surface: "#f4f5f6"},
	"horizon":    {Primary: "#24466e", Secondary: "#1d84b5", Surface: "#f6f9fb"},
	"willow":     {Primary: "#7f8d6b", Secondary: "#66754f", Surface: "#f6f5f0"},
}

// resolveThemeColors applies the precedence: per-field config override >
// built-in palette for the theme name > cobalt.
func resolveThemeColors(theme string) ThemeColors {
	tc, ok := builtinThemeColors[theme]
	if !ok {
		tc = builtinThemeColors["cobalt"]
	}
	over := config.Options.Mobile.ThemeColors
	if over.Primary != "" {
		tc.Primary = over.Primary
	}
	if over.Secondary != "" {
		tc.Secondary = over.Secondary
	}
	if over.Surface != "" {
		tc.Surface = over.Surface
	}
	return tc
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

	amounts := opts.Mobile.SuggestedAmountsCents
	if len(amounts) == 0 {
		amounts = []int64{2500, 5000, 10000, 25000}
	}
	country := opts.Mobile.CountryCode
	if country == "" {
		country = "US"
	}
	currency := opts.Mobile.Currency
	if currency == "" {
		currency = "usd"
	}

	return ctx.WriteJSON(AppConfig{
		// CopyrightOwner is the site's plain-text name (banner_inner_html is
		// HTML and unusable as a label in a native UI).
		ChurchName:           opts.CopyrightOwner,
		Theme:                opts.Theme,
		ThemeColors:          resolveThemeColors(opts.Theme),
		LogoURL:              opts.Mobile.LogoURL,
		StripePublishableKey: opts.Stripe.PubKey,
		GivingContacts:       contacts,
		Giving: GivingCfg{
			SuggestedAmountsCents: amounts,
			AppleMerchantID:       opts.Mobile.AppleMerchantID,
			GooglePayMerchantID:   opts.Mobile.GooglePayMerchantID,
			CountryCode:           country,
			Currency:              currency,
		},
		Features: AppFeatures{
			Giving:      opts.Stripe.PubKey != "" && opts.Stripe.PrivKey != "",
			SermonAudio: opts.IDrive.Enabled,
			Chat:        true,
			PrayerWall:  true,
		},
		ServerVersion: config.Version,
	})
}
