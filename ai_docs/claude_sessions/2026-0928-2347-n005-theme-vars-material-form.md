# N-005: theme overrides for framework CSS vars + material-form slimming

Session ID: `94221085-fab0-4a83-8d9b-13d285c765ba`
Date: 2026-09-28

(Earlier in this session: N-004 review, see
`2026-0928-2339-n004-unique-index-recommendation.md`.)

## Goal

Next-list item N-005, a stylus tidy-up in the site repos (cema, ccswm):

1. Add `--af-*` (admin forms) / `--chg-*` (data grids) overrides to the sites'
   theme files; none existed.
2. Slim the old material-form classes, whose only user is
   `church/page/login_form.go`.

No framework (church) code changed; all edits are in `../cema` and `../ccswm`,
identical in both.

## 1. Theme → framework CSS variables

- **Delivery constraint.** The framework inlines `template.AdminCSS` and
  `grid.CSS` in `<head>` *after* the site's `app.css` link
  (`template/page.html.go`), declaring defaults on `.af-scope` / `.ch-grid`.
  A site override with the same selector would lose the cascade, so overrides
  use `body .af-scope` / `body .ch-grid` (0-1-1 beats 0-1-0, no
  `!important`). `<body>` always exists, and `.af-scope` sits on `#mid`, never
  on `body`.
- **One new theme variable**, `theme-ui-accent`, appended to all six theme
  files per site. Value = the theme's `theme-a-color` (link color), chosen over
  `theme-accent-bgcolor` because some accent backgrounds (Fellowship
  `#e2725b`, Horizon `#1d84b5`) are too light for light button text; every
  link color is dark enough.
- **Cobalt sets `null`** → the new partial emits nothing, so the live look of
  both sites (both run Cobalt) is byte-for-byte unchanged in behavior.
- **New partial `styles/styl/_styl/_framework_vars.styl`** (auto-included by
  `base.styl`'s `@require '_styl/*'`) maps it, accent family only:
  - `--af-accent`, `--af-accent-hover` (`darken 15%`), `--af-accent-soft`
    (`rgba .15`)
  - `--chg-accent`, `--chg-accent-soft` (`tint 90%`, near-white like the
    default `#eef3f9`)
  - Neutrals (text, card/input surfaces, borders) deliberately stay at the
    framework defaults, per AdminCSS's own theming contract.

## 2. Material form

Findings from compiling the old file:

- `mat-form()` at the top of `_material_form.styl` was **never a mixin**: a
  column-0 comment right after it kept Stylus from parsing a mixin body, so
  the rules compiled at the root on `@require`, and the `mat-form()` call in
  `_mid_layout.styl` emitted nothing. (First rewrite made it a real mixin,
  which nested everything under `body #main`; caught in the compiled diff and
  reverted to root emission to keep specificity and column-independence.)
- A column-0 brace block (`.form-help {`) partway down left everything after
  it **unscoped**: global `.form-group input`, `.form-group .control-label`,
  `.checkbox input { opacity: 0.00000001; position: absolute }`, `.button`, …
  Summernote's dialogs use `form-group` (5×) and `checkbox` (1×), so its
  Insert Link dialog was likely getting borderless inputs and an invisible
  "open in new window" checkbox.

Rewrite (501 → ~190 lines, identical in both sites):

- Only what the login markup uses: wrapper card + hover shadow, `.page-title`,
  `.form-group` spacing, input, floating `.control-label`, `.bar` focus
  underline, submit `.button` with ripple. All nested under
  `.wrapper-material-form`, emitted at the root as before.
- Variables prefixed `mf-` (`mf-body-bg`, `mf-shadow-N`) since they are now
  true globals; no name clashes found.
- Dropped: checkbox/radio helpers, `.form-help`, `.has-error`,
  select/textarea/file inputs, `.form-inner/.form-inline/.form-pack/
  .button-container/editable` (retired material page form). Go-side grep
  confirmed no emitters of those classes.
- Removed the dead `mat-form()` call in `_mid_layout.styl`, leaving a comment.

## Build

`dist/css/` is committed in both site repos. Rebuilt with the package.json
stylus commands (with `-m`):

```
stylus -m styles/styl/master.styl -o dist/css/app.css
stylus -m styles/styl/theme_masters -o dist/css/themes
```

Baseline check first: the unmodified source rebuilt identical to the committed
CSS (ignoring the source-map comment), so all diffs are this change's.

## Verification (cema run locally on :8088, Postgres dev DB)

- `/login`: swapped old vs new `app.css` (both served from one scratch origin
  so font URLs resolve alike) and compared computed styles of every login
  element + `::before` — **no differences**, empty and filled states. Focus
  state couldn't be driven from script (Chrome drops focus); those rules are
  unchanged except for scoping.
- `/sermons` grid: live Cobalt keeps `--chg-accent: #3f6ea5`; loading
  `themes/sanctuary.css` switches it to `#7a2f3a` (override beats the inline
  default); screenshot looked coherent.
- **Not verified:** Summernote Insert Link dialog (needs an admin login).
- Pre-existing, unchanged: Chrome's pre-interaction password autofill leaves
  the label over the dots → raised as N-058.

## Commits

- cema, ccswm: stylus sources + rebuilt `dist/css`. Left alone (not this
  session's): ccswm's modified `.gitignore`; cema's untracked `.cats-todo/`,
  `.ced/`, `cema-linux-amd64`.
- church: this doc + `next-list.md`.

## Next

Closed: N-005. Declined: None. Raised: N-058.
Deferred: None. Promoted: None.
Updated: None. Full list: `ai_docs/todo/next-list.md`.
