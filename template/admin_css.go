package template

// AdminCSS is the framework-wide stylesheet for the admin screens (forms,
// lists, dashboard). It is inlined into <head> for admin pages only (see
// Page()), the same zero-site-rebuild delivery used by grid.CSS and
// ResponsiveCSS: each church site compiles its own app.css, so shipping the
// admin look with the framework is the only way to evolve admin UI without
// coordinating a stylus rebuild in every site.
//
// Theming contract: every color/metric that a site could reasonably want to
// brand is a CSS custom property, declared with a default on .af-scope
// (the admin content root emitted by Page()). A site theme overrides them
// from its own stylesheet without touching markup, e.g.:
//
//	body.theme-sanctuary .af-scope { --af-accent: #7a1f2b; --af-accent-hover: #5d1721; }
//
// Anything not exposed as a variable is deliberately fixed so admin pages
// stay readable regardless of how adventurous a public theme is.
//
// Naming: af- ("admin form/frame") — generalized from the event form's
// former ef- classes so all admin modules share one vocabulary:
//
//	.af-scope                    admin root (declares the variables)
//	.af-wrap                     centered column that holds a form/screen
//	.af-card / .af-card__title   white section card with small-caps heading
//	.af-row / .af-row--3         responsive field grids (collapse < 640px)
//	.af-field                    label-above-input block
//	.af-seg                      segmented radio control
//	.af-switch                   checkbox drawn as an on/off switch
//	.af-footer / .af-submit      form footer bar with primary action
//	.af-btn / --danger / --ghost small action buttons (module/menu-item rows)
//	.af-dash*                    admin home dashboard cards
//
// Layout down to phones: field grids collapse to one column at 640px; the
// 420px block tightens paddings so cards remain comfortable at a 6.5"
// (~390-430px CSS px) phone width. Horizontal overflow is prevented at the
// wrapper (min-width:0 everywhere flex/grid could infer content width).
const AdminCSS = `
.af-scope {
	--af-accent: #337ab7;
	--af-accent-hover: #286090;
	--af-accent-soft: rgba(51,122,183,0.15);
	--af-danger: #c0392b;
	--af-danger-hover: #96281b;
	--af-ok: #7cb342;
	--af-text: #2d3436;
	--af-text-soft: #57606a;
	--af-text-faint: #98a1a8;
	--af-card-bg: #ffffff;
	--af-card-border: #dfe6e0;
	--af-inset-bg: #f4f8f4;
	--af-inset-border: #e0e9e0;
	--af-input-bg: #fbfdfb;
	--af-input-border: #c9d3cc;
	--af-radius: 8px;
}
.af-wrap { max-width: 56rem; margin: 0.5rem auto 2rem; color: var(--af-text);
	font-size: 1rem; min-width: 0; }
.af-wrap--wide { max-width: 72rem; }
.af-page-title { text-align: center; text-transform: capitalize;
	margin: 0.6rem 0 1rem; color: var(--af-text); }
.af-card { background: var(--af-card-bg); border: 1px solid var(--af-card-border);
	border-radius: var(--af-radius); padding: 1rem 1.2rem 1.2rem;
	margin-bottom: 1.1rem; box-shadow: 0 1px 3px rgba(0,0,0,0.06); min-width: 0; }
.af-card__title { font-size: 0.8rem; font-weight: 600; letter-spacing: 0.08em;
	text-transform: uppercase; color: #6b7c74; margin-bottom: 0.9rem;
	border-bottom: 1px solid #eef2ee; padding-bottom: 0.45rem;
	display: flex; align-items: center; justify-content: space-between; gap: 0.6rem; }
.af-row { display: grid; grid-template-columns: 1fr 1fr; gap: 0.9rem 1.4rem;
	min-width: 0; }
.af-row--3 { grid-template-columns: 1fr 1fr 1fr; }
.af-field { display: flex; flex-direction: column; min-width: 0; }
.af-field label { font-size: 0.82rem; font-weight: 600; color: var(--af-text-soft);
	margin-bottom: 0.28rem; }
.af-field .af-opt { font-weight: 400; color: var(--af-text-faint); }
.af-req { color: #d9534f; }
.af-field input, .af-field select, .af-field textarea {
	font-size: 0.95rem; color: var(--af-text); background: var(--af-input-bg);
	border: 1px solid var(--af-input-border); border-radius: 5px;
	padding: 0.42rem 0.55rem; line-height: 1.4; width: 100%; box-shadow: none;
	transition: border-color 0.2s ease, box-shadow 0.2s ease; min-width: 0; }
.af-field input:focus, .af-field select:focus, .af-field textarea:focus {
	outline: none; border-color: var(--af-accent);
	box-shadow: 0 0 0 3px var(--af-accent-soft); }
.af-field input:disabled, .af-field select:disabled {
	background: #eef1ee; color: var(--af-text-faint); cursor: not-allowed; }
.af-help { font-size: 0.8rem; color: var(--af-text-faint); margin: 0.45rem 0 0; }
/* Segmented control: real radios stay in the form; labels draw the control */
.af-seg { display: inline-flex; border: 1px solid var(--af-input-border);
	border-radius: 6px; overflow: hidden; margin: 0.2rem 0 0.4rem;
	max-width: 100%; }
.af-seg input[type="radio"] { position: absolute; opacity: 0; width: 0; height: 0; }
.af-seg label { padding: 0.4rem 1.15rem; font-size: 0.9rem; cursor: pointer;
	background: var(--af-input-bg); color: var(--af-text-soft);
	border-left: 1px solid var(--af-input-border); margin: 0;
	transition: background 0.15s ease, color 0.15s ease; white-space: nowrap; }
.af-seg label:first-of-type { border-left: none; }
.af-seg input:checked + label { background: var(--af-accent); color: #fff; }
.af-seg input:focus-visible + label {
	box-shadow: inset 0 0 0 2px var(--af-accent-soft); }
/* Inset panel (e.g. recurrence details) */
.af-inset { background: var(--af-inset-bg); border: 1px solid var(--af-inset-border);
	border-radius: 6px; padding: 0.8rem 0.9rem; margin-top: 0.6rem; }
/* Publish toggle: a plain checkbox (still posts "on") drawn as a switch */
.af-switch { display: inline-flex; align-items: center; cursor: pointer; gap: 0.6rem; }
.af-switch input { position: absolute; opacity: 0; width: 0; height: 0; }
.af-switch .af-slider { width: 2.4rem; height: 1.3rem; background: var(--af-input-border);
	border-radius: 1rem; position: relative; transition: background 0.2s ease;
	flex: none; }
.af-switch .af-slider::before { content: ''; position: absolute; top: 0.15rem;
	left: 0.15rem; width: 1rem; height: 1rem; background: #fff; border-radius: 50%;
	transition: transform 0.2s ease; box-shadow: 0 1px 2px rgba(0,0,0,0.25); }
.af-switch input:checked + .af-slider { background: var(--af-ok); }
.af-switch input:checked + .af-slider::before { transform: translateX(1.1rem); }
.af-switch input:focus-visible + .af-slider {
	box-shadow: 0 0 0 3px var(--af-accent-soft); }
.af-switch .af-switch-text { font-size: 0.95rem; font-weight: 600;
	color: var(--af-text-soft); }
.af-footer { display: flex; align-items: center; justify-content: space-between;
	padding: 0.4rem 0.2rem; gap: 0.8rem; flex-wrap: wrap; }
.af-submit { background: var(--af-accent); color: #fff;
	border: 1px solid var(--af-accent-hover); border-radius: 6px; font-size: 1rem;
	padding: 0.5rem 2.6rem; cursor: pointer;
	transition: background 0.2s ease, box-shadow 0.2s ease;
	box-shadow: 0 2px 4px rgba(0,0,0,0.15); }
.af-submit:hover { background: var(--af-accent-hover);
	box-shadow: 0 3px 8px rgba(0,0,0,0.2); }
/* Small action buttons (reorder/remove rows, add module, ...) */
.af-btn { display: inline-flex; align-items: center; justify-content: center;
	gap: 0.3rem; background: var(--af-input-bg); color: var(--af-text-soft);
	border: 1px solid var(--af-input-border); border-radius: 5px;
	font-size: 0.85rem; padding: 0.3rem 0.7rem; cursor: pointer; line-height: 1.2;
	transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease; }
.af-btn:hover { border-color: var(--af-accent); color: var(--af-accent); }
.af-btn--primary { background: var(--af-accent); border-color: var(--af-accent-hover);
	color: #fff; }
.af-btn--primary:hover { background: var(--af-accent-hover); color: #fff; }
.af-btn--danger:hover { border-color: var(--af-danger); color: var(--af-danger); }
.af-btn:disabled { opacity: 0.45; cursor: not-allowed; }
/* Editors (Summernote lives in its scoped-bootstrap island); label spacing only */
.af-editor { margin-bottom: 1rem; }
.af-editor label { display: block; font-size: 0.82rem; font-weight: 600;
	color: var(--af-text-soft); margin-bottom: 0.28rem; }
/* Admin home dashboard */
.af-dash { display: grid; grid-template-columns: repeat(auto-fill, minmax(13rem, 1fr));
	gap: 0.9rem; margin-top: 1rem; }
.af-dash a.af-dash__card { display: block; background: var(--af-card-bg);
	border: 1px solid var(--af-card-border); border-radius: var(--af-radius);
	padding: 1rem 1.1rem; text-decoration: none; color: var(--af-text);
	box-shadow: 0 1px 3px rgba(0,0,0,0.06);
	transition: box-shadow 0.15s ease, border-color 0.15s ease, transform 0.15s ease; }
.af-dash a.af-dash__card:hover { border-color: var(--af-accent);
	box-shadow: 0 3px 10px rgba(0,0,0,0.12); transform: translateY(-1px); }
.af-dash__title { font-weight: 600; font-size: 1.02rem; margin-bottom: 0.25rem;
	color: var(--af-accent); }
.af-dash__desc { font-size: 0.85rem; color: var(--af-text-faint); line-height: 1.35; }
.af-dash__actions { margin-top: 0.55rem; display: flex; gap: 0.5rem; flex-wrap: wrap; }

/* Phones: single-column field grids first, then tighter card chrome */
@media (max-width: 640px) {
	.af-row, .af-row--3 { grid-template-columns: 1fr; }
	.af-footer { justify-content: stretch; }
	.af-submit { flex: 1; }
}
@media (max-width: 420px) {
	.af-wrap { margin: 0.3rem auto 1.2rem; }
	.af-card { padding: 0.75rem 0.7rem 0.9rem; }
	.af-seg label { padding: 0.4rem 0.8rem; }
}
`
