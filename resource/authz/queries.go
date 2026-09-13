package authz

import (
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/church/util/inputerr"
	"github.com/rohanthewiz/serr"
)

// Hand-written SQL (no SQLBoiler models): the roles tables postdate the
// generated models, same approach as event_locations and api_tokens.
//
// Portability across the two backends (Postgres, and bytdb over the wire)
// shapes these queries: only single-table SELECTs, $n placeholders,
// INSERT ... RETURNING and plain DELETEs. Joins, GROUP BY and ON CONFLICT DO
// NOTHING are avoided. The data is tiny (a handful of roles, one row per
// user per role, a few dozen permissions), so the few joins and counts needed
// happen in Go.

// Role is a named permission combination.
type Role struct {
	ID          int64
	Name        string
	Description string
	Perms       Set
	UserCount   int // filled by ListRoles only
}

// LoadActor resolves an enabled user's effective permissions.
// found=false (no error) when the username doesn't exist or the account is
// disabled. The admin guard treats that as not signed in, which is how
// disabling a user ends their web admin session immediately.
func LoadActor(exec db.Executor, username string) (a *Actor, found bool, err error) {
	a = &Actor{Username: username, perms: Set{}}
	row := exec.QueryRow(`SELECT id, role FROM users WHERE username = $1 AND enabled = $2`, username, true)
	if err = row.Scan(&a.UserID, &a.BaseRole); err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, serr.Wrap(err, "error loading admin user", "username", username)
	}
	// SuperAdmin bypasses permissions, so its roles are never consulted.
	// Returning before the role queries also means a Postgres site that
	// hasn't run the roles migration yet still admits its SuperAdmin (the
	// user_roles read would fail), which is the way back in to fix it.
	if a.BaseRole == SuperAdminRole {
		return a, true, nil
	}

	roleIDs, err := RoleIDsForUser(exec, a.UserID)
	if err != nil {
		return nil, false, err
	}
	byRole, err := permsForRoles(exec, roleIDs)
	if err != nil {
		return nil, false, err
	}
	for _, perms := range byRole {
		for p := range perms {
			// Stored grants are re-validated: a permission retired from the
			// catalog in a later release must stop granting anything even if
			// its rows linger.
			if Valid(p) {
				a.perms[p] = struct{}{}
			}
		}
	}
	return a, true, nil
}

// RoleIDsForUser returns the ids of the roles assigned to a user.
func RoleIDsForUser(exec db.Executor, userID int64) ([]int64, error) {
	rows, err := exec.Query(`SELECT role_id FROM user_roles WHERE user_id = $1`, userID)
	if err != nil {
		return nil, serr.Wrap(err, "error loading user roles", "user_id", strconv.FormatInt(userID, 10))
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, serr.Wrap(err, "error scanning user role")
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "error reading rows")
	}
	return ids, nil
}

// permsForRoles loads the permission sets of the given roles, keyed by role
// id, in one query. The id list is interpolated rather than parameterised,
// safe for the reason given at event.EventPoints: every element is an int64
// formatted by strconv. An empty list touches no database.
func permsForRoles(exec db.Executor, roleIDs []int64) (map[int64]Set, error) {
	out := make(map[int64]Set, len(roleIDs))
	if len(roleIDs) == 0 {
		return out, nil
	}
	ids := make([]string, 0, len(roleIDs))
	for _, id := range roleIDs {
		ids = append(ids, strconv.FormatInt(id, 10))
		out[id] = Set{}
	}
	rows, err := exec.Query(`SELECT role_id, permission FROM role_permissions WHERE role_id IN (` +
		strings.Join(ids, ",") + `)`)
	if err != nil {
		return nil, serr.Wrap(err, "error loading role permissions")
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var p string
		if err := rows.Scan(&id, &p); err != nil {
			return nil, serr.Wrap(err, "error scanning role permission")
		}
		if out[id] == nil {
			out[id] = Set{}
		}
		out[id][Permission(p)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "error reading rows")
	}
	return out, nil
}

// ListRoles returns every role with its permissions and assigned-user count,
// ordered by name.
func ListRoles(exec db.Executor) ([]Role, error) {
	rows, err := exec.Query(`SELECT id, name, description FROM roles`)
	if err != nil {
		return nil, serr.Wrap(err, "error listing roles")
	}
	var roles []Role
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.ID, &r.Name, &r.Description); err != nil {
			rows.Close()
			return nil, serr.Wrap(err, "error scanning role")
		}
		roles = append(roles, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "error reading roles")
	}
	if len(roles) == 0 {
		return roles, nil
	}

	ids := make([]int64, 0, len(roles))
	for _, r := range roles {
		ids = append(ids, r.ID)
	}
	byRole, err := permsForRoles(exec, ids)
	if err != nil {
		return nil, err
	}
	counts, err := userCountsByRole(exec)
	if err != nil {
		return nil, err
	}
	for i := range roles {
		roles[i].Perms = byRole[roles[i].ID]
		roles[i].UserCount = counts[roles[i].ID]
	}
	sort.Slice(roles, func(i, j int) bool {
		return strings.ToLower(roles[i].Name) < strings.ToLower(roles[j].Name)
	})
	return roles, nil
}

// userCountsByRole counts assignments per role.
func userCountsByRole(exec db.Executor) (map[int64]int, error) {
	rows, err := exec.Query(`SELECT role_id FROM user_roles`)
	if err != nil {
		return nil, serr.Wrap(err, "error counting role assignments")
	}
	defer rows.Close()
	counts := map[int64]int{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, serr.Wrap(err, "error scanning role assignment")
		}
		counts[id]++
	}
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "error reading rows")
	}
	return counts, nil
}

// GetRole loads one role with its permissions. found=false (no error) when
// there is no such role.
func GetRole(exec db.Executor, id int64) (r Role, found bool, err error) {
	row := exec.QueryRow(`SELECT id, name, description FROM roles WHERE id = $1`, id)
	if err = row.Scan(&r.ID, &r.Name, &r.Description); err != nil {
		if err == sql.ErrNoRows {
			return r, false, nil
		}
		return r, false, serr.Wrap(err, "error loading role", "id", strconv.FormatInt(id, 10))
	}
	byRole, err := permsForRoles(exec, []int64{id})
	if err != nil {
		return r, false, err
	}
	r.Perms = byRole[id]
	return r, true, nil
}

// SaveRole inserts (ID == 0) or updates a role and replaces its permissions
// with Normalize(r.Perms). It returns the role's id.
//
// Authorization (CanGrant) is the caller's job. This function only enforces
// data rules: a non-empty name, unique regardless of case.
//
// Not transactional: the Executor seam has no Begin, and bytdb's wire
// transaction support isn't something this path should be the first to
// depend on. Instead the permission replace deletes revoked grants before
// inserting new ones, so a failure part-way leaves the role holding fewer
// permissions than intended, never more. Saving the form again completes it.
func SaveRole(exec db.Executor, r Role, updatedBy string) (int64, error) {
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	if r.Name == "" {
		return 0, serr.Wrap(inputerr.New("A role name is required", nil))
	}

	// Case-insensitive uniqueness, checked in Go: lower() in a unique index
	// isn't portable to bytdb, and the table is a handful of rows. The unique
	// index on name still backstops an exact-duplicate race.
	existing, err := exec.Query(`SELECT id, name FROM roles`)
	if err != nil {
		return 0, serr.Wrap(err, "error checking role names")
	}
	for existing.Next() {
		var id int64
		var name string
		if err := existing.Scan(&id, &name); err != nil {
			existing.Close()
			return 0, serr.Wrap(err, "error scanning role name")
		}
		if id != r.ID && strings.EqualFold(name, r.Name) {
			existing.Close()
			return 0, serr.Wrap(inputerr.New("A role named \""+name+"\" already exists", nil))
		}
	}
	existing.Close()

	now := time.Now()
	if r.ID == 0 {
		err = exec.QueryRow(`INSERT INTO roles (name, description, created_at, updated_at, updated_by)
			VALUES ($1, $2, $3, $3, $4) RETURNING id`, r.Name, r.Description, now, updatedBy).Scan(&r.ID)
		if err != nil {
			return 0, serr.Wrap(err, "error inserting role", "name", r.Name)
		}
	} else {
		res, err := exec.Exec(`UPDATE roles SET name = $1, description = $2, updated_at = $3, updated_by = $4
			WHERE id = $5`, r.Name, r.Description, now, updatedBy, r.ID)
		if err != nil {
			return 0, serr.Wrap(err, "error updating role", "id", strconv.FormatInt(r.ID, 10))
		}
		if n, err := res.RowsAffected(); err == nil && n == 0 {
			return 0, serr.Wrap(inputerr.New("That role no longer exists", nil))
		}
	}

	if err := replaceRolePerms(exec, r.ID, Normalize(r.Perms)); err != nil {
		return r.ID, err
	}
	return r.ID, nil
}

// replaceRolePerms makes the stored grants for a role equal want: revoked
// grants are deleted first, then new ones inserted (see SaveRole for why
// that order).
func replaceRolePerms(exec db.Executor, roleID int64, want Set) error {
	byRole, err := permsForRoles(exec, []int64{roleID})
	if err != nil {
		return err
	}
	have := byRole[roleID]
	idStr := strconv.FormatInt(roleID, 10)

	for p := range have {
		if !want.Has(p) {
			if _, err := exec.Exec(`DELETE FROM role_permissions WHERE role_id = $1 AND permission = $2`,
				roleID, string(p)); err != nil {
				return serr.Wrap(err, "error revoking role permission", "role_id", idStr, "permission", string(p))
			}
		}
	}
	for _, p := range want.Sorted() {
		if !have.Has(p) {
			if _, err := exec.Exec(`INSERT INTO role_permissions (role_id, permission) VALUES ($1, $2)`,
				roleID, string(p)); err != nil {
				return serr.Wrap(err, "error granting role permission", "role_id", idStr, "permission", string(p))
			}
		}
	}
	return nil
}

// DeleteRole removes a role, its grants and its assignments. Child rows are
// deleted explicitly rather than left to ON DELETE CASCADE so the outcome
// doesn't hinge on either backend's cascade behavior; children go first so a
// failure never leaves grants pointing at a missing role.
func DeleteRole(exec db.Executor, id int64) error {
	idStr := strconv.FormatInt(id, 10)
	for _, stmt := range []string{
		`DELETE FROM user_roles WHERE role_id = $1`,
		`DELETE FROM role_permissions WHERE role_id = $1`,
		`DELETE FROM roles WHERE id = $1`,
	} {
		if _, err := exec.Exec(stmt, id); err != nil {
			return serr.Wrap(err, "error deleting role", "id", idStr)
		}
	}
	return nil
}

// SetUserRoles makes a user's assignments equal roleIDs, removing first and
// then adding, for the same fewer-never-more reason as replaceRolePerms.
func SetUserRoles(exec db.Executor, userID int64, roleIDs []int64) error {
	have, err := RoleIDsForUser(exec, userID)
	if err != nil {
		return err
	}
	haveSet := map[int64]bool{}
	for _, id := range have {
		haveSet[id] = true
	}
	wantSet := map[int64]bool{}
	for _, id := range roleIDs {
		wantSet[id] = true
	}
	uid := strconv.FormatInt(userID, 10)

	for id := range haveSet {
		if !wantSet[id] {
			if _, err := exec.Exec(`DELETE FROM user_roles WHERE user_id = $1 AND role_id = $2`, userID, id); err != nil {
				return serr.Wrap(err, "error removing user role", "user_id", uid, "role_id", strconv.FormatInt(id, 10))
			}
		}
	}
	now := time.Now()
	for id := range wantSet {
		if !haveSet[id] {
			if _, err := exec.Exec(`INSERT INTO user_roles (user_id, role_id, created_at) VALUES ($1, $2, $3)`,
				userID, id, now); err != nil {
				return serr.Wrap(err, "error adding user role", "user_id", uid, "role_id", strconv.FormatInt(id, 10))
			}
		}
	}
	return nil
}

// RoleNamesByUser maps each user id to the sorted names of their roles, for
// the users list. Two whole-table reads; see the file comment on size.
func RoleNamesByUser(exec db.Executor) (map[int64][]string, error) {
	roles, err := ListRoles(exec)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(roles))
	for _, r := range roles {
		names[r.ID] = r.Name
	}

	rows, err := exec.Query(`SELECT user_id, role_id FROM user_roles`)
	if err != nil {
		return nil, serr.Wrap(err, "error loading role assignments")
	}
	defer rows.Close()
	out := map[int64][]string{}
	for rows.Next() {
		var uid, rid int64
		if err := rows.Scan(&uid, &rid); err != nil {
			return nil, serr.Wrap(err, "error scanning role assignment")
		}
		if n, ok := names[rid]; ok {
			out[uid] = append(out[uid], n)
		}
	}
	for uid := range out {
		sort.Strings(out[uid])
	}
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "error reading rows")
	}
	return out, nil
}

// ---- Publish / enable flags ----

// FlagTarget names a boolean "is live" column that a publish or enable
// permission governs. It is a closed set of values, never built from input,
// because the table and column are interpolated into SQL.
type FlagTarget struct {
	table, column string
}

var (
	FlagPagePublished    = FlagTarget{"pages", "published"}
	FlagMenuPublished    = FlagTarget{"menu_defs", "published"}
	FlagArticlePublished = FlagTarget{"articles", "published"}
	FlagSermonPublished  = FlagTarget{"sermons", "published"}
	FlagEventPublished   = FlagTarget{"events", "published"}
	FlagUserEnabled      = FlagTarget{"users", "enabled"}
)

// ResolveFlag decides the value to save for a publish/enable flag.
//
//   - The actor holds the permission: the submitted value stands.
//   - They don't, on create (id empty or "0"): false. A new item waits for
//     someone who may publish it.
//   - They don't, on update: the stored value is kept, so an editor saving a
//     live article neither unpublishes it nor can publish a draft.
//
// Keeping the stored value (rather than refusing the whole save) matters
// because the form still posts the flag. A disabled checkbox posts nothing,
// which would otherwise read as "unpublish".
func ResolveFlag(exec db.Executor, a *Actor, perm Permission, target FlagTarget, id string, submitted bool) (bool, error) {
	if a.Can(perm) {
		return submitted, nil
	}
	id = strings.TrimSpace(id)
	if id == "" || id == "0" {
		return false, nil
	}
	intID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return false, serr.Wrap(err, "invalid id for flag lookup", "id", id)
	}
	var current bool
	err = exec.QueryRow(`SELECT `+target.column+` FROM `+target.table+` WHERE id = $1`, intID).Scan(&current)
	if err == sql.ErrNoRows {
		// The row is gone; the save that follows will fail or insert on its
		// own terms. Unpublished is the safe value to hand it.
		return false, nil
	}
	if err != nil {
		return false, serr.Wrap(err, "error loading current flag", "table", target.table, "id", id)
	}
	return current, nil
}

// ---- Managing users ----

// RoleFieldPrefix names each role checkbox on the user form: "role:12". One
// field per role for the reason given at PermFieldPrefix.
const RoleFieldPrefix = "role:"

// CanManageUser extends the no-escalation rule from permissions to people:
// an actor may edit or delete a user only if everything that user can do is
// something the actor can do too.
//
// Without it, users.update alone would be a takeover path: reset an
// Administrator's password, sign in as them, and hold everything. For the
// same reason a SuperAdmin account (which bypasses permissions entirely) is
// manageable only by another SuperAdmin.
//
// A missing user is reported as manageable: there is nothing to protect, and
// the save or delete that follows reports the absence on its own terms.
func CanManageUser(exec db.Executor, a *Actor, userID int64) (bool, error) {
	if a == nil {
		return false, nil
	}
	if a.IsSuper() {
		return true, nil
	}
	var baseRole int
	err := exec.QueryRow(`SELECT role FROM users WHERE id = $1`, userID).Scan(&baseRole)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, serr.Wrap(err, "error loading user for authorization", "user_id", strconv.FormatInt(userID, 10))
	}
	if baseRole == SuperAdminRole {
		return false, nil
	}
	roleIDs, err := RoleIDsForUser(exec, userID)
	if err != nil {
		return false, err
	}
	byRole, err := permsForRoles(exec, roleIDs)
	if err != nil {
		return false, err
	}
	target := Set{}
	for _, perms := range byRole {
		for p := range perms {
			target[p] = struct{}{}
		}
	}
	return a.CanGrant(target), nil
}

// PermsByUser maps each user id holding any role to their effective
// permissions, for the users list (which locks rows the viewer can't manage).
// Two whole-table reads; see the file comment on size.
func PermsByUser(exec db.Executor) (map[int64]Set, error) {
	rows, err := exec.Query(`SELECT role_id, permission FROM role_permissions`)
	if err != nil {
		return nil, serr.Wrap(err, "error loading role permissions")
	}
	byRole := map[int64]Set{}
	for rows.Next() {
		var rid int64
		var p string
		if err := rows.Scan(&rid, &p); err != nil {
			rows.Close()
			return nil, serr.Wrap(err, "error scanning role permission")
		}
		if byRole[rid] == nil {
			byRole[rid] = Set{}
		}
		byRole[rid][Permission(p)] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "error reading role permissions")
	}

	rows, err = exec.Query(`SELECT user_id, role_id FROM user_roles`)
	if err != nil {
		return nil, serr.Wrap(err, "error loading role assignments")
	}
	defer rows.Close()
	out := map[int64]Set{}
	for rows.Next() {
		var uid, rid int64
		if err := rows.Scan(&uid, &rid); err != nil {
			return nil, serr.Wrap(err, "error scanning role assignment")
		}
		if out[uid] == nil {
			out[uid] = Set{}
		}
		for p := range byRole[rid] {
			out[uid][p] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "error reading role assignments")
	}
	return out, nil
}
