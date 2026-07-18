# Session: Multi-Theme System + Preview for ccswm and cema

- Session ID: `c95652cb-3bcd-474d-b69c-f64fc8e521fa`
- Date: 2026-07-17 23:54
- Repos touched: `~/projs/go/church/ccswm` (uncommitted, on master),
  `~/projs/go/church/cema` (committed on branch `feature/site-themes`, `eed84f6`)
- Framework repo (`church/church`) untouched this session.

## Goal

Create several tasteful themes (church / social / professional / tech / modern)
for the ccswm site with a way to preview them; then port the same to cema on a
feature branch.

## The theming system (same shape in both sites)

Stylus partials under `styles/styl/_styl/` now reference only generic `theme-*`
variables. Each theme is one file defining the full variable set (~30 vars):

```
styles/styl/
├── master.styl              # entry -> dist/css/app.css (LIVE theme = cobalt)
├── base.styl                # shared skeleton: partial requires + html/body
├── theme_masters/<name>.styl  # per-theme master -> dist/css/themes/<name>.css
└── _styl/themes/<name>.styl   # the variable sets (cobalt.styl documents the contract)
```

Require order matters: `_globals.styl` (base `color-*`) → theme file →
`base.styl` (body block invokes partial mixins which resolve `theme-*` lazily).

Key refactor moves:
- `theme-cobalt-*` block in `_globals.styl` commented out; values moved to
  `_styl/themes/cobalt.styl` under generic names (live output preserved).
- Theme-class selector gates `.theme-cobalt` → `[class*="theme-"]` (same
  specificity) so one compiled stylesheet themes the site regardless of the
  `theme:` name in cfg/options.yml. Body class comes from
  `church/template/page.html.go` (`theme-<config.Options.Theme>`).
- Previously hardcoded colors wired to vars: body bg `#f9f9f9` →
  `theme-page-bgcolor`; `#main` col bg → `theme-main-bgcolor`; scripture
  highlight `#a0cb7e` → `theme-scripture-bgcolor`; banner gradient(s) →
  `theme-banner-bgnd`; `menuitem-active` greenyellow underline →
  `theme-nav-active-color`; module-table key-column underline `#bcebc9` →
  `theme-table-accent-color`; event badge/panel → `theme-accent-bgcolor`,
  `theme-accent-color`, `theme-accent-color-muted`, `theme-panel-bgcolor`.
- npm: new `build-css-task:themes-compile` ⇒ `stylus styles/styl/theme_masters
  -o dist/css/themes -m` (runs as part of `npm run build-css`).

## Themes (6 per site)

cobalt (original, stays live), sanctuary (cream/burgundy/gold),
fellowship (teal/coral/warm white), graphite (charcoal/slate/blue),
horizon (navy/cyan), willow (sage/warm paper). The 5 new palettes are pure hex
and shared verbatim between sites.

## Preview

- `dist/theme_preview.html` — toolbar (theme buttons w/ color chips) over an
  iframe; hot-swaps the framed page's `app.css` link to
  `/assets/css/themes/<name>.css`; selection persists via localStorage and
  re-applies across in-frame navigation. Frame targets: static sample page
  (default) or live site `/`.
- `dist/theme_sample.html` — static page with the framework's real markup
  skeleton (banner, menus, event single, sermon table, article, footer);
  previews with no app/DB.
- `scripts/preview_themes.sh [port]` — serves a temp dir with `assets -> dist`
  symlink via `python3 -m http.server` so the absolute `/assets/...` URLs work
  standalone: `http://localhost:8899/assets/theme_preview.html`.
  (Framework serves dist at `/assets` via `s.StaticFiles("/assets/", "dist", 1)`
  in `church/router_rweb.go`.)
- `ai_docs/theming.md` in each site documents structure/build/adoption.

## cema-specific notes (branch `feature/site-themes`)

- cema's cobalt differs from ccswm's: banner `linear-gradient(180deg, #B8D3F2,
  #3D5E85)`, nav `color-blue`, green module-heading gradient, hover gradient.
- Two extra vars only in cema's variable set: `theme-banner-ext-bgnd` (strip at
  banner bottom; was a vendor-prefixed gradient) and `theme-nav-hilite-bgnd`
  (hovered menu-item wash; was `theme-cobalt-menuitem-hilite-background`).
  Ancient `-moz/-webkit/-ms` gradient fallbacks commented out.
- Ported `modules/event_single.styl` from ccswm (cema had none — closes the
  follow-up from the 2026-07-17 event-page session) + `event_single()` call in
  `_module.styl`; added missing `color-gray-blue = #9eaac2` to cema globals.
- cema working tree left ON the feature branch.

## Verification done

- ccswm: recompiled app.css diffed byte-identical to pre-refactor except the
  `[class*="theme-"]` gates; `dist/css/themes/cobalt.css` == app.css.
- cema: same, plus expected additions (event_single CSS) and removed vendor
  prefixes.
- Headless Chrome (`--headless --screenshot --virtual-time-budget=8000+`)
  screenshots of the sample page per theme, eyeballed. Note: without
  virtual-time budget, Google-font FOIT hides the banner title; an occasional
  render shows fallback body-font metrics (cosmetic artifact only — theme CSS
  proven structurally identical via color-normalized diff).
- In-session sandbox blocks `python3 -m http.server` when serving from
  `/var/folders` mktemp dirs (silent kill); works with TMPDIR in scratchpad and
  in normal terminals.

## Follow-ups / next steps

- ccswm changes are UNCOMMITTED (user hasn't asked; cema was committed on its
  feature branch). Commit ccswm similarly when approved.
- Eyeball themes against the real running sites (only static sample verified).
- Consider porting the ccswm theme_preview chip colors if cema cobalt chip
  needs tweaking (currently `#3d5e85`/`#05668d`).
- Merge `feature/site-themes` into cema master after review; then decide
  whether any site actually switches its live theme (swap one `@require` in
  `master.styl`).
