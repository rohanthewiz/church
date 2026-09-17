// Package formdraft carries a refused form's typed values back to the form.
//
// Admin saves follow post/redirect/get: a refused save redirects to the form
// with a flash, and the form GET renders from the database. Without this,
// everything typed since the form was opened was lost on any refusal (blank
// title, bad date, expired token). Now:
//
//	POST /admin/articles/update/7 ──► refused ──► Save(ctx, "/admin/articles/edit/7", presenter)
//	                                             303 ─► GET /admin/articles/edit/7
//	basectlr render ──► Take(ctx, request path) ──► params["_global"][ParamKey] = JSON
//	article form module ──► FromParams(params, &pres) ──► renders the draft
//
// Design choices:
//   - Stored in the in-process kvstore, not a cookie: an article body is far
//     larger than a cookie can hold, and a draft never needs to leave the server.
//   - Keyed by session cookie AND form path, so a draft only ever returns to
//     the browser session that typed it, and only on the form it came from.
//   - One-shot: Take deletes it. A reload after that shows the stored record
//     again, which is the way out if the admin wants to discard their edits.
//   - Short TTL: a draft nobody came back for expires on its own.
//   - Cookies are SameSite=Lax (rweb's default), so a cross-site POST carries
//     no session cookie and can't plant a draft in someone's form.
//
// Callers must strip secrets before Save (the user form's password fields).
package formdraft

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/rohanthewiz/church/core/kvstore"
	"github.com/rohanthewiz/church/resource/session"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/rweb"
)

// ParamKey is the params["_global"] key a draft rides to form modules on.
const ParamKey = "form_draft"

// ttl bounds how long a draft waits for its redirect. The redirect follows
// immediately, so this only needs to outlast a slow round trip.
const ttl = 5 * time.Minute

const keyPrefix = "formdraft:"

// key scopes a draft to one session and one form path. Query strings are not
// part of a form's identity, so they are dropped.
func key(sessionKey, formPath string) string {
	if i := strings.IndexByte(formPath, '?'); i >= 0 {
		formPath = formPath[:i]
	}
	return keyPrefix + sessionKey + ":" + formPath
}

// sessionKey returns the request's session cookie, or "" when there is none
// (no draft can be scoped, so none is kept).
func sessionKey(ctx rweb.Context) string {
	k, err := ctx.GetCookie(session.CookieName)
	if err != nil {
		return ""
	}
	return k
}

// Save keeps v (JSON-encoded) as the draft for formPath in this session.
// Failure is logged and otherwise ignored: losing a draft is the old
// behaviour, and never a reason to fail the redirect.
func Save(ctx rweb.Context, formPath string, v any) {
	sk := sessionKey(ctx)
	if sk == "" {
		return
	}
	byts, err := json.Marshal(v)
	if err != nil {
		logger.LogErr(err, "Could not encode form draft", "form", formPath)
		return
	}
	if err := kvstore.Set(key(sk, formPath), string(byts), ttl); err != nil {
		logger.LogErr(err, "Could not store form draft", "form", formPath)
	}
}

// Take returns and deletes the draft for formPath in this session, or "".
func Take(ctx rweb.Context, formPath string) string {
	sk := sessionKey(ctx)
	if sk == "" {
		return ""
	}
	k := key(sk, formPath)
	draft, err := kvstore.Get(k)
	if err != nil {
		return "" // none: the normal case for every render
	}
	_ = kvstore.Del(k)
	return draft
}

// FromParams decodes the draft handed to a module into v, reporting whether
// there was one. A draft that doesn't decode is treated as absent.
func FromParams(params map[string]map[string]string, v any) bool {
	raw := params["_global"][ParamKey]
	if raw == "" {
		return false
	}
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		logger.LogErr(err, "Could not decode form draft; rendering the stored record")
		return false
	}
	return true
}

// SameItem reports whether a draft's id belongs to the form being rendered:
// the item's id on an edit form (itemIDs from module options), or no id on a
// new form. The draft key already scopes by form path; this guards a form
// module rendered somewhere its path doesn't identify the item.
func SameItem(draftID string, itemIDs []int64) bool {
	draftID = strings.TrimSpace(draftID)
	if len(itemIDs) == 0 {
		return draftID == "" || draftID == "0"
	}
	return draftID == strconv.FormatInt(itemIDs[0], 10)
}
