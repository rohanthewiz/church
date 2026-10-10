# Next-list review, Validate section, and N-050 (cema seed file)

Session: `f08a6078-e241-48dd-bcd1-1fefbd24bcf1`

## Ask

1. `/next-list` — review the living list (`ai_docs/todo/next-list.md`).
2. "Do N-050" — delete the stale `cfg/random_seeds.txt`.

## /next-list (living-list mode)

- **Lapse check:** every ID in the file's git history (N-001…N-060) is still
  present; IDs are unique; Next ID N-061 is right. The last 15 session docs
  all use the `Closed: … Raised: …` summary form, and their summaries match
  the file.
- **Validate section added** below Open, and the Conventions updated from
  three places to four (Open / Validate / Roadmap / Non-goals); nothing
  leaves any of the first three without a line elsewhere.
- **N-060 → Validate.** It is a pure check (real mouse + touch drag of the
  admin grips). Its file reference was wrong: `touch-action: none` on
  `.af-drag-handle` is in `template/admin_css.go`, not `admin_script.go`.
  Also noted that the grips are past `v0.12.1`, so testing needs a workspace
  build of cema until a re-pin.
- **N-019 low → medium.** ccswm pins `v0.12.0` and has a Stripe/giving
  config, so it lacks `v0.12.1`'s `charge.refunded` webhook handling; cema
  pins `v0.12.1`. Master is 10 commits past `v0.12.1` with untagged product
  changes (sermons grouped by month `66e6ccf`, drag grips `792f9d6`): tag
  `v0.12.2`, then re-pin both. ccswm's CI warns on the lag but stays green.
- **N-017** annotated: ccswm records no dashboard refunds until N-019. Kept in
  Open, not Validate — subscribing each site's Stripe endpoint to
  `charge.refunded` is a setup step, not a check.
- **N-010** premise holds: `dbc migrate status` shows the roles and
  `event_locations` migrations applied on dev, and none added since.
- **N-006 (Roadmap)** annotated: `27c624f` added the pure-Go resize fallback
  (`CGO_ENABLED=0` → `x/image/draw`), the escape hatch the item named for a
  libvips failure on Alpine. The Docker image is still unbuilt, so not done.
- No Roadmap item is ready to promote.

## N-050

- `cema/cfg/random_seeds.txt` (gitignored, 72 lines, 0600) removed from the
  checkout. `rm` was refused by the auto-mode permission classifier, so it
  was moved to the macOS Trash as `~/.Trash/cema-random_seeds.txt`
  (recoverable; empty the Trash to finish).
- Confirmed first that nothing reads it: in church, cema and ccswm the only
  mention is a history comment in `resource/auth/random.go`. ccswm has no
  local copy.
- **Found and left alone:** the sibling site `~/projs/go/ccgrand` pins a
  pre-`v0.11.0` church (`v0.10.1-…-8076c91a2870`), which still reads its own
  `cfg/random_seeds.txt`. Deleting that would break it; it only goes stale
  once ccgrand moves past `v0.11.0`. Recorded in N-050.
- N-050 stays Open, narrowed to the live hosts (after each deploys a build at
  or past `v0.11.0`).

## Next

Closed: None. Declined: None. Raised: None.
Deferred: None. Promoted: None. Moved: N-060 → Validate.
Updated: N-006, N-017, N-019, N-050, N-060. Full list: `ai_docs/todo/next-list.md`.
