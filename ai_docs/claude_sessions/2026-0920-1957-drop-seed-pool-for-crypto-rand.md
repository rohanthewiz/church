# Drop the seed pool for crypto/rand

Session id: `12dca931-d34a-48bf-ace7-3955c2d1206a`

The user proposed dropping the "random seeds" approach in favor of a proper
random number generator. Done across church, cema and ccswm: `RandomKey()` now
draws straight from `crypto/rand`, and `cfg/random_seeds.txt` with everything
that existed to support it is gone.

## Why the pool was not worth keeping

`resource/auth/random.go` loaded `cfg/random_seeds.txt` in `init()` and built
each key as

```go
PasswordHash(RandomString(), fmt.Sprintf("%s%d%s", RandomString(), time.Now().UnixNano(), RandomString()))
```

where `RandomString()` picked a pool line using `crypto/rand`. So:

- **No added entropy.** The picks were already CSPRNG-driven; the pool only
  narrowed 256 bits of OS randomness to three ~6-bit choices (72 lines) plus a
  guessable nanosecond clock. Its secrecy was the whole security argument,
  which is exactly what N-014 had to defend the day before.
- **A hard boot dependency.** `init()` `log.Fatal`'d without the file, before
  `main()`. That forced a fixture copy into all 15 packages importing auth, a
  second key in every `<site>-config` Secret, a `deploy.sh seeds` phase, a
  production guard (`checkSeedsForEnv`) and a sample-equality guard.
- **An scrypt derivation (N=16384) per session key** on the login path.
- A latent off-by-one: `RandomInt(len-1)` never picked the last line.

## What changed

### church — `9c796bd`

- **`resource/auth/random.go`** rewritten: no `init()`, no pool, no
  `checkSeedsForEnv`. `RandomKey()` = hex of 32 `crypto/rand` bytes;
  `RandomString()` = hex of 16 bytes (kept for its one caller, easy_tabs DOM
  ids); `RandomInt` unchanged. `rand.Read`'s error is deliberately ignored:
  since Go 1.24 it never returns one, crashing the process instead of handing
  back predictable bytes (go.mod is 1.26.1). The file header records the
  design reasoning.
- **Output shape preserved on purpose:** 64-char lowercase hex, the same as
  the scrypt version, so session cookies, Redis keys, form tokens and the
  SuperAdmin bootstrap token keep their format.
- **Tests:** `random_seeds_env_test.go` and `random_seeds_sample_test.go`
  deleted; new `random_test.go` pins key/string shape, uniqueness over 5000
  draws (a smoke test against an unfilled buffer, not a statistical test) and
  `RandomInt`'s range.
- **Fixtures:** `cfg/random_seeds.txt` removed from the module root,
  `auth_controller`, `payment_controller` and twelve `resource/*` packages,
  along with the header notes in tests and `test_scripts/` that explained them.
- **Deploy:** `deploy.sh` loses `cmd_seeds`, the `seeds` phase (usage, arg
  parsing, `all`, the phase-order diagram) and the `random_seeds.txt` key of
  `<site>-config`. Dockerfile and both site manifests' comments updated; the
  `/app/cfg` directory mount stays as is. `Dockerfile.dockerignore` keeps its
  `**/cfg/random_seeds.txt` line, now commented: old checkouts still hold the
  file and it was a secret.
- **Docs:** README seed section replaced by a "no longer needed" note;
  `CEMA_LOCAL_SERVER.md` and `deploy/k8s/README.md` (phase table, "what each
  site needs to boot" now two items, add-a-site steps) updated.

### cema — `56c95a6`, ccswm — `d1f23fa`

`cfg/random_seeds.txt.sample` deleted; README seed section replaced by the same
note. `.gitignore` keeps `cfg/random_seeds.txt` for the same reason as the
dockerignore line. ccswm's untracked `.cats-todo/` was left out.

## Compatibility

- **Existing logins survive.** Login compares
  `PasswordHash(password, stored_salt)` with the stored hash, both from the DB;
  `GenSalt` never read the pool. `RandomKey` outputs are stored, never
  re-derived.
- **Existing k8s Secrets** with a `random_seeds.txt` key keep working; the key
  is simply ignored.
- **Live sessions** are unaffected: keys already issued are just opaque
  strings in the session store.

## Verification

- `go build ./...` in church, cema and ccswm.
- `go test ./...` in church: all 26 packages pass, now with no seed fixture in
  any package directory — which is itself the proof that nothing still needs
  the file at init.
- `bash -n deploy/deploy.sh`.
- Not done: no site was booted against the change. `auth_flow_test.go` covers
  login → session key → cookie in-process.

## Gotchas

- `git add <deleted path>` fails with "pathspec did not match" when the
  deletion is already staged by `git rm`; the first cema/ccswm commit attempt
  aborted on that and was redone adding only the README.
- The editor's gopls reports `go.work requires go >= 1.26.1 (running go
  1.25.4)`; the shell's `go` is 1.26.5 and builds fine. It is an LSP toolchain
  mismatch, not a project problem.
- `resource/auth/crypt_scrypt.go` was already not gofmt-clean (N-028 territory);
  left alone.

## Next

Closed: N-048 (superseded — nothing reads the pool once a host runs this
build). Declined: None. Raised: N-049 (re-pin cema and ccswm to church at or
past `9c796bd`; until then their pinned-church builds still need a seed file
and no longer have a sample). Updated: N-001 (its `deploy.sh` command loses
the `seeds` phase). Full list: `ai_docs/todo/next-list.md`.
