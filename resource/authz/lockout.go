package authz

import (
	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
)

// Role-manager lockout guard.
//
// roles.update is how roles get repaired. If the last ordinary account
// holding it loses it, only a SuperAdmin can fix roles afterwards. SuperAdmin
// remains that recovery path, but on many sites it is a bootstrap login
// nobody uses day to day, so the guard stops the change instead.
//
// Every write that can take roles.update away from someone is checked
// against the state it would leave:
//
//	edit a role     (its permissions change)      ─┐
//	delete a role                                  │   holders after == 0
//	change a user's roles                          ├─► and holders now > 0
//	disable or delete a user                      ─┘   ─► refuse
//
// "Holders" are enabled accounts, other than SuperAdmins, whose roles grant
// roles.update. SuperAdmins are left out because they bypass permissions
// whatever their roles, so they don't count as a role-based manager.
//
// Only a transition to zero is refused. A site that has no holder today (only
// its SuperAdmin manages roles) can still be edited freely.
//
// The guard is the caller's to apply, and callers skip it for a SuperAdmin
// actor: that is the recovery path, and it can always restore a holder.
//
// Like the rest of this package, it is computed in Go from whole-table reads,
// not a JOIN. The tables are small, and single-table SELECTs are what both
// backends share.

// PendingChange describes a role or account write that is about to happen.
// Set the role fields, the user fields, or both.
type PendingChange struct {
	// RoleID is the role being edited (RolePerms holds its permissions after
	// the save) or deleted (DeleteRole).
	RoleID     int64
	RolePerms  Set
	DeleteRole bool

	// UserID is the account being saved (SetUserRoles with UserRoles, the role
	// ids it will hold) and/or disabled or deleted (RemoveUser).
	UserID       int64
	SetUserRoles bool
	UserRoles    []int64
	RemoveUser   bool
}

// LocksOutRoleManagers reports whether applying change would leave no role
// manager (see above) when there is at least one now.
func LocksOutRoleManagers(exec db.Executor, change PendingChange) (bool, error) {
	rolePerms, err := loadAllRolePerms(exec)
	if err != nil {
		return false, err
	}
	assignments, err := loadAllAssignments(exec)
	if err != nil {
		return false, err
	}
	eligible, err := loadRoleManagerCandidates(exec)
	if err != nil {
		return false, err
	}

	if countHolders(RolesUpdate, rolePerms, assignments, eligible) == 0 {
		return false, nil
	}

	// Apply the change to copies. The maps are only this call's, but copying
	// keeps "before" and "after" independent should the order above change.
	afterPerms := make(map[int64]Set, len(rolePerms))
	for id, s := range rolePerms {
		afterPerms[id] = s
	}
	if change.RoleID != 0 {
		if change.DeleteRole {
			delete(afterPerms, change.RoleID)
		} else {
			afterPerms[change.RoleID] = change.RolePerms
		}
	}

	afterAssign := make(map[int64][]int64, len(assignments))
	for uid, ids := range assignments {
		afterAssign[uid] = ids
	}
	afterEligible := make(map[int64]bool, len(eligible))
	for uid := range eligible {
		afterEligible[uid] = true
	}
	if change.UserID != 0 {
		if change.SetUserRoles {
			afterAssign[change.UserID] = change.UserRoles
		}
		if change.RemoveUser {
			delete(afterEligible, change.UserID)
		}
	}

	return countHolders(RolesUpdate, afterPerms, afterAssign, afterEligible) == 0, nil
}

// countHolders counts eligible users with at least one role granting perm.
// A role id missing from rolePerms (deleted) grants nothing.
func countHolders(perm Permission, rolePerms map[int64]Set, assignments map[int64][]int64, eligible map[int64]bool) int {
	n := 0
	for uid, roleIDs := range assignments {
		if !eligible[uid] {
			continue
		}
		for _, rid := range roleIDs {
			if rolePerms[rid].Has(perm) {
				n++
				break
			}
		}
	}
	return n
}

// loadAllRolePerms reads every grant, keyed by role id.
func loadAllRolePerms(exec db.Executor) (map[int64]Set, error) {
	rows, err := exec.Query(`SELECT role_id, permission FROM role_permissions`)
	if err != nil {
		return nil, serr.Wrap(err, "error loading role permissions")
	}
	defer rows.Close()
	out := map[int64]Set{}
	for rows.Next() {
		var rid int64
		var p string
		if err := rows.Scan(&rid, &p); err != nil {
			return nil, serr.Wrap(err, "error scanning role permission")
		}
		if out[rid] == nil {
			out[rid] = Set{}
		}
		out[rid][Permission(p)] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "error reading role permissions")
	}
	return out, nil
}

// loadAllAssignments reads every user→role assignment, keyed by user id.
func loadAllAssignments(exec db.Executor) (map[int64][]int64, error) {
	rows, err := exec.Query(`SELECT user_id, role_id FROM user_roles`)
	if err != nil {
		return nil, serr.Wrap(err, "error loading role assignments")
	}
	defer rows.Close()
	out := map[int64][]int64{}
	for rows.Next() {
		var uid, rid int64
		if err := rows.Scan(&uid, &rid); err != nil {
			return nil, serr.Wrap(err, "error scanning role assignment")
		}
		out[uid] = append(out[uid], rid)
	}
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "error reading role assignments")
	}
	return out, nil
}

// loadRoleManagerCandidates returns the ids of enabled, non-SuperAdmin users.
func loadRoleManagerCandidates(exec db.Executor) (map[int64]bool, error) {
	rows, err := exec.Query(`SELECT id, role FROM users WHERE enabled = $1`, true)
	if err != nil {
		return nil, serr.Wrap(err, "error loading enabled users")
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		var role int
		if err := rows.Scan(&id, &role); err != nil {
			return nil, serr.Wrap(err, "error scanning enabled user")
		}
		if role != SuperAdminRole {
			out[id] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, serr.Wrap(err, "error reading enabled users")
	}
	return out, nil
}

// RoleManagerLockoutMsg is the refusal shown when LocksOutRoleManagers is
// true. The caller appends what was not saved.
const RoleManagerLockoutMsg = "That would leave no account (other than a SuperAdmin) able to manage roles. " +
	"Give another person a role with the roles.update permission first."
