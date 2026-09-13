# Session: admin save failures as flashes (article, sermon, user)

**Session:** https://claude.ai/code/session_01XmyuJDjLKSFLe5DSTmjVH6
**Date:** 2026-09-12
**Continues:** `2026-0912-1752-auth-leaks-form-error-flashes.md` (same session)

## What happened

1. Did item 8 from the previous `## Next`: expired-csrf bare 500s in the
   article, user and sermon controllers.
2. Then, on request, converted every other save failure in those three
   controllers into a flash redirect back to the form.

Build, vet and the full `go test ./...` pass; cema builds. **Not exercised
against a running site**: the redirects are covered by compilation only, the
validation layer by unit tests.

## Part 1: expired csrf → form with warn flash

| Handler | Redirect |
|---|---|
| `article_controller` `UpsertArticleRWeb` | `/admin/articles/edit/<id>` or `/new` |
| `user_controller` `UpsertUserRWeb` | `/admin/users/edit/<id>` or `/new` |
| `sermon_controller` `UpsertSermonRWeb` | `/admin/sermons/edit/<id>` or `/new` |
| `sermon_controller` `AdminSermonCleanupRunRWeb` | `/admin/sermons/cleanup` |

- Form URL from the hidden id field (`article_id`/`user_id`/`sermon_id`), same
  rule as events: non-empty and not `"0"` → edit, else new.
- Message: "Your form has expired and was not saved. Please refresh the form and
  try again." (cleanup: "...nothing was deleted...").
- Unused `errors` imports removed from article and user controllers; sermon
  still uses `serr` (ImportRWeb).

## Part 2: all other save failures

### Shared classification: `util/inputerr`

- `InputError{Msg, Err}`, `New(msg, err)`, `UserMessage(err) (msg, ok)` moved out
  of `resource/event/validate.go` into `util/inputerr/inputerr.go`.
- `resource/event` keeps its API via `type InputError = inputerr.InputError`,
  `inputErr` → `inputerr.New`, `UserMessage` delegating. Event controller and
  tests unchanged.
- Reason for a shared package: article/sermon/user must classify the same way
  without importing each other or `resource/event`.

### Presenters

- `article.modelFromPresenter`: blank title → `inputerr.New("An article needs a title")`.
- `user.modelFromPresenter`: password mismatch → InputError. (`errors` import removed.)
- `sermon.modelFromPresenter`: blank title and date parse now InputErrors, via
  the new shared `dateTaught()`. Removed its `[Debug] datetimez` println.
- New `resource/sermon/validate.go`: `Presenter.Validate()` (title + date, no DB,
  no FS) and `dateTaught()` (date + " 11:00 " + server zone, same parse as
  before; blank date refused explicitly). `modelFromPresenter` keeps its own
  checks because `import2.go` calls `Upsert` without `Validate`.

### Controllers

All failures redirect to the form with `RedirectRWebError`. Unlike events
(3 untransacted writes → server faults go to the list), these upserts are a
single Insert/Update, so a failed save wrote nothing and the form is safe.

```
input refused (InputError)   ──► form, reason + "was not saved"   not logged
bad role (user <select>)     ──► form, "Please choose a role"     logged
DB handle / upsert fault     ──► form, generic "not saved"        logged
sermon audio store fault     ──► form, "audio file could not be stored" logged
```

- **Sermon ordering fix:** `serPres.Validate()` now runs before `GetYear()` and
  the audio copy. Previously a bad date on an existing sermon was refused only in
  `Upsert`, after `os.Create` had already truncated the old audio file. Also
  `GetYear` picks the upload directory from `DateTaught`.
- Sermon: a partial local file is still possible after an `io.Copy` failure, and
  an uploaded file stays on disk if the DB save then fails.
- **User log leak fixed:** the upsert failure log used `%#v` of the presenter,
  which included the typed password. Password fields are blanked on a copy
  before logging.
- Article/sermon upsert failures now logged with id + title (article had no log).
- GET handlers (New/Edit/List/Show) still `return err`: `page.*Form()` etc. are
  static page definitions that only fail on a programming bug. Deliberately left.

### Tests

- `util/inputerr/inputerr_test.go`: message, `Error()` keeps cause, `errors.Is`
  through Unwrap, survives two `serr.Wrap` layers, server/nil not misclassified.
- `resource/sermon/validate_test.go`: Validate ok / blank title / blank date /
  bad date; `modelFromPresenter` title and date classified as input.
- `resource/user/user_presenter_test.go`: password mismatch is InputError.
- `resource/article/article_presenter_test.go`: blank title is InputError.
- `modelFromPresenter(nil, …)` is safe in tests: with `Id == ""`,
  `findByIdOrCreate` never touches the executor.
- Added `resource/user/cfg/random_seeds.txt` (copied from `resource/article/cfg`):
  `resource/auth` `init()` opens `cfg/random_seeds.txt` relative to the test
  package dir and `log.Fatal`s otherwise. Same fixture convention as 9 other
  packages.

## Environment notes

- **Seeds are committed:** every `resource/*/cfg/random_seeds.txt` fixture is
  byte-identical to the live `cfg/random_seeds.txt`, and all are tracked in git.
  Last session stopped printing seeds at boot because they are secret, but they
  are in repo history regardless.
- gopls "go.work requires go >= 1.26.1 (running go 1.25.4)" is still the editor
  toolchain mismatch; shell `go` builds fine.
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
7. Live-check the previous session's changes on a running site (recipe in
   `2026-0912-1718-…`): event lat 300 / one coordinate / expired csrf → back on
   form with flash and no new row; menu/page post with empty `items`/`modules` →
   form flash; successful event edit after a refused save returns to the list
   (referrer guard); fresh DB without superadmin → `token.txt` is `-rw-------`,
   and boot log shows no seeds.
8. **New:** live-check this session: expired csrf on article/sermon/user forms
   and sermon cleanup → form with warn flash; blank article title, bad sermon
   date (with an audio file chosen, on an existing sermon: old audio intact),
   password mismatch → form with reason; stopped DB → form with generic error.
9. **New:** duplicate title on a new article/sermon (unique slug) is the likeliest
   real failure and still shows a generic "Error saving". Pre-check the slug on
   create and refuse as an InputError; needs a not-found check that works on both
   Postgres and bytdb executors.
10. **New:** seeds hygiene: replace the `resource/*/cfg/random_seeds.txt` fixtures
    with dummy seeds, and consider rotating the live `cfg/random_seeds.txt` since
    it is in git history.
11. **New, optional:** sermon upload leaves a partial local file after a copy
    failure, and an orphan file if the DB save fails; and a same-name re-upload
    truncates the existing file before the save is known to succeed (write to a
    temp file and rename after a successful upsert).
12. Keep typed values on a refused save (re-render the form from the posted
    presenter instead of redirecting) — now applies to events, articles, sermons
    and users; and/or wrap `UpsertEvent`'s three table writes in a transaction.
13. **Postgres coverage still missing:** sermon create/import + audio, Stripe
    payment intent/history/webhook, `/chat/stream` SSE, user create/delete, image
    upload. Optionally make `test_scripts/bytdb_wire_check` accept a Postgres DSN
    so the same 35 checks prove both backends.
14. Apply the pending `event_locations` goose migration to `church_development`.
15. Add `bytdb_to_pg` (and the `pg_to_bytdb` date fix) plus the Postgres smoke
    result to `ai_docs/fable_bytdb_k8s_readiness.md` §7.
16. **Optional:** per-table content checksum in `bytdb_to_pg` beyond row counts.
17. **Non-goal (declined):** a `-truncate` flag on `bytdb_to_pg`. Refusing
    non-empty destinations protects live Postgres sites.
18. **Non-goal (no action):** page `opts.item_ids` saved as `[]` instead of
    `null` after an admin page save. Go nil-vs-empty-slice JSON, harmless,
    identical on both backends.
19. **Non-goal (no action):** GET form/list/show handlers in these controllers
    still return errors; the page definitions are static and only fail on a code
    bug.
