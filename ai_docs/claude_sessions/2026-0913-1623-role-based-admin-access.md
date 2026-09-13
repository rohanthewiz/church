# Session: Role-based admin access (Role Management)

**Session:** https://claude.ai/code/session_01NQNwAU3z6vZ7GgJDWAPfjE
**Date:** 2026-09-13
**Continues:** `2026-0913-1542-postgres-default-db.md`

## What happened

The user asked for role management:

- Create a role from any combination of permissions.
- Assign multiple roles to a person.
- Permissions by resource: menus (CRUD, enable), pages, articles, sermons and
  events (CRUD, publish), users (CRUD, enable), charges/giving (read only).
- UI: a new Role Management page; role assignment on the existing Users page.

It is built, tested on bytdb and on real Postgres, and the cema and ccswm site
binaries build.

## Security bug found and fixed

Before this session, **any signed-in user could use the whole admin area**,
including a RegisteredUser (9). There were two causes:

1. `AdminGuardRWeb` only checked that the session had a username.
2. rweb group middleware **falls through to the route handler** when it
   returns nil without calling `Next()` (rweb `Group.go`), and a redirect
   returns nil. So a "denied" request still ran the admin handler behind the
   303: its body and side effects happened anyway.

Fix: the group guard now only resolves the actor (or records a denial). Every
admin route is wrapped in a per-route decorator, `authctlr.Require(perm, h)` or
`RequireAdmin(h)`, which is what keeps a handler from running. This is the same
decorator pattern `apitoken.APIGuard` already used for the same reason. Tests
assert the handler body is absent on denial.

## Design

```
users ──< user_roles >── roles ──< role_permissions ("articles.publish")
```

- **Package `resource/authz`:**
  - `permissions.go`: the fixed permission catalog, named constants, `Set`,
    and `Normalize`, which drops unknowns and adds `<res>.read` when any other
    action is held.
  - `actor.go`: `Actor` (`Can`, `HasAdminAccess`, `CanGrant`), request-context
    helpers, and render-param encoding (`ParamKey`/`ParamValue`/`FromParams`).
    Modules render from params, not the context, so permissions ride
    `params["_global"]["perms"]`.
  - `queries.go`: hand-written SQL portable to both backends. Single-table
    SELECTs only (no JOIN, GROUP BY or ON CONFLICT DO NOTHING); joins are done
    in Go. Functions: `LoadActor`, `ListRoles`, `GetRole`, `SaveRole`,
    `DeleteRole`, `SetUserRoles`, `RoleNamesByUser`, `PermsByUser`,
    `CanManageUser`, `ResolveFlag`.
  - `seed.go`: `EnsureDefaultRoles`.
  - `module_roles_list.go`, `module_role_form.go`: the UI.
- **Catalog is code, roles are data.** Permissions only mean something where a
  handler checks them.
- **`roles.*` permissions were added** beyond the brief. Without them, anyone
  who could open Role Management could write themselves a role holding
  everything.
- **No-escalation rule:**
  - You can only grant or revoke permissions you hold (`CanGrant`). This covers
    both defining a role's permissions and assigning roles.
  - You can only edit or delete a user whose effective permissions are a subset
    of yours (`CanManageUser`). Otherwise `users.update` would allow resetting
    an Administrator's password and signing in as them.
  - Only a SuperAdmin can grant SuperAdmin or manage a SuperAdmin account.
- **SuperAdmin (legacy `users.role = 99`) bypasses permission checks.**
  `LoadActor` returns before the role queries for a SuperAdmin, so a Postgres
  site that hasn't run the migration still admits its SuperAdmin.
- **Legacy `users.role` kept.** It still drives chat and prayer-wall moderation
  and the mobile `/auth/me` `role`/`role_name`, so there is no mobile contract
  change. The Users form labels it "Base Role".
- **Permissions are loaded from the DB on every admin request.** Revoking a role
  or disabling a user takes effect on the next click.
- **Default roles:** on the first boot with an empty `roles` table,
  `admin.bootstrapRoles` creates Administrator (everything), Publisher (content
  and menus, including publish/delete/enable) and Editor (content
  create/read/update). Users at legacy role 1/5/7 are assigned the matching
  role. Legacy 9 gets none, which is the intended loss of admin access.
- **Publish/enable is field-level** (`authz.ResolveFlag`):
  - With the permission, the submitted value stands.
  - Without it, a create is saved unpublished and an update keeps the stored
    value.
  - The form disables the switch and posts the stored value in a hidden field.
    A disabled checkbox posts nothing, which would read as "unpublish".
- **Form fields are one per item** (`perm:articles.publish`, `role:12`) because
  rweb's `FormValue` only returns a field's first value.
- **Writes aren't transactional** (the Executor seam has no `Begin`). Replaces
  delete before they insert, so a partial failure leaves fewer grants, never
  more.
- **Route → permission convention:** list = read, new + create POST = create,
  edit + update POST = update, delete POST = delete. Page preview = read. Sermon
  import = create; sermon cleanup = update (it removes cached copies, not
  sermons). Dashboard, logout and `/debug/*` need any admin access.

## Changes

- **Schema**
  - New `db/migrate/20260913160000_CreateRolesTables.sql`: `roles`,
    `role_permissions` and `user_roles`, each with a surrogate id plus a unique
    natural-key index.
  - `db/bytdb_schema.go`: the same three tables, with FK indexes.
- **Routing:** `router_rweb.go`
  - Admin routes moved into the exported `RegisterAdminRoutes(s)`, so checks
    drive the production wiring.
  - Every admin and debug route is wrapped.
  - New routes: `/admin/roles` (list/new/create/edit/update/delete) and
    `/admin/giving`.
- **Auth:** `auth_controller/auth_middleware_rweb.go` has the rewritten
  `AdminGuardRWeb` plus `Require`/`RequireAdmin`.
- **Rendering:** `basectlr/base_controller_rweb.go` passes perms in `_global`.
  So do the direct `template.Page` calls in `admin_controller`,
  `article_controller`, `sermon_controller`, `page_controller` and
  `menu_controller`.
- **Role Management:** new `role_controller/` and `page/role_pages.go`, with
  registry entries. `availableModuleTypes` excludes `role` and `giving`.
- **Users**
  - `resource/user/module_user_form.go`:
    - Roles card with a checkbox per role; roles you can't grant are locked.
    - Base Role select hides SuperAdmin from non-supers.
    - Enabled switch requires `users.enable`.
    - Form locks when you can't manage the user.
  - `resource/user/module_users_list.go`: Roles column, "locked" rows, and
    create/delete actions shown only with the permission.
  - `resource/user/user_queries.go`: `UpsertUserID` returns the saved id.
    `UpsertUser` wraps it.
  - `user_controller/user_controller_rweb.go`: `CanManageUser` check,
    SuperAdmin grant refusal, `ResolveFlag` for enabled, and role assignment.
    Roles you can't grant keep their current state. Delete is guarded too.
- **Publish/enable enforcement:**
  - Article, sermon, event, page and menu upsert handlers.
  - Sermon resolves the flag before the audio copy, so a refusal costs no file.
  - Forms: `module_article_form`, `module_sermon_form`, `module_event_form`,
    `module_menu_form`, `page/module_page_form`.
- **Giving:** new `resource/payment/module_giving_list.go`,
  `payment_controller/admin_giving_rweb.go` and `page.GivingList()`. A
  read-only grid with date, name, email, amount, status, description, comment
  and receipt link.
- **Dashboard:** `page/admin_home.go` filters cards by permission and adds Roles
  and Giving cards.
- **Menus:** the admin submenu fallback (`resource/menu/menu_def.go`) and the
  bootstrap seed (`admin/bootstrap.go`) gain Roles and Giving links.
- **Bootstrap:** `admin/bootstrap.go` has the new `bootstrapRoles()`, which logs
  and does not fail.
- **Tests and checks**
  - `resource/authz/permissions_test.go`: catalog, `Normalize`, actor checks,
    params round trip.
  - `resource/authz/queries_bytdb_test.go`: real queries on embedded bytdb over
    the wire.
  - `resource/authz/cfg/random_seeds.txt`: a copy of the existing fixture.
    `resource/auth`'s `init` needs it once the package imports app.
  - `auth_controller/auth_flow_test.go`:
    - Decorator wiring and body-leak assertions.
    - New tests: member without roles, disabled user, permission denied,
      SuperAdmin bypass.
  - `test_scripts/roles_smoke`: 20 end-to-end checks through
    `RegisterAdminRoutes` on bytdb.
  - `test_scripts/roles_pg_check`: applies the migration's Up section and runs
    the authz queries in one transaction on the local Postgres, **always rolled
    back**.

## Verification

- `go build ./...`: OK.
- `go vet` on all touched packages: OK.
- `go test ./...`: all packages ok.
- `go run ./test_scripts/roles_smoke`: all 20 checks pass. They cover:
  - roles list and form render
  - creating a role from a permission combination (an unknown permission is
    ignored)
  - two roles on one user combine
  - route denial
  - the dashboard card filter
  - Administrator reads giving records
  - a users-only manager can't take over an Administrator, can't assign
    themselves Administrator, and can't grant SuperAdmin
  - an editor's new article is saved unpublished and the switch is disabled
  - removing all roles ends admin access immediately
- `go run ./test_scripts/roles_pg_check` (local Postgres, `church_development`):
  all checks pass, and the transaction rolled back, so the database is
  unchanged.
- cema and ccswm binaries build.
- **Not done:** clicking through the new screens in a real browser on a running
  site.

## Gotchas found

- **rweb group middleware fall-through** (above). Any future admin route
  **must** be wrapped in `Require`/`RequireAdmin`, or it is reachable by anyone.
- **`serr.Wrap(nil)` prints a "Not wrapping a nil error" warning.** Check
  `rows.Err()` explicitly instead.
- **A harness that renders pages must set `config.Options`** (for example to
  `&config.EnvConfig{}`). `template.Page` dereferences
  `config.Options.Theme`.
- **`gofmt -l` exits 0 even when it lists files.** Test for empty output.
- **zsh:** `echo ===` is `=`-expansion and aborts the command chain.
- **Pre-existing gofmt drift, left alone:** `router_rweb.go`,
  `auth_controller/auth_middleware_rweb.go`,
  `user_controller/user_controller_rweb.go`,
  `resource/user/user_presenter.go`, `event_controller/event_controller_rweb.go`
  and `admin_controller/admin_controller_rweb.go`. These are whitespace or
  import-order issues that were already in HEAD.

## Deploy impact

- **Postgres sites:** run `goose up` for `20260913160000_CreateRolesTables.sql`.
  Until then only the SuperAdmin can use the admin area, and bootstrap logs
  "could not ensure default roles".
- **bytdb sites (k8s):** the tables are created automatically at boot.
- **Legacy role-9 accounts lose admin access.** Staff at 1/5/7 are backfilled.
- **Admin menus already stored in a site's DB** are unchanged. They still list
  every admin link. The guards redirect unpermitted clicks to the dashboard
  with a warning.

## Next

1. **New:** run `goose up` for the roles migration on `church_development` and
   on any Postgres site. Also still pending from before: the `event_locations`
   migration (old item 18).
2. **New:** browser click-through on a running site:
   - role form (matrix toggles, auto-Read)
   - user form Roles card and locked states
   - giving list paging
   - flash messages on refusals
   - publish switch disabled for an Editor
3. **New:** filter the nav's Admin submenu by permission. Map known `/admin/…`
   URLs to their read permission in `menu.buildMenu`, which only gets
   `loggedIn` today. DB-stored admin menus currently show dead links to
   unpermitted users.
4. **New:** add "+" / delete visibility by permission to the other admin list
   modules (articles, sermons, events, pages, menus). Handlers already refuse;
   this is UI polish. Users and roles lists are done.
5. **New, optional:** a guard against removing the last role-holder with
   `roles.update` (non-SuperAdmin lockout). SuperAdmin remains the recovery
   path, so this is low priority.
6. **New, optional:** consider an explicit permission (or SuperAdmin-only) for
   `/debug/*`. Today any admin can toggle process-wide debug state.
7. **New:** decide whether chat/prayer-wall moderation should move from legacy
   `users.role` to a permission, for example `chat.moderate`. It was left on the
   legacy role deliberately, to avoid touching the mobile contract.
8. **New:** run `test_scripts/roles_smoke` in CI (or convert it to a Go test
   with a cfg fixture) so route wiring can't silently lose a `Require`.
9. cema's `pg:` fix (`12049cf`) is only on `feature/site-themes`. Merge it to
   cema's main branch before building cema from there.
10. Boot cema locally with no `db.type` against Postgres. Confirm there is no
    `bytdb serving` line and that pages render (recipe in `2026-0912-1718-…`,
    minus `DB_TYPE=postgres`).
11. Document the `db:` block (`type: postgres|bytdb`, `file`, `listen`) in
    `cema/cfg/options-sample.yml` and `ccswm/cfg/options-sample.yml`.
12. Update docs that still describe bytdb as the default:
    - `deploy/k8s/README.md` §migration step 6
    - `ai_docs/fable_bytdb_k8s_readiness.md`
13. Start Docker and run `./deploy/deploy.sh images`, the one unproven fix from
    2026-08-01. If bimg/libvips 8.15 causes trouble, pin an older Alpine or use
    a pure-Go resizer.
14. Provision LKE + Object Storage and fill `deploy/backup.env`. Then run
    `./deploy/deploy.sh preflight infra`, point DNS, and run
    `./deploy/deploy.sh base seeds secrets images sites verify`.
15. Create `ccswm/cfg/options.yml` from the sample. It needs a real `pg:` block,
    or `db.type: bytdb`.
16. Move `resource/chimage` uploads onto IDrive e2 beside sermon media, retiring
    the `dist/img` volume mount.
17. From the readiness doc:
    - Call `CloseDB()` on SIGTERM in `ServeRWeb`.
    - Migrate `resource/dbbackup` off aws-sdk-go-v2 onto `replicate/s3`.
18. **Optional hardening:** `imagePullSecrets` (if the ghcr packages go private)
    and a www→apex redirect.
19. Live-check the 2026-09-12 flash changes on a running site (recipe in
    `2026-0912-1718-…`):
    - bad event coordinates or an expired csrf
    - empty menu `items` / page `modules`
    - referrer guard after a refused save
    - `token.txt` permissions on a fresh DB
20. Live-check the admin save flashes from `2026-0912-2125-…`:
    - expired csrf
    - blank article title
    - bad sermon date with audio chosen
    - password mismatch
    - stopped DB
21. Duplicate title on a new article/sermon (unique slug) still shows a generic
    "Error saving". Pre-check the slug on create as an InputError, working on
    both executors.
22. Seeds hygiene:
    - Replace the `resource/*/cfg/random_seeds.txt` fixtures with dummy seeds.
      This now includes `resource/authz/cfg`.
    - Consider rotating the live `cfg/random_seeds.txt`.
23. **Optional:** sermon upload file handling (partial file on copy failure,
    orphan on DB failure, truncate-before-save). Fix with a temp file and rename
    after a successful upsert.
24. Keep typed values on a refused save: re-render the form from the posted
    presenter (events, articles, sermons, users) and/or wrap `UpsertEvent`'s
    writes in a transaction.
25. **Postgres coverage still missing:**
    - sermon create/import + audio
    - Stripe intent/history/webhook
    - `/chat/stream` SSE
    - image upload
    - Optionally let `bytdb_wire_check` take a Postgres DSN.

    (User create/delete and roles are now exercised: `roles_pg_check` for the
    authz SQL, `roles_smoke` on bytdb for the handlers.)
26. Add `bytdb_to_pg` (and the `pg_to_bytdb` date fix), plus the Postgres smoke
    result, to `ai_docs/fable_bytdb_k8s_readiness.md` §7.
27. **Optional:** per-table content checksum in `bytdb_to_pg` beyond row counts.
28. **Non-goal (declined):** a `-truncate` flag on `bytdb_to_pg`. Refusing
    non-empty destinations protects live Postgres sites.
29. **Non-goal (no action):** page `opts.item_ids` saved as `[]` instead of
    `null`. Harmless, and identical on both backends.
30. **Non-goal (no action):** GET form/list/show handlers still return errors
    rather than flashes. Page definitions are static and only fail on a code
    bug.
31. **Non-goal (declined this session):** a transaction around role/permission
    writes. Delete-before-insert ordering bounds a partial failure to fewer
    grants, and the Executor seam has no `Begin`.
