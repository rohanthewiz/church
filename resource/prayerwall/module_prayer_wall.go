package prayerwall

import (
	"strconv"

	"github.com/rohanthewiz/church/app"
	"github.com/rohanthewiz/church/config"
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/module"
	"github.com/rohanthewiz/church/resource/chat"
	"github.com/rohanthewiz/church/resource/user"
	"github.com/rohanthewiz/element"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

const ModuleTypePrayerWall = "prayer_wall"

// ModulePrayerWall renders the wall: a submission form (logged-in members),
// the request feed, and a live chat discussion strip at the bottom — the
// chat module in its embedded role.
//
// Unlike the JS-hydrated chat widget, the wall itself is server-rendered:
// requests are durable content, and classic form posts (with the site's CSRF
// token + flash redirect) match how the rest of the site mutates data.
type ModulePrayerWall struct {
	module.Presenter
	csrf string
}

func NewModulePrayerWall(pres module.Presenter) (module.Module, error) {
	mod := new(ModulePrayerWall)
	mod.Name = pres.Name
	mod.Opts = pres.Opts
	if mod.Opts.Limit < 1 {
		mod.Opts.Limit = 25
	}
	csrf, err := app.GenerateFormToken()
	if err != nil {
		return nil, serr.Wrap(err, "Could not generate form token for prayer wall")
	}
	mod.csrf = csrf
	return module.Module(mod), nil
}

// channel is the wall discussion's chat channel — overridable per placement
// via ItemSlug so two wall pages could hold distinct conversations.
func (m ModulePrayerWall) channel() string {
	if m.Opts.ItemSlug != "" && chat.ValidChannel(m.Opts.ItemSlug) {
		return m.Opts.ItemSlug
	}
	return "prayer-wall"
}

func (m ModulePrayerWall) getData() ([]Request, error) {
	dbH, err := db.Db()
	if err != nil {
		return nil, serr.Wrap(err, "Could not obtain DB handle")
	}
	return ListRequests(dbH, int(m.Opts.Limit), int(m.Opts.Offset))
}

// viewer resolves what the rendering user may do. The username arrives via
// the _global render params (set in basectlr from the session); the role
// needs a DB lookup, accepted here because the module is already a DB round
// trip and only logged-in views pay it.
func (m ModulePrayerWall) viewer(params map[string]map[string]string) (username string, canModerate bool) {
	glob, ok := params["_global"]
	if !ok {
		return "", false
	}
	username = glob["username"]
	if username == "" {
		return "", false
	}
	dbH, err := db.Db()
	if err != nil {
		logger.LogErr(err, "prayer wall: could not obtain DB handle for viewer role")
		return username, false
	}
	au, found, err := user.AuthUserByUsername(dbH, username)
	if err != nil || !found {
		if err != nil {
			logger.LogErr(err, "prayer wall: could not load viewer role")
		}
		return username, false
	}
	return username, chat.CanModerate(au.Role)
}

func (m *ModulePrayerWall) Render(params map[string]map[string]string, loggedIn bool) string {
	if opts, ok := params[m.Opts.Slug]; ok { // params addressed to us
		m.SetLimitAndOffset(opts)
	}

	reqs, err := m.getData()
	if err != nil {
		logger.LogErr(err, "Error obtaining data in ModulePrayerWall")
		return ""
	}
	username, canModerate := m.viewer(params)

	title := m.Opts.Title
	if title == "" {
		title = "Prayer Wall"
	}

	b := element.NewBuilder()
	b.DivClass("ch-module-wrapper ch-"+m.Opts.ModuleType).R(
		b.DivClass("ch-module-heading").T(title),
		b.DivClass("ch-module-body").R(
			b.Wrap(func() {
				if loggedIn {
					m.renderForm(b)
				} else {
					b.PClass("pw-login-hint").R(
						b.A("href", "/login").T("Log in"),
						b.T(" to share a prayer request"),
					)
				}

				if len(reqs) == 0 {
					b.PClass("pw-empty").T("No prayer requests yet.")
				}
				for _, r := range reqs {
					m.renderRequest(b, r, username, canModerate)
				}
				b.T(m.RenderPagination(len(reqs)))
			}),
		),
		b.Style().T(wallCSS),
		b.Wrap(func() {
			// The live discussion strip — chat embedded beneath the wall.
			chat.RenderWidget(b, chat.WidgetCfg{
				Channel: m.channel(),
				Title:   "Live Discussion",
				Compact: true,
			})
		}),
	)
	return b.String()
}

// renderForm is the member submission form (classic POST + flash redirect).
func (m ModulePrayerWall) renderForm(b *element.Builder) {
	b.DetailsClass("pw-new").R(
		b.Summary().T("Share a prayer request"),
		b.Form("method", "post", "action", "/prayer-requests").R(
			b.Input("type", "hidden", "name", "csrf", "value", m.csrf),
			b.Input("type", "text", "name", "title", "maxlength", strconv.Itoa(MaxTitleLen),
				"placeholder", "Title", "required", "required"),
			b.TextArea("name", "body", "rows", "3", "maxlength", strconv.Itoa(MaxBodyLen),
				"placeholder", "How can we pray for you?", "required", "required").R(),
			b.Button("type", "submit").T("Post Request"),
		),
	)
}

// renderRequest renders one card, including the moderation / withdraw
// controls the viewer is entitled to (server re-verifies on post).
func (m ModulePrayerWall) renderRequest(b *element.Builder, r Request, username string, canModerate bool) {
	author := r.DisplayName
	if author == "" {
		author = r.Username
	}
	idStr := strconv.FormatInt(r.Id, 10)

	cardClass := "pw-request"
	if r.Answered {
		cardClass += " pw-answered"
	}
	b.DivClass(cardClass).R(
		b.DivClass("pw-req-head").R(
			b.SpanClass("pw-req-title").T(r.Title),
			b.Wrap(func() {
				if r.Answered {
					b.SpanClass("pw-badge").T("Answered")
				}
			}),
			b.SpanClass("pw-req-meta").T(" — "+author+", "+r.CreatedAt.Format(config.DisplayDateFormat)),
		),
		b.PClass("pw-req-body").T(r.Body),
		b.Wrap(func() {
			if r.Answered && r.AnsweredNote != "" {
				b.PClass("pw-req-note").T("Praise report: " + r.AnsweredNote)
			}

			if canModerate {
				b.FormClass("pw-inline", "method", "post", "action", "/prayer-requests/answered/"+idStr).R(
					b.Input("type", "hidden", "name", "csrf", "value", m.csrf),
					b.Wrap(func() {
						if r.Answered {
							b.Input("type", "hidden", "name", "answered", "value", "false")
							b.Button("type", "submit").T("Reopen")
						} else {
							b.Input("type", "hidden", "name", "answered", "value", "true")
							b.Input("type", "text", "name", "note", "placeholder", "Praise report (optional)")
							b.Button("type", "submit").T("Mark Answered")
						}
					}),
				)
			}
			if canModerate || (username != "" && username == r.Username) {
				label := "Remove"
				if !canModerate {
					label = "Withdraw" // the requester retracting their own
				}
				b.FormClass("pw-inline", "method", "post", "action", "/prayer-requests/delete/"+idStr,
					"onsubmit", "return confirm('Remove this prayer request?')").R(
					b.Input("type", "hidden", "name", "csrf", "value", m.csrf),
					b.Button("type", "submit").T(label),
				)
			}
		}),
	)
}

// wallCSS is scoped under the module wrapper class so it cannot leak into
// site themes (same approach as the chat widget's styles).
const wallCSS = `
.ch-prayer_wall .pw-new { margin: 0.5em 0 1em; }
.ch-prayer_wall .pw-new form { display: flex; flex-direction: column; gap: 0.4em; max-width: 34em; margin-top: 0.5em; }
.ch-prayer_wall .pw-request { border: 1px solid #e2e2e2; border-radius: 6px; padding: 0.6em 0.8em; margin: 0.6em 0; }
.ch-prayer_wall .pw-answered { background: #f2faf2; border-color: #cfe8cf; }
.ch-prayer_wall .pw-req-title { font-weight: bold; }
.ch-prayer_wall .pw-req-meta { color: #777; font-size: 0.9em; }
.ch-prayer_wall .pw-badge { background: #3a8a3a; color: #fff; border-radius: 4px; padding: 0 0.4em; margin-left: 0.5em; font-size: 0.8em; }
.ch-prayer_wall .pw-req-body { white-space: pre-wrap; margin: 0.4em 0; }
.ch-prayer_wall .pw-req-note { color: #2e6b2e; font-style: italic; }
.ch-prayer_wall .pw-inline { display: inline-block; margin-right: 0.5em; }
.ch-prayer_wall .pw-login-hint, .ch-prayer_wall .pw-empty { color: #666; }
`
