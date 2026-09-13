# Session: Giving by month, year navigation, CSV export

**Session:** https://claude.ai/code/session_01NQNwAU3z6vZ7GgJDWAPfjE
**Date:** 2026-09-13
**Continues:** `2026-0913-1623-role-based-admin-access.md`

## What happened

The user asked for a page showing current giving:

- Default to year to date (YTD), grouped by month.
- Navigate backwards year by year until a year has no data.
- Export as CSV with headings.

Mid-task the user said to update the existing read-only giving page
(`/admin/giving`, built last session) rather than add a new one. That is what
was done. The old paged grid on that page was replaced by the report.

Built and tested on bytdb (unit tests, a bytdb query test, the HTTP smoke
script). The full `go test ./...` passes and the cema and ccswm binaries build.
**Not clicked through in a browser.**

## Behaviour

- **`GET /admin/giving[?year=YYYY]`** (`charges.read`). No year means the
  current year (YTD).
  - **Toolbar:**
    - "« prev year" is shown while `year > earliest year with any charge`.
    - "next year »" is shown while `year < current year`.
    - "Year to date" shortcut when two or more years back.
    - "Export CSV" button.
  - **Year param clamping:** empty, invalid or future → current year; before
    the first data → earliest year.
  - **"By month" card:** Month, Gifts, Gross, Refunded, Net, plus a Total
    footer.
    - Past year: all 12 months. Current year: January to the current month.
    - Empty months are shown. Months with gifts link to their card.
  - **One card per month with gifts,** newest month first, gifts newest first.
    The card title carries the gift count and net subtotal.
  - **Unpaid charges** are listed greyed (`af-muted`) and excluded from all
    money totals. A note gives their count.
- **`GET /admin/giving/csv[?year=YYYY]`** (`charges.read`): the same year as
  the page.
  - **Headings:** Date, Month, Name, Email, Amount, Refunded, Net, Status,
    Description, Comment, Receipt Number, Receipt URL.
  - **Rows:** one per charge, oldest first. Month is `2026-03` so a
    spreadsheet can pivot.
  - **Amounts:** plain decimals (`1234.56`). Net is blank for unpaid charges.
  - **File format:** UTF-8 BOM (Excel) and CRLF line endings.
  - **Formula-injection guard:** a leading `= + - @ \t \r` gets a `'` prefix.
    Name, comment and description come from the public giving form.
  - **Headers:** `Content-Disposition: attachment`,
    filename `giving-2026-ytd.csv` or `giving-2025.csv`,
    `Cache-Control: no-store`.

## Design

```
LoadGivingYear ──► EarliestGivingYear   (SELECT created_at ... IS NOT NULL ORDER BY ASC LIMIT 1)
               ──► charges in [Jan 1 year, Jan 1 year+1)   site-local time, half-open
               ──► GroupGivingYear      (pure; months + totals)
                        │
          ┌─────────────┴──────────────┐
   module render (HTML)          WriteGivingCSV
```

- **One loader for page and CSV** (`resource/payment/giving_report.go`), so
  they can't disagree about a year's gifts.
- **SQL portable to bytdb:** single-table range filter and `ORDER BY`, with no
  `MIN`, `GROUP BY` or `SUM`. Grouping and sums happen in Go, because a year's
  charges are small and the page lists them all anyway. `ORDER BY ... LIMIT 1`
  is used instead of `MIN()`. Charges are also sorted in Go by
  (`created_at`, `id`) for stable ties.
- **Time zone:** month and year cuts use `now.Location()` (`time.Local`). A gift
  at 8pm on Dec 31 local stays in December.
  - Under `TZ=UTC` (k8s) the cuts are UTC.
  - `EarliestGivingYear` ignores future-dated rows (clock skew) so navigation
    can't pass the current year.
  - `GroupGivingYear` extends the YTD month range if a charge is dated after
    the current month.
- **Totals:**
  - Only `paid` charges count.
  - `Net = Gross − Refunded`.
  - Refunded is capped to `[0, amount_paid]` per charge (`refundedCents`).
- **No paging:** paging would split a month and its subtotal. `Limit: 50` was
  removed from `page.GivingList()`.
- **Escaping:** donor text goes through `html.EscapeString` then `.T()`.
  element's `.TE()` exists in the workspace (v0.7.0), but cema and ccswm pin
  element v0.5.4, which lacks it. The first cema build failed on it.
- **Receipt link** is rendered only for a plain `https://` URL with no quotes,
  brackets or spaces.
- **Year param plumbing:** modules render from params (no request context), so
  `basectlr.RenderPageListWithOptsRWeb(pg, ctx, mainOpts)` passes
  `{"year": ...}`. It shares the production panic recovery that a direct
  `template.Page` call would skip.
- **CSV totals row declined:** it breaks sorting and filtering, and a
  spreadsheet computes it.

## Changes

- `resource/payment/giving_report.go` (new):
  - `GivingTotals` (with `Net`), `GivingMonth` (`Key`, `Label`), `GivingYear`
    (`YTD`, `HasPrev`, `HasNext`, `CSVFilename`).
  - `LoadGivingYear`, `EarliestGivingYear`, `ResolveGivingYear`,
    `GroupGivingYear`.
  - `GivingCSVHeadings`, `WriteGivingCSV`, `csvText`, `decimalCents`.
- `resource/payment/module_giving_list.go`: rewritten render (toolbar, summary
  card, monthly cards).
  - `GetData(rawYear)` replaces the limit/offset query.
  - `dollars` now groups thousands (`$1,234.56`).
  - New `plural`, `isAre`, `renderGivingMonth`, `renderReceiptLink`.
  - `chargeStatus` unchanged.
- `payment_controller/admin_giving_rweb.go`: `AdminListGivingRWeb` passes
  `year`. New `AdminGivingCSVRWeb` buffers the CSV and then writes the headers
  and bytes.
- `basectlr/base_controller_rweb.go`: new `RenderPageListWithOptsRWeb`.
- `router_rweb.go`: `ad.Get("/giving/csv", req(authz.ChargesRead, ...))`.
- `page/payment_pages.go`: `GivingList` drops `Limit`, with a comment.
- `page/admin_home.go`: Giving card description mentions months and CSV.
- `template/admin_css.go`: `.af-toolbar`, `.af-yearnav`, `.af-yearnav__label`,
  `.af-yearnav__period`, `a.af-btn`, `.af-table-wrap`, `.af-table` (`th`, `td`,
  `.af-num`, `td.af-wraptext`, `tfoot`, `tr.af-muted`). The header comment
  vocabulary was updated.
- `resource/payment/giving_report_test.go` (new):
  - `ResolveGivingYear` cases.
  - YTD grouping in a UTC−6 zone (Jan 31 20:00 local stays in January).
  - Refund/pending totals; a past year has 12 months.
  - CSV (BOM, CRLF, headings, formula guard, quoting, blank Net for pending,
    filenames); `dollars` grouping.
  - `TestLoadGivingYearAgainstBytDB`: boundary rows at Jan 1 00:00 and Dec 31
    23:59:59, a NULL `created_at` row, clamping to the earliest year, and the
    previous year's 12 months.
- `test_scripts/roles_smoke/main.go`: 4 new checks, 24 total, all pass:
  - YTD default with month label, `$2,500.00` and a back link.
  - Donor markup escaped.
  - The earliest year has no further back link but has a forward link.
  - CSV content type, filename and headings, with only this year's rows. A
    users-only role is refused the CSV with a 303 and no body.

## Verification

- `go build ./...`: OK. `go vet` on the touched packages: OK.
- `go test ./...`: all packages ok.
- `go run ./test_scripts/roles_smoke`: all 24 checks pass.
- cema and ccswm binaries build (built to the scratchpad, not the repo).
- Local Postgres (`church_development`): the earliest-charge and year-range
  queries run without error. The dev DB has **0 charges**, so there was no data
  comparison.
- **Not done:** browser view of the page (layout, phone width, CSV opened in
  Excel).

## Gotchas found

- **The Write tool turned a `"﻿"` escape into a literal BOM byte.** Go
  rejects that ("illegal byte order mark"). It was fixed with perl
  `s/\xEF\xBB\xBF/\\uFEFF/g`. Watch for this with any `\uXXXX` in written
  files.
- **element `.T()` does not escape.** `.TE()` does, but only from element
  v0.7.0, and site binaries pin v0.5.4. Use `html.EscapeString` for DB values
  in framework code.
- **`psql` isn't on PATH.** It is at `/opt/homebrew/opt/postgresql@16/bin/psql`.
- **gopls reports "go.work requires go >= 1.26.1 (running go 1.25.4)"** on
  every file. This is editor-only noise; `go build` and `go test` work.
- **Pre-existing gofmt drift, left alone:** `basectlr/base_controller_rweb.go`
  (the `RenderPageListRWeb` comment indentation) and `page/payment_pages.go`
  (import order and field alignment). This is in addition to last session's
  list.

## Next

1. **New:** browser click-through of `/admin/giving` on a running site:
   - year navigation limits
   - month anchors
   - the unpaid-charge note
   - phone-width table scrolling
   - CSV download opened in Excel (accented names, formula guard, numeric
     amounts)
2. **New:** check the giving report against real charge data on Postgres
   (e.g. a copy of a site DB). The dev DB has none. Compare the monthly totals
   with Stripe's dashboard for one month.
3. **New:** set the site time zone explicitly on k8s. Add `TZ` or a config
   option, or month cuts will be UTC; this includes giving months. Consider a
   config-driven `*time.Location` instead of `time.Local`.
4. **New, optional:** bump cema and ccswm to element ≥ v0.7.0 so framework code
   can use `.TE()`. Until then, don't use `.TE()` in church packages.
5. **New, optional:** a summary-only CSV (month totals), if the treasurer wants
   one alongside the per-gift export.
6. Run `goose up` for the roles migration (`20260913160000_CreateRolesTables.sql`)
   on `church_development` and on any Postgres site. Also still pending: the
   `event_locations` migration.
7. Browser click-through of the role screens on a running site:
   - role form (matrix toggles, auto-Read)
   - user form Roles card and locked states
   - flash messages on refusals
   - publish switch disabled for an Editor

   (The "giving list paging" item is superseded by item 1.)
8. Filter the nav's Admin submenu by permission. Map known `/admin/…` URLs to
   their read permission in `menu.buildMenu`. DB-stored admin menus currently
   show dead links to unpermitted users.
9. Add "+" / delete visibility by permission to the other admin list modules
   (articles, sermons, events, pages, menus). Handlers already refuse; this is
   UI polish.
10. **Optional:** a guard against removing the last role-holder with
    `roles.update` (non-SuperAdmin lockout). SuperAdmin remains the recovery
    path.
11. **Optional:** consider an explicit permission (or SuperAdmin-only) for
    `/debug/*`. Today any admin can toggle process-wide debug state.
12. Decide whether chat/prayer-wall moderation should move from legacy
    `users.role` to a permission (e.g. `chat.moderate`). It was left on the
    legacy role to avoid touching the mobile contract.
13. Run `test_scripts/roles_smoke` in CI, or convert it to a Go test with a cfg
    fixture, so route wiring can't silently lose a `Require`. It now also
    covers giving and CSV.
14. cema's `pg:` fix (`12049cf`) is only on `feature/site-themes`. Merge it to
    cema's main branch before building cema from there.
15. Boot cema locally with no `db.type` against Postgres. Confirm there is no
    `bytdb serving` line and that pages render (recipe in `2026-0912-1718-…`,
    minus `DB_TYPE=postgres`).
16. Document the `db:` block (`type: postgres|bytdb`, `file`, `listen`) in
    `cema/cfg/options-sample.yml` and `ccswm/cfg/options-sample.yml`.
17. Update docs that still describe bytdb as the default:
    - `deploy/k8s/README.md` §migration step 6
    - `ai_docs/fable_bytdb_k8s_readiness.md`
18. Start Docker and run `./deploy/deploy.sh images`, the one unproven fix from
    2026-08-01. If bimg/libvips 8.15 causes trouble, pin an older Alpine or use
    a pure-Go resizer.
19. Provision LKE + Object Storage and fill `deploy/backup.env`. Then run
    `./deploy/deploy.sh preflight infra`, point DNS, and run
    `./deploy/deploy.sh base seeds secrets images sites verify`.
20. Create `ccswm/cfg/options.yml` from the sample. It needs a real `pg:` block,
    or `db.type: bytdb`.
21. Move `resource/chimage` uploads onto IDrive e2 beside sermon media, retiring
    the `dist/img` volume mount.
22. From the readiness doc:
    - Call `CloseDB()` on SIGTERM in `ServeRWeb`.
    - Migrate `resource/dbbackup` off aws-sdk-go-v2 onto `replicate/s3`.
23. **Optional hardening:** `imagePullSecrets` (if the ghcr packages go private)
    and a www→apex redirect.
24. Live-check the 2026-09-12 flash changes on a running site (recipe in
    `2026-0912-1718-…`):
    - bad event coordinates or an expired csrf
    - empty menu `items` / page `modules`
    - referrer guard after a refused save
    - `token.txt` permissions on a fresh DB
25. Live-check the admin save flashes from `2026-0912-2125-…`:
    - expired csrf
    - blank article title
    - bad sermon date with audio chosen
    - password mismatch
    - stopped DB
26. Duplicate title on a new article/sermon (unique slug) still shows a generic
    "Error saving". Pre-check the slug on create as an InputError, working on
    both executors.
27. Seeds hygiene:
    - Replace the `resource/*/cfg/random_seeds.txt` fixtures with dummy seeds,
      including `resource/authz/cfg`.
    - Consider rotating the live `cfg/random_seeds.txt`.
28. **Optional:** sermon upload file handling (partial file on copy failure,
    orphan on DB failure, truncate-before-save). Fix with a temp file and rename
    after a successful upsert.
29. Keep typed values on a refused save: re-render the form from the posted
    presenter (events, articles, sermons, users) and/or wrap `UpsertEvent`'s
    writes in a transaction.
30. **Postgres coverage still missing:**
    - sermon create/import + audio
    - Stripe intent/history/webhook
    - `/chat/stream` SSE
    - image upload
    - giving report with data (item 2)
    - Optionally let `bytdb_wire_check` take a Postgres DSN.
31. Add `bytdb_to_pg` (and the `pg_to_bytdb` date fix), plus the Postgres smoke
    result, to `ai_docs/fable_bytdb_k8s_readiness.md` §7.
32. **Optional:** per-table content checksum in `bytdb_to_pg` beyond row counts.
33. **Non-goal (declined):** a `-truncate` flag on `bytdb_to_pg`. Refusing
    non-empty destinations protects live Postgres sites.
34. **Non-goal (no action):** page `opts.item_ids` saved as `[]` instead of
    `null`. Harmless, and identical on both backends.
35. **Non-goal (no action):** GET form/list/show handlers still return errors
    rather than flashes. Page definitions are static and only fail on a code
    bug.
36. **Non-goal (declined):** a transaction around role/permission writes.
    Delete-before-insert ordering bounds a partial failure to fewer grants, and
    the Executor seam has no `Begin`.
37. **Non-goal (declined this session):** a totals row in the giving CSV. It
    breaks sorting and filtering; a spreadsheet computes it.
38. **Non-goal (declined this session):** `GROUP BY`/`SUM` for the giving
    report. It isn't bytdb-portable, and the page lists every gift anyway.
