# Session: auth leaks + admin form errors as flashes (Next items 7 and 8)

**Session:** https://claude.ai/code/session_01XmyuJDjLKSFLe5DSTmjVH6
**Date:** 2026-09-12
**Continues:** `2026-0912-1718-postgres-site-smoke-test.md` (same session)

## What happened

Did items 7 and 8 from the previous doc's `## Next`. Build, vet and the full
`go test ./...` pass; cema builds. **Not exercised against a running site**:
the new redirects/flashes, and the referrer guard, are covered only by
compilation and by unit tests of the validation layer.

## Item 7: `resource/auth` leaks

- `resource/auth/random.go` `init()`: no longer prints the first 50 crypto
  seeds (they feed `RandomKey`: session keys, superadmin bootstrap token). Only
  the count is printed.
- `admin/bootstrap_superuser.go` `AuthBootstrap`: `token.txt` written with
  `0600` (was `os.ModePerm`), then `os.Chmod(tokenFile, 0600)`. The Chmod is
  needed because `os.WriteFile` applies perm only on create, so a token file
  from an earlier boot kept `rwxr-xr-x`. Write errors are now logged (were
  ignored). Uses the `tokenFile` const instead of the duplicated literal.

## Item 8: bare 500s → flash redirects

### Bug found underneath (event)

`Presenter.UpsertEvent` inserted/updated the `events` row first, then parsed and
validated recurrence and location (they need the event ID), with **no
transaction**. A bad latitude on a *new* event therefore created the event
without its location and returned an error; a retry created a duplicate.

### Fix: validate before the first write

New `resource/event/validate.go`:

- `InputError{Msg, Err}`: an admin-input refusal. `Error()` = `Msg[: Err]`,
  `Unwrap()` = `Err`.
- `UserMessage(err) (msg, ok)`: `errors.As` for `*InputError`. Chosen over serr's
  `SetUserMsg` because `serr.UserMsgFromErr` only type-asserts the outermost
  error and these travel up through `serr.Wrap`. (`SErr` has `Unwrap`, so
  `errors.As` works through it.)
- `Presenter.Validate()` (no DB): title, date+time (`eventDateTime`, same
  server-zone parse as `modelFromPresenter`), `parseRecurrence`, `parsePoint`.
- `UpsertEvent` calls `p.Validate()` first. `upsertRecurrenceRule` and
  `upsertEventPoint` now reuse `parseRecurrence`/`parsePoint` and set
  `EventID`; `UpsertRecurrence`/`UpsertEventPoint` still validate at the DB
  boundary. `modelFromPresenter`'s own title/date checks left in place.
- Note: `SErr.Error()` returns only the core message (fields/location are not
  in it), so Validate messages pass straight into flashes.

### Controller behavior (events, menus, pages)

```
expired csrf            ──► form (RedirectRWebWarn)    nothing written
refused input / no JSON ──► form (RedirectRWebError)   nothing written
DB handle / upsert fault──► list (RedirectRWebError)   partial write possible
```

- Form URL: `/admin/<res>/edit/<id>` when id is non-empty and not `"0"`, else
  `/admin/<res>/new`.
- Server faults go to the list, not the form: an event insert can succeed before
  the recurrence/location write fails, so "new" would invite a duplicate.
- Menu: empty `items` and JSON unmarshal failure → form with error flash (logged,
  since it means `preSubmit()` JS did not run). Page: empty `modules` → same.
- Redirect re-renders the form from the DB; **typed-but-unsaved values are lost**.
  Flash says the save did not happen.
- Unused `errors` imports removed from the three controllers; `time` from
  `resource/event/queries.go`. Log label typos "Error in event upsert" in the
  menu/page controllers corrected to menu/page.

### Referrer guard

`context/rweb_helpers.go` `SetFormReferrerRWeb`: if the `Referer` path equals
the current request path, keep the existing `FormReferrer`. Without it, the 303
back to the edit form (browser keeps the POSTing form as Referer) would record
the form as its own referrer, so the next successful event save "returned" to
the form. Applies to article/sermon/event edit handlers (the callers). Uses
rweb `Request().Path()`.

### Tests

`resource/event/validate_test.go`: `TestPresenterValidate` (minimal ok, point +
monthly-last rule ok, blank title, blank time, bad date, latitude only, lat 300,
non-numeric longitude, monthly week 5, bad until) and
`TestUserMessageThroughWrap` (classification survives `serr.Wrap`; plain server
error not misclassified).

## Environment notes

- gopls diagnostics "go.work requires go >= 1.26.1 (running go 1.25.4)" are an
  editor toolchain mismatch; the shell `go` is 1.26.5 and builds fine.
- `event_controller_rweb.go` and `context/rweb_helpers.go` were already not
  gofmt-clean at HEAD (trailing whitespace, no final newline). Left as-is to keep
  the diff focused.
- `.cats-todo/` (personal, untracked) still deliberately not committed.

## Next

1. Start Docker and run `./deploy/deploy.sh images`, the one unproven fix from
   2026-08-01. If bimg/libvips 8.15 fights: pin older Alpine or replace bimg with
   a pure-Go resizer (restores `CGO_ENABLED=0` / static image).
2. Provision LKE + Object Storage, fill `deploy/backup.env`, then
   `./deploy/deploy.sh preflight infra` → DNS to NodeBalancer IP →
   `./deploy/deploy.sh base seeds secrets images sites verify`.
3. Create `ccswm/cfg/options.yml` from the sample (cema already has one).
4. Move `resource/chimage` uploads onto IDrive e2 beside sermon media, retiring
   the `dist/img` volume mount.
5. From the readiness doc: SIGTERM → `CloseDB()` in `ServeRWeb`; migrate
   `resource/dbbackup` off aws-sdk-go-v2 onto `replicate/s3`.
6. Optional hardening: `imagePullSecrets` if ghcr packages go private; www→apex
   redirect.
7. **New:** live-check this session's changes on a running site (recipe in
   `2026-0912-1718-…`): event lat 300 / one coordinate / expired csrf → back on
   form with flash and no new row; menu/page post with empty `items`/`modules` →
   form flash; successful event edit after a refused save returns to the list
   (referrer guard); fresh DB without superadmin → `token.txt` is `-rw-------`,
   and boot log shows no seeds.
8. **New:** same bare-500-on-expired-csrf pattern remains in
   `sermon_controller` (2 sites), `user_controller`, `article_controller`
   upserts. Switch to `app.RedirectRWebWarn` back to the form.
9. **New, optional:** keep typed values on a refused event save (re-render the
   form from the posted presenter instead of redirecting), and/or wrap
   `UpsertEvent`'s three table writes in a transaction so server faults cannot
   leave a partial event.
10. **Postgres coverage still missing:** sermon create/import + audio, Stripe
    payment intent/history/webhook, `/chat/stream` SSE, user create/delete, image
    upload. Optionally make `test_scripts/bytdb_wire_check` accept a Postgres DSN
    so the same 35 checks prove both backends.
11. Apply the pending `event_locations` goose migration to `church_development`.
12. Add `bytdb_to_pg` (and the `pg_to_bytdb` date fix) plus the Postgres smoke
    result to `ai_docs/fable_bytdb_k8s_readiness.md` §7.
13. **Optional:** per-table content checksum in `bytdb_to_pg` beyond row counts.
14. **Non-goal (declined):** a `-truncate` flag on `bytdb_to_pg`. Refusing
    non-empty destinations protects live Postgres sites.
15. **Non-goal (no action):** page `opts.item_ids` saved as `[]` instead of
    `null` after an admin page save. Go nil-vs-empty-slice JSON, harmless,
    identical on both backends.
