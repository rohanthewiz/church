# N-022: does any site need `chat.moderate` added to Publisher/Editor by hand?

Session ID: `94221085-fab0-4a83-8d9b-13d285c765ba`
Date: 2026-09-28

(Earlier in this session: `2026-0928-2339-n004-unique-index-recommendation`,
`2026-0928-2347-n005-theme-vars-material-form`,
`2026-0928-2351-n009-postgres-import-webhook-tests`,
`2026-0928-2354-n021-mobile-can-moderate`.)

## Goal

N-022 (optional): hand-add `chat.moderate` to the Publisher and Editor roles
on sites whose default roles were seeded before the permission existed.
Checked whether any such site exists before touching data. No code or data
changed.

## Background

- `authz.EnsureDefaultRoles` (`resource/authz/seed.go`) seeds Administrator,
  Publisher and Editor once — the first boot that finds `roles` empty — so a
  later change to the defaults never reaches an already-seeded site.
- `authz.CanModerate` = legacy editor-or-above (`LegacyModerator`) OR a role
  granting `chat.moderate`. The permission on Publisher/Editor therefore only
  matters for someone holding one of those roles whose base `users.role` is
  member (9).

## Findings

- **The window was ~2 hours, with no release in it.** `EnsureDefaultRoles`
  arrived in `0298241` (2026-09-13 16:25); `chat.moderate` joined the
  Publisher/Editor defaults in `de76881` (18:17). Tags: `v0.10.0` (2025-11)
  predates roles entirely; `v0.11.0` (2026-09-20) and later include both. Any
  site seeded from a released build has the permission.
- **Local Postgres DBs already have it** (read-only query via a scratch Go
  program, DSN built in the shell from cema's dev config, not printed):
  - `church_development`: defaults seeded 2026-09-13 19:10; Publisher and
    Editor grant `chat.moderate`.
  - `church_test`: same seeding time, same result.
  - Custom roles without it (Sermon Maintainer, Treasurer, Read Only, User
    Manager) are admin-made; granting it is an admin decision, not N-022.
- **No bytdb site files exist locally**, and the k8s sites have never booted
  (image never built), so they will seed the current defaults on first start.

## Outcome

Closed as nothing to do. The one residual case — a site outside this machine
first booted from an untagged build in that two-hour window — is noted in the
Closed entry as the reopen condition.

## Next

Closed: N-022. Declined: None. Raised: None.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
