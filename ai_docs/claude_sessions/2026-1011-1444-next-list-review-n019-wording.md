# Next-list review; N-019 wording

Session: `b331d8f0-853b-4c62-b44f-0ce620010531`

## Ask

`/next-list`: review the living list (`ai_docs/todo/next-list.md`).

## /next-list (living-list mode)

- **Lapse check:** every ID in the file's git history (N-001 to N-060) is
  still present. No ID appears twice, and Next ID N-061 is correct. The last 15
  session docs all use the `Closed: … Raised: …` summary form, and their
  Closed/Raised/Moved lines match the file.
- **Premises re-checked against the code and the site repos:** all still hold.
  - **N-010:** `dbc migrate status` shows the roles and `event_locations`
    migrations applied on dev (2026-09-13). The live Postgres sites are still
    unknown.
  - **N-017:** `test_scripts/giving_reconcile` exists, and the webhook handles
    `charge.refunded`. The production run is still the owner's step.
  - **N-019:** ccswm pins `v0.12.0` and cema pins `v0.12.1`. No `v0.12.2`
    tag exists yet. Master carries untagged product changes (`66e6ccf`,
    `792f9d6`).
  - **N-050:** neither local checkout has `cfg/random_seeds.txt`. Only a
    history comment in `resource/auth/random.go` mentions it. The live hosts
    are still pending.
  - **N-060:** `.af-drag-handle` is in `template/admin_css.go`, and the
    delegated `pointerdown` handler is in `template/admin_script.go`, as the
    item says.
- **Roadmap light pass:** nothing is done or ready to promote. ccswm still has
  no `cfg/options.yml` (N-007), and neither site's `options-sample.yml` has a
  `db:` block yet (N-016).
- **Edit:** N-019 said master was "10 commits past `v0.12.1`". The previous
  review's own commit made it 11, so the count went stale within a day. The
  item now points at `git log v0.12.1..HEAD` instead of a number.

### Sorted view (by age, N = 59 docs)

Open: N-010 (age 28 · medium), N-017 (age 22 · medium), N-019 (age 21 ·
medium), N-050 (age 16 · low). Validate: N-060 (age 1 · low).

### Suggestions (not acted on)

- Tagging `v0.12.2` and re-pinning (N-019) unblocks the most. It lets ccswm
  record refunds (N-017) and lets N-060 run on a pinned cema build.
- N-050 is old and low value, and it now waits only on a live deploy. It could
  move to the Infrastructure track on the Roadmap. The user decides.

## Next

Closed: None. Declined: None. Raised: None.
Deferred: None. Promoted: None. Moved: None.
Updated: N-019. Full list: `ai_docs/todo/next-list.md`.
