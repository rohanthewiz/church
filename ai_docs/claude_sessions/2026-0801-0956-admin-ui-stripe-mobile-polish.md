# Session: Admin UI Overhaul + Stripe Hardening + Mobile Polish

- Session ID: `4fcfed93-4060-4e38-b9ef-c447cad3737a`
- Date: 2026-08-01 09:56
- Repos touched: `~/projs/go/church/church` (2 commits on master: `5025a6f`, `2320f34`),
  `~/projs/go/church/church_mobile` (1 commit on master: `9b76896`).
  cema/ccswm untouched (both still build via go.work).
- Context in: `ai_docs/claude_sessions/2026-0717-2354-site-themes-ccswm-cema.md`

## Goal

Thorough usability/theming/responsiveness pass over core church admin screens
(page edit form and menu management called out as worst), commit; then audit
church_mobile UIs and identify what core can provide mobile; then a Stripe
once-over across both platforms.

## Commit 1 — `5025a6f` Admin UI overhaul (core church)

### Shared admin stylesheet (the theming/responsive foundation)

- Site-side audit finding: `_material_form.styl` (byte-identical in cema/ccswm)
  is hardcoded-color, has ZERO media queries anywhere in either site's stylus,
  `body/#mid min-width: 800px`, and everything from `.form-help` (line 138,
  col-0 dedent bug) onward compiles **globally unscoped** into app.css.
- New `template/admin_css.go`: framework-wide `af-*` vocabulary
  (af-wrap/card/card__title/row/row--3/field/seg/switch/slider/footer/submit/
  btn/btn--danger/btn--primary/inset/help/opt/req/editor/dash*) driven by
  `--af-*` CSS custom properties declared on `.af-scope`. Emitted in
  `template.Page` head only when `page.IsAdmin`; `.af-scope` added to `#mid`.
- Theming contract: sites re-skin admin via e.g.
  `body.theme-sanctuary .af-scope { --af-accent: #7a1f2b; }` — no stylus rebuild.
- Responsive: field grids collapse at 640px; tighter card chrome at 420px.
  Verified via headless Chrome screenshots at true 390px container width
  (NB: desktop Chrome clamps window width to ~500px — use a fixed-width inner
  div to test narrower).

### Page form (rebuilt: page/module_page_form.go + pack/src/module_page_form.js)

- Server side: af-cards — Page Details (title, auto-slug, side-column toggles
  maintaining hidden `available_positions`, Publish + **Make Home Page**
  switches), Modules card with count badge + Add button + empty state, footer
  with Cancel link.
- BUG FIXED: form never rendered `is_home` → every page update silently reset
  the home flag. Also old `count` limiter never incremented.
- JS rewritten vanilla (was jQuery string-builder + jquery.serialize-object):
  module cards with collapse/reorder/delete-confirm, live type+title summary,
  **type-aware fields** via contentBys (SingleId/MultiId → item-ids visible;
  Pagination → limit/offset + list-only switches; Form/other → neither),
  single-main-module enforcement, 16-module cap. Submit builds
  `{"mods":[...]}` explicitly — booleans as booleans, item_ids/limit/offset as
  strings (matches ModuleReceiver exactly). Round-trip + reorder verified in
  headless Chrome (`--dump-dom` + test script writing into a `<pre>`).
- Packed via `go generate ./...` (pack/packer.go → pack/packed/).

### Menu form (rebuilt: resource/menu/module_menu_form.go)

- Same af-card system; responsive `mf-item` grid rows (label/URL/submenu slug +
  reorder/delete tools; single column <760px); slug now readonly (server
  ignores posted slugs); submenu help text; explicit `{"items":[...]}`
  serialization; verified in headless Chrome.

### Other admin forms migrated to af-*

- User form: Role is now a select built from `RoleToString` (incl. SuperAdmin
  99 which the old label omitted); email `type=email`; username readonly on
  update (server only sets at create); password required on create, blank-keeps
  on update with help text; live password-match feedback (was submit alert());
  token-revocation side effects explained (disable/password change).
- Article form: af-card, categories placeholder/hint, published switch in footer.
- Sermon form: **sermon_date now required** (empty date previously reached
  time.Parse, 500'd, and destroyed the whole typed form incl. two Summernote
  bodies); `accept="audio/*"`; background-upload help text; audio-link-override
  coupling explained ("edits ignored unless override on").
- Event form: its `ef-*` CSS generalized INTO the shared stylesheet; markup
  renamed to af-*; only recurrence panel/summary CSS remains event-specific
  (`.ef-recur-panel` now also `af-inset`; JS className updated to match).

### Lists / grid

- grid.go: EditLink/DeleteLink labels now "Edit"/"Delete";
  `EditLinkNamed/DeleteLinkNamed(href, name)` carry the item title →
  delete dialog says `Delete "<name>"?` + aria-labels; all six list modules
  updated to pass titles. Action column headers labeled "Actions".
- Server-paging awareness: wrapper gets `data-server-paged` when Limit>0; JS
  suppresses the client pager (was a permanent "Page 1 of 1" next to the real
  server pager) and labels counts "N rows on this page".
- Sortable th: `scope=col tabindex=0 aria-sort` + JS keydown Enter/Space and
  live aria-sort updates. Phone: 44px tap targets on row action links;
  `.ch-grid-html` max-width `min(32rem, 78vw)`; swal confirm color from
  `--chg-danger` computed style.
- **Sermons admin list had NO Limit** (rendered every sermon) → Limit: 50
  (page/sermon_pages.go). Users list gained explicit Edit action (first-name
  link was the only way in). Sermons wrapper adds `list-wrapper` alongside
  legacy `ch-sermons-list-wrapper`.

### Nav / flash / login / dashboard / import / cleanup

- `/admin/home` was `ctx.WriteString("Hello Administrator!")` → real dashboard:
  `page/admin_home.go` ModuleAdminDashboard (af-dash cards: Pages/Menus/
  Articles/Sermons/Events/Users/Sermon Import/Sermon Cleanup with "+ New"
  quick actions via JS-nav buttons — no nested <a>). Registered in
  modules_registry; availableModuleTypes filter now also excludes "dashboard".
- Admin submenu was MISSING Sermons + Events → added in hardwired
  menu_def.go AND admin/bootstrap.go; bootstrap now refreshes a menu's items
  on startup **only when updated_by == "bootstrap"** (never clobbers human
  edits) so existing installs pick up new nav entries.
- `template/chrome_css.go` (all pages): flash banner now `position:fixed` at
  viewport top (was absolute top:107px desktop magic number), real
  `<button.flash-close>` + `role=status aria-live=polite`, messages
  HTML-escaped in flash.Render (element never escapes; messages embed path
  params). Nav submenus: tap-toggle JS for `li > a[href="#"]` parents adding
  `.submenu-open` (+aria-expanded) — CSS `:hover`-only submenus were
  unreachable on touch; outside-tap closes.
- Login form: now carries a CSRF token (AuthHandlerRWeb verifies; redirect to
  /login on failure), autocomplete username/current-password, autofocus,
  autocapitalize=none. auth_flow_test.go updated (withCSRF helper) + new
  missing-CSRF test.
- Sermon import: was `GET /admin/sermons/import` → ran full legacy-DB bulk
  import, returned raw JSON, **printed PG2 credentials via fmt.Printf**
  (import2.go:30 — now commented out with warning). Now GET renders a
  standalone confirmation card; new `POST /admin/sermons/import`
  (ImportRunRWeb, CSRF-checked) runs it and flashes the count.
- Sermon cleanup: `.sc-year-group` gets `overflow-x:auto` (5-col monospace
  table overflowed phones).

## Commit 2 — `2320f34` Stripe hardening + app-config enrichment (core)

### Web giving form (resource/payment/module_payment_form.go + packed JS)

- Form posted to DEAD route `/payments/create` (commented out in router) —
  JS-off submit 404'd losing input. Now `action="#" onsubmit="return false"`
  (flow is entirely in the JS submit listener) + `<noscript>` notice.
- Labels wired to real ids; name/email/amount `required` with proper
  types/autocomplete/inputmode; comment `maxlength=480`; `#card-errors` moved
  below the amount field (+aria-live). Self-contained `paymentFormCSS`
  (max-width 30rem, full-width inputs/button) overrides the sites' broken
  `width:88vw` stylus rule by cascade order.
- `page.PaymentForm()` had `IsAdmin: true` on the PUBLIC giving page (loaded
  bootstrap.js+summernote for donors, now would also have injected AdminCSS)
  → false.
- Packed JS: `$(document).ready` → vanilla DOMContentLoaded (giving no longer
  dies if CDN jQuery fails).

### Server-side payment fixes (payment_controller, resource/payment)

- Web CreatePaymentIntentRWeb now mirrors the mobile twin: fullname REQUIRED
  (a nameless completed gift charged the card, failed recorder's hard
  name-requirement → no local row/receipt AND webhook 500-looped for ~3 days),
  email "@" sanity, NaN/Inf guard on ParseFloat, `MaxChargeCents` $25k ceiling
  (new const in giving.go + applied to mobile endpoint too), comment cap
  (`MaxCommentLen` 480 < Stripe's 500/value metadata cliff),
  `source: web` metadata, and **rate limiting** via new exported
  `payment.AllowIntent(ip)` (web route previously had NO limit — card-testing
  surface). Limiter now evicts fully-expired IP keys (unbounded map growth).
- **Double-record race closed**: recordPaymentIntent's lookup-then-upsert is
  serialized under a global mutex (webhook + receipt-redirect fire
  simultaneously; charges table has no unique index on payment_token;
  per-intent lock map rejected — lifecycle race on 3rd caller; global mutex is
  correct at donation volume). Receipt email moved to a goroutine (was
  blocking the giver's page AND the webhook ack).
- Receipt page: `redirect_status=failed` now renders an honest "payment not
  completed" page (previously showed "Thanks for your donation!" and could
  fall back to the PREVIOUS gift's session-stashed receipt URL). Success
  receipts show gift amount + date + link home: finalizePayment returns the
  PI; controller passes `payment.ReceiptMeta` JSON via module Meta
  (parseReceiptMeta accepts legacy bare-URL strings).

### app-config enrichment (what core now provides mobile)

- config.EnvConfig gains `Mobile` block (yaml `mobile:`): logo_url,
  theme_colors{primary,secondary,surface}, suggested_amounts_cents,
  apple_merchant_id, google_pay_merchant_id, country_code, currency.
- resource/apiv1/appconfig.go: `theme_colors` resolved builtin-palette-by-
  theme-name (six shared palettes hardcoded: cobalt #9eaac2/#6794c3/#f9f9f9,
  sanctuary #8d545c/#6b2531/#f7f4ed, fellowship, graphite, horizon, willow —
  anchors = nav-bgcolor/accent-bgcolor/page-bgcolor from site stylus) with
  per-field config override, unknown theme → cobalt. Plus `logo_url` and
  `giving` block (defaults: [2500,5000,10000,25000], US, usd). New contract
  tests: TestAppConfigThemeColors, TestAppConfigGivingDefaults.

## Commit 3 — `9b76896` church_mobile UI polish

- **Branding**: MaterialApp ColorScheme.fromSeed seeds from
  `config.themeColors.primary` (accent grafted as `secondary`), light+dark;
  Home app bar shows logo (Image.network resolved against api.baseUri, quiet
  errorBuilder fallback). New model classes ThemeColors (hex→Color parser),
  GivingConfig; AppFeatures now parses chat/prayer_wall flags.
- **Giving screen**: `_applyStripeKey` NEVER reads `Stripe.publishableKey`
  (getter throws when unset — permanently bricked giving after a boot-time
  config failure, misreported as network error); assigns + `applySettings()`
  once per key (static `_appliedStripeKey`); `Stripe.merchantIdentifier` from
  apple_merchant_id. PaymentSheet gets `applePay:`/`googlePay:` params only
  when merchant ids configured (googlePay testEnv when pk_test). Quick chips
  from `config.giving.suggestedAmountsCents` as ChoiceChips with selected
  state + caret-at-end; amount validator caps at $25k mirror. Success dialog
  states amount + "View history" action when signed in. Prefill listens to
  session (tab lives in IndexedStack — mid-session login never reached it);
  listener captured in initState, removed via stored ref (AppScope.of illegal
  in dispose).
- **ApiClient**: 20s `.timeout` on all requests; `_decode` accepts any 2xx
  (204 deletes); `deleteJson` helper added (chat/prayer-wall prerequisite).
- **Refresh honesty**: AsyncView is now stateful — keeps last good data
  rendered under a LinearProgressIndicator while a replacement future runs
  (was: full blank to centered spinner); all five list screens' `_reload`
  return the fetch future so RefreshIndicator spinners track reality.
  History `_refresh` fetches page 0 and swaps on success (no clear-first flash).
- **Paging**: Articles + Events honor `has_more` with a trailing "Load more"
  row (`_extra` accumulation + `_hasMore` override pattern over the AsyncView
  first page — both previously discarded the envelope, truncating archives).
  Sermons scroll-listener gated on `_error == null` (failed page previously
  re-fired on every scroll tick near bottom; tail Retry resumes).
- **HTML bodies**: flutter_html `TagExtension` for `img` resolves
  site-relative src against API base (inline images rendered broken before).
- **Player**: slider tracks drag locally (`_dragMs`), single seek on
  `onChangeEnd` (was a seek storm of HTTP Range requests per tick), value
  label; tooltips on back-10/play-pause/forward-30.
- flutter analyze clean; flutter test all pass.

## Verification done

- Go: `go build ./...` + full `go test ./...` clean in church; cema + ccswm
  build via go.work. auth/payment/apiv1 suites pass incl. new tests.
- New page/menu form JS: node --check + headless-Chrome functional tests
  (add/reorder/serialize → exact server contract shapes confirmed).
- Screenshots of all rebuilt forms + dashboard at 390px and 1200px.
- Flutter: analyze zero issues, tests pass.

## Follow-ups / next steps

- **Apple Pay**: needs `ios/Runner/Runner.entitlements` + merchant ID in the
  Apple dev portal (code path is ready; config-gated).
- **`API_BASE` still defaults to `http://localhost:8000`** — release builds
  need `--dart-define=API_BASE=...` (no cleartext exceptions in native configs).
- Chat + prayer wall Flutter screens: server fully ready (endpoints, SSE,
  flags); `deleteJson`/2xx groundwork done; screens not yet built.
- Recommended next API additions for the app: item image/thumbnail URLs,
  event start/end RFC3339 datetimes (+add-to-calendar), sermon
  duration/size, search endpoint, filter facets (years/teachers/categories).
- Consider a mini-player bar in the shell (SermonAudioController anticipates
  it); no in-app pause once off the sermon screen.
- Consider `UNIQUE INDEX on charges(payment_token)` as a DB-level backstop to
  the recording mutex (works only if bytdb supports unique indexes).
- Web form token remains reusable for 1h (VerifyFormToken not single-use) —
  noted, deliberately not changed (failed-validation resubmits would break).
- Site themes may want `--af-*` / `--chg-*` overrides added to their theme
  stylus files now that the hooks exist (sites untouched this session).
- Old material-form classes (`wrapper-material-form` etc.) now used only by
  the login form; the site stylus partials could eventually slim down.
