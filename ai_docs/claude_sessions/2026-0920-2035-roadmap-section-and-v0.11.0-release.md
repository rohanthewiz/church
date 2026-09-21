# A Roadmap section for the next list, and the church v0.11.0 release

Session id: `6b2be37e-4a63-482d-a67f-0bb548ab9b01`

Two asks. First: the next list had Open and Non-goals but nowhere for work we
want to do later, so everything wanted sat in Open whether or not it was next.
Add a Roadmap section and move the bytdb, Docker, k8s/LKE and object storage
work and testing into it. Second: quick-check local cema, and if good, tag a
church release and pin cema and ccswm to it.

## The Roadmap section

`ai_docs/todo/next-list.md` now has four sections: Open, Roadmap, Non-goals,
Closed. The distinction is intent, not size or age:

- **Open** — what we intend to pick up next.
- **Roadmap** — what we want to do in the future, but not immediately. Parked,
  not declined.
- **Non-goals** — what we are likely not to do.

Design choices:

- An item keeps its header line (`ID · raised · value`) unchanged in Roadmap,
  so a move between Open and Roadmap is a pure cut and paste, and age still
  computes from `raised`. When an item was deferred is left to git history and
  to the track note, rather than a per-item field that would change the line.
- The no-silent-deletion rule was widened: nothing leaves Open *or* Roadmap
  without a line in another section.
- Roadmap groups items under a short track note. The first is the
  **Infrastructure track**, deferred together 2026-09-20; the sites stay on
  Postgres on their current hosts meanwhile.

Moved to Roadmap (11): N-001 (LKE + Object Storage provisioning and deploy),
N-002 (boot a site on bytdb), N-006 (Docker image build), N-007 (ccswm
`options.yml`, IDrive, uploads mount), N-008 (k8s hardening), N-011 (readiness
doc §7), N-012 (`bytdb_to_pg` checksum), N-016 (`db:` block in site samples),
N-023 (`images_to_e2` before cema's cutover), N-024 (DB backup against real
object storage), N-027 (`core/s3ops` onto `replicate/s3`).

Two judgment calls, both left in Open and reported to the user: N-004 (unique
index on `charges.payment_token` — payment hardening that only names bytdb as
a condition) and N-009 (Postgres coverage — one bullet touches
`bytdb_wire_check`, but the item is Postgres testing).

### Tooling updated to match (global, outside the repo)

Without these the new section would have read as a lapse.

- `~/.claude/skills/next-list/SKILL.md`: defines the three unfinished sections;
  an Open↔Roadmap move is not a lapse; Roadmap items get a lighter pass (closed
  if the code shows them done, flagged if one looks ready to promote, never
  promoted or parked by the skill — what is "next" is the user's call); the
  sorted view leaves them out and names them in one `Roadmap (parked):` line;
  seeding writes the section, empty unless a session doc says an item is
  deferred.
- `~/.claude/commands/sess-save.md`: sessions move deferred items into Roadmap
  and promoted ones back; `## Next` gains `Deferred:` and `Promoted:` parts.

Committed as `e651064` and pushed.

## cema check, then v0.11.0

The cema already listening on 8088 had started at 19:32, ten minutes before the
binary was last rebuilt and before the seed pool commit, so it was not evidence
of anything. It was left running. A fresh build against workspace church was
booted beside it with `SERVER_PORT=8099`:

- no seed-file read in the boot log; `config.TimeZone is America/Chicago`
- `/`, `/articles`, `/events`, `/sermons`, `/login`, `/api/v1/articles` at 200
- `/admin/articles`, `/admin/home`, `/debug/show` redirect (303) to `/login`
- session keys are 64-hex, from `crypto/rand`
- SIGTERM: "draining in-flight requests" then "Database closed; shutdown
  complete"
- all 26 church packages pass; cema builds and vets; church CI green

The `Error sending log to Slack: not_authed` lines are local config noise.

**Version.** `v0.11.0`, not `v0.10.1`. `v0.10.0` dates from 2025-07-26 and
master is 116 commits past it: Echo→rweb, the bytdb backend, roles and
permissions, the giving report, and the seed pool removal, which changes what
a site's `cfg/` must hold. Pre-1.0, that is a minor bump. Annotated tag on
`e651064`, pushed.

**Pins.** In each site, outside the workspace:

    GOWORK=off go get github.com/rohanthewiz/church@v0.11.0
    GOWORK=off go mod tidy && GOWORK=off go build -o /dev/null . && GOWORK=off go vet ./...

`GOWORK=off` matters: with `go.work` active the local church is used and the
pin is never exercised, which is the very gap N-049 described. cema `a1bae84`
and ccswm `feaa843` replace the `18713dd` pseudo-version; only `go.mod` and
`go.sum` changed. Both sites' CI went green on the `pinned` and
`church-master` jobs. ccswm's uncommitted `.gitignore` edit (`.cats-todo`) was
left out of the commit and is still pending there.

N-049 closed in `b9b3788`, one commit past the tag (list only).

## Gotchas

- A running local site is not proof of the current code. Check `ps -o lstart`
  against the binary's mtime and the commit before trusting it; boot a fresh
  build on another port with `SERVER_PORT` rather than stopping the user's.
- The owner's step from N-048/N-049 (delete stale seed files) lived only inside
  Closed entries, which is a quiet way to lose open work. It is now N-050.

## Commits

- church: `e651064` Roadmap section · tag `v0.11.0` · `b9b3788` close N-049 ·
  this doc
- cema: `a1bae84` Pin church to v0.11.0
- ccswm: `feaa843` Pin church to v0.11.0

## Next

Closed: N-049. Declined: None. Raised: N-050.
Deferred: N-001, N-002, N-006, N-007, N-008, N-011, N-012, N-016, N-023, N-024,
N-027. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
