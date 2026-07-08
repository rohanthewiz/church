// Package apiv1 holds cross-resource helpers for the /api/v1 JSON API consumed
// by the mobile app (see ai_docs/plans/2026-0707-mobile-app-flutter-api-plan.md).
//
// Design constraints:
//   - Resource-specific handlers live with their resource (resource/sermon,
//     resource/article, resource/event) following the existing pattern; only
//     shared plumbing and cross-resource aggregation (the feed) live here.
//   - This package must not be imported by resource packages' non-API code
//     paths beyond these helpers, and it must never import controllers, so the
//     dependency direction stays: controllers/router -> resources -> apiv1.
//   - Errors are returned as a stable JSON shape {"error": "..."} so the app
//     can surface one consistent failure UI.
package apiv1

import (
	"strconv"

	"github.com/rohanthewiz/rweb"
)

// ParseLimitOffset extracts standard pagination params.
// A hard cap keeps a single request from dragging the whole table across the
// wire — the app should page, not bulk-sync.
func ParseLimitOffset(ctx rweb.Context, defaultLimit, maxLimit int) (limit, offset int) {
	limit = defaultLimit
	if l, err := strconv.Atoi(ctx.Request().QueryParam("limit")); err == nil && l > 0 {
		limit = l
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if o, err := strconv.Atoi(ctx.Request().QueryParam("offset")); err == nil && o > 0 {
		offset = o
	}
	return limit, offset
}

// Error writes the API's uniform JSON error shape.
// technical details deliberately stay server-side (logs) — clients get a
// message safe to show end users.
func Error(ctx rweb.Context, status int, msg string) error {
	return ctx.Status(status).WriteJSON(map[string]string{"error": msg})
}
