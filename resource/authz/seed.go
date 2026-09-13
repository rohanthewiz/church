package authz

import (
	"strconv"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/serr"
)

// Legacy users.role values, mirrored from user.Roles for the backfill (see
// SuperAdminRole for why they aren't imported).
const (
	legacyAdmin     = 1
	legacyPublisher = 5
	legacyEditor    = 7
)

// defaultRole is a role created on a site's first boot with roles support.
type defaultRole struct {
	name, desc string
	perms      Set
	legacy     int // users with this legacy role are assigned it once
}

func defaultRoles() []defaultRole {
	// Content = everything a publisher manages day to day.
	content := func(withPublishDelete bool) Set {
		s := Set{}
		for _, res := range []string{"pages", "articles", "sermons", "events"} {
			s[Permission(res+"."+ActCreate)] = struct{}{}
			s[Permission(res+"."+ActRead)] = struct{}{}
			s[Permission(res+"."+ActUpdate)] = struct{}{}
			if withPublishDelete {
				s[Permission(res+"."+ActDelete)] = struct{}{}
				s[Permission(res+"."+ActPublish)] = struct{}{}
			}
		}
		return s
	}
	publisher := content(true)
	for _, p := range []Permission{MenusCreate, MenusRead, MenusUpdate, MenusDelete, MenusEnable} {
		publisher[p] = struct{}{}
	}
	// Legacy Publisher (5) and Editor (7) already moderate chat by base role
	// (LegacyModerator). The matching default roles carry the permission too,
	// so a site that later moves someone's base role to member keeps the
	// behaviour their role describes.
	editor := content(false)
	publisher[ChatModerate] = struct{}{}
	editor[ChatModerate] = struct{}{}

	return []defaultRole{
		{"Administrator", "Everything, including users, roles and giving records.", AllPermissions(), legacyAdmin},
		{"Publisher", "Create, publish and delete site content and menus.", publisher, legacyPublisher},
		{"Editor", "Write and edit content. Publishing and deleting are left to a publisher.", editor, legacyEditor},
	}
}

// EnsureDefaultRoles runs at bootstrap. The first time the roles table is
// found empty it:
//
//  1. creates Administrator, Publisher and Editor roles, and
//  2. assigns them to existing users by their legacy role (1, 5, 7), so staff
//     keep working admin access across the upgrade.
//
// "Roles table empty" is the one-shot marker, so this never re-runs on a site
// where roles exist, and never re-adds a role an admin deliberately deleted.
// The one exception is deleting every role, which brings the defaults back on
// the next boot; that is deliberate as a recovery path.
//
// Users at legacy 9 (RegisteredUser) get no role. Before roles existed, any
// signed-in user could reach /admin, which was never the intent of level 9,
// so their loss of admin access here is the fix, not a regression.
func EnsureDefaultRoles(exec db.Executor) error {
	var count int64
	if err := exec.QueryRow(`SELECT count(*) FROM roles`).Scan(&count); err != nil {
		return serr.Wrap(err, "error counting roles (has the roles migration run?)")
	}
	if count > 0 {
		return nil
	}

	roleIDByLegacy := map[int]int64{}
	for _, d := range defaultRoles() {
		id, err := SaveRole(exec, Role{Name: d.name, Description: d.desc, Perms: d.perms}, "bootstrap")
		if err != nil {
			return serr.Wrap(err, "error creating default role", "name", d.name)
		}
		roleIDByLegacy[d.legacy] = id
	}

	rows, err := exec.Query(`SELECT id, role FROM users`)
	if err != nil {
		return serr.Wrap(err, "error loading users for role backfill")
	}
	type assignment struct{ userID, roleID int64 }
	var todo []assignment
	for rows.Next() {
		var uid int64
		var legacy int
		if err := rows.Scan(&uid, &legacy); err != nil {
			rows.Close()
			return serr.Wrap(err, "error scanning user for role backfill")
		}
		if rid, ok := roleIDByLegacy[legacy]; ok {
			todo = append(todo, assignment{uid, rid})
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return serr.Wrap(err, "error reading users for role backfill")
	}

	// Writes happen after the cursor is closed: some drivers can't run a
	// statement on the connection while a result set is still open on it.
	for _, as := range todo {
		if err := SetUserRoles(exec, as.userID, []int64{as.roleID}); err != nil {
			return serr.Wrap(err, "error assigning backfilled role", "user_id", strconv.FormatInt(as.userID, 10))
		}
	}
	logger.Info("Created default roles", "roles", strconv.Itoa(len(roleIDByLegacy)),
		"users_assigned", strconv.Itoa(len(todo)))
	return nil
}
