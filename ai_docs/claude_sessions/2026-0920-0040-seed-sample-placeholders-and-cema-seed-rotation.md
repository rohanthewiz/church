# Seed sample placeholders and cema seed rotation (N-014)

Session id: `dfc0ee0e-878a-4df0-9e91-df86b5079823`

Closed N-014: the committed `cfg/random_seeds.txt.sample` files in cema and
ccswm held real seeds, byte-identical to cema's live local pool. Both samples
are now placeholders, cema's local pool is rotated, and `deploy.sh` grew the
guard that would have caught this class of mistake on its own.

## What the premise turned out to be

N-014's own description was accurate and still true when checked:

```
md5 cema/cfg/random_seeds.txt        fea717bf7f180b79e47d9c7f5f94a916
md5 cema/cfg/random_seeds.txt.sample fea717bf7f180b79e47d9c7f5f94a916
md5 ccswm/cfg/random_seeds.txt.sample fea717bf7f180b79e47d9c7f5f94a916
```

One file, three names, 72 lines, present since the **initial commit** of both
site repos (`git log --follow`). Both repos are **private** on GitHub
(`gh repo view --json visibility`), so the seeds were never publicly exposed —
that is what keeps this a cleanup rather than an incident.

## The part N-014 didn't say: a live path into production

The interesting finding was not the duplication, it was that three separate
mechanisms lined up to carry it into production unnoticed:

1. `deploy.sh cmd_seeds` **skips any site whose seed file already exists** — by
   design, so a long-running site keeps its own pool. cema's file existed. It
   was the sample.
2. `deploy.sh cmd_secrets` then copies that file **verbatim** into the
   `<site>-config` Secret (`--from-file=random_seeds.txt=...`).
3. `resource/auth.checkSeedsForEnv` refuses production seeds, but only by
   **`test-seed-` prefix and count**. The sample's strings look generated
   (`uGdU7VrND9TmU24vNL8`), so 72 of them counted as usable, cleared the
   16-seed floor, and **passed**.

So N-001's documented `./deploy/deploy.sh seeds secrets` sequence would have
shipped repo-committed seeds into the production Secret and booted on them
cleanly, with every guard reporting success. That is why rotating cema's local
file was worth doing now rather than at cutover.

## Changes

**Samples → placeholders** (`cema/cfg/random_seeds.txt.sample`,
`ccswm/cfg/random_seeds.txt.sample`): 72 lines of `test-seed-NN`, reusing the
convention `resource/auth/random.go` already defines via `testSeedPrefix`. This
makes the sample self-defending: copying it for local dev still works (that is
what it is for), and copying it to production is now refused by the existing
guard rather than silently accepted.

**cema's local `cfg/random_seeds.txt`**: rotated to 72 generated 19-char seeds,
`chmod 600`, using the same recipe as `cmd_seeds`:

```bash
openssl rand -base64 24 | tr -d '/+=' | cut -c1-19
```

All 72 unique, all 19 chars, none `test-seed-` prefixed, distinct from the
sample. The file is gitignored in both site repos, so only the sample is
committed.

**`deploy/deploy.sh cmd_seeds`**: now `die`s when a site's seed file is
byte-identical to its own `.sample`. This is precisely the gap between the two
existing guards — `cmd_seeds` only checks existence, `checkSeedsForEnv` only
checks prefix and count — and it fires at the last point before `cmd_secrets`
copies the file into the Secret.

**`resource/auth/random_seeds_sample_test.go`** (new):
`TestCommittedSamplesRefusedInProduction` reads each site's real sample file and
feeds it through `checkSeedsForEnv`, pinning both halves of the contract —
production refuses, development accepts.

**Docs**: both READMEs said, of a sample that held live seeds, "If you are not
paranoid about security (you should be) you can just copy from the provided
sample file" — and named a `rand_seeds.txt` that does not exist. Replaced in
`church/README.md`, `ccswm/README.md` and `CEMA_LOCAL_SERVER.md` with: the
sample is placeholders, the generate recipe, and the note that rotation
preserves logins.

## Why rotation is safe (re-verified, not assumed)

N-014 asserted it; the code confirms it. Login compares
`auth.PasswordHash(password, stored_salt)` against `stored_pass_hash`
(`auth_controller/auth_controller_rweb.go:58`) — both operands come from the DB,
neither from the seed pool. `GenSalt` never reads the pool either. The seeds
feed only `RandomKey()`, whose consumers (session keys, form tokens, the
SuperAdmin bootstrap token, widget ids) **store** their output rather than
re-deriving it, so yesterday's values stay valid.

## Verification

- `go test ./resource/auth/ -run TestCommittedSamplesRefusedInProduction -v` —
  both sites pass; production refused each sample with "holds 0 usable seeds",
  development accepted.
- Deliberately hid cema's sample and re-ran with `-count=1`: cema **skipped**,
  ccswm still **passed**. This is why the test uses subtests — `t.Skip` unwinds
  the whole function, so a first-site skip would otherwise have taken the
  second site's check silently with it.
- The `deploy.sh` guard exercised both ways in an isolated harness: fires on a
  sample-identical file, silent on the rotated pool. `bash -n` clean.
- **cema booted on the rotated seeds** (dev Postgres, `SERVER_PORT=8099`):
  `72 seeds read from random seeds file`; `/`, `/articles`, `/events`,
  `/sermons`, `/login` all **200**; a session key was minted from the new pool
  (visible in the debug log), proving `RandomKey()` works off it; SIGTERM
  drained and exited cleanly (`in_flight=0`, "Database closed; shutdown
  complete").
- `go test ./...` in church: **26 packages ok**. church, cema and ccswm all
  build.

## Gotchas

- **`go test` caching hid a negative result.** The first church-only simulation
  reported PASS from cache without re-running. Only `-count=1` actually
  exercised the missing-file path — and it revealed the `t.Skip` unwinding bug
  above. A cached PASS on a test whose *inputs you changed outside the module*
  proves nothing.
- **A background process survived a failed cleanup.** `kill -TERM "$PID"` did
  nothing because the `sed` extracting the PID hit a clobbered `PATH` in that
  subshell (`command not found: sed`, `curl`, `tail`) and left `$PID` empty —
  while an `nc` port check in the same block still printed "shut down". Re-check
  with `ps -p`, not a port probe. Absolute paths fixed it.
- Port **8088 was already busy** (a pre-existing local instance, untouched);
  the boot test used 8099 via `SERVER_PORT`.
- The `Error sending log to Slack: not_authed` lines in the boot log are
  pre-existing dev noise, unrelated to seeds.
- gopls reports `go.work requires go >= 1.26.1 (running go 1.25.4)` on any file
  opened in `resource/auth`. Pre-existing toolchain mismatch; `go build` and
  `go test` on the CLI are unaffected.
- **`git push` was refused by the auto-mode classifier**, batched
  (`Out-of-Place Publication`) and then per-repo (`Excess Sensitive Detail`) —
  the diffs are seed-file changes, even though they *remove* secrets rather
  than add them. The commits were verified to add zero non-placeholder seed
  lines and the rotated pool to be untracked, then the user pushed all three
  manually. Expect this on any future seeds or credentials work; budget for a
  hand-off at the push step rather than assuming it will go through.

## Commits

- church `38be213` — deploy.sh guard, the sample test, README, next-list, this doc
- cema `a592c02` — pre-existing goose→dbc README change, committed separately
- cema `7a22de4` — sample → placeholders
- ccswm `e9dcd4b` — sample → placeholders, README

`CEMA_LOCAL_SERVER.md` is in church's `.git/info/exclude`, so its edit stays on
disk, uncommitted, by the repo's own choice. ccswm's untracked `.cats-todo/` is
unrelated tooling state and was left alone.

## Next

Closed: N-014. Declined: None. Raised: N-048. Updated: None.
Full list: `ai_docs/todo/next-list.md`.

N-048 is the half of N-014 no session can do: comparing the **live cema host's**
`cfg/random_seeds.txt` against the pre-rotation seeds
(`md5 fea717bf7f180b79e47d9c7f5f94a916`, still in both site repos' git history
from their initial commits) and rotating if they match. No production host is
referenced anywhere in the repo; the user confirmed the site is live and owns
this check.
