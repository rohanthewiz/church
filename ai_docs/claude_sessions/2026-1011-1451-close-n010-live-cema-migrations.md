# Close N-010: migrations run on live cema

Session: `1f4a29eb-ab9a-4e72-bcdb-b07ea04c9094`

## Ask

The owner pasted next-list item N-010 ("run the roles and `event_locations`
migrations (`dbc migrate up`) on any Postgres site before deploying current
church to it") and said it was already done: the migrations have been run on
the live site, calvaryeastmetro.org.

## What was done

- **Cross-checked the other repos.** cema's own copy of this item (cema N-001,
  "deploy cema and run `dbc migrate up` on real prod … cema's instance of
  church's N-010") was already closed on 2026-10-04, which agrees with the
  owner's statement.
- **ccswm stays a caveat.** `ccswm/main.go:30` shows ccswm also defaults to
  Postgres (bytdb is opt-in). Nothing in the repos says whether ccswm has a
  live Postgres database, so the closed entry says that ccswm, or any new
  Postgres site, still needs `dbc migrate up` before it gets current church.
- **Moved N-010 to Closed** in `ai_docs/todo/next-list.md`, committed as
  `0381c3c`. This doc then added its filename stem to the closed entry.

The live database was not inspected from this session. The closure rests on
the owner's statement and cema's closed N-001.

## Next

Closed: N-010. Declined: None. Raised: None.
Deferred: None. Promoted: None. Moved: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
