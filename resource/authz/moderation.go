package authz

import (
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/logger"
)

// Chat and prayer-wall moderation (pin/delete a message, mark a request
// answered, remove anyone's request) is allowed by either of two grants:
//
//	legacy users.role editor-or-above ──┐
//	                                     ├─► may moderate
//	a role holding chat.moderate ───────┘
//
// Why both, rather than moving moderation onto the permission outright:
//
//   - Existing moderators keep working with no data change. Their base role
//     already says editor-or-above, and a site's roles may predate
//     chat.moderate (default roles are only seeded once).
//   - The mobile app decides whether to draw moderation buttons. Older
//     builds mirror the legacy rule from /auth/me's `role`, and newer ones
//     read the additive `can_moderate` flag. Keeping the legacy rule means
//     an old build never shows buttons the server would refuse.
//   - The permission adds what the base role can't express: a trusted member
//     who moderates chat without any admin access (chat.moderate is
//     SiteOnly).

// LegacyModerator reports whether a legacy users.role value moderates on its
// own. The scale is inverted (lower = more privileged: Admin 1, Publisher 5,
// Author/Editor 7, RegisteredUser 9) EXCEPT SuperAdmin at 99, so a plain <=
// comparison would wrongly exclude SuperAdmin; hence the explicit check.
// Zero (no role loaded) never moderates.
//
// church_mobile mirrors this rule in User.canModerate as its fallback for
// servers that don't send can_moderate; keep the two in step.
func LegacyModerator(role int) bool {
	if role == SuperAdminRole {
		return true
	}
	return role >= legacyAdmin && role <= legacyEditor
}

// CanModerate reports whether the enabled account username (whose legacy
// role is already known to the caller) may moderate.
//
// The legacy check runs first and costs nothing, so staff never pay the role
// queries. For everyone else the permission is resolved from the database on
// each call, for the same reason the admin guard does it: revoking a role
// ends moderation on the next request.
//
// A lookup error denies (fail closed) and is logged. On a Postgres site that
// hasn't run the roles migration the queries fail, which leaves exactly the
// legacy behaviour.
func CanModerate(exec db.Executor, username string, legacyRole int) bool {
	if LegacyModerator(legacyRole) {
		return true
	}
	if username == "" {
		return false
	}
	a, found, err := LoadActor(exec, username)
	if err != nil {
		logger.LogErr(err, "Could not resolve moderation permission", "username", username)
		return false
	}
	return found && a.Can(ChatModerate)
}
