# Session: Blue Letter Bible ScriptTagger + bibleref Parser Sketch

**Session ID**: `fbca1de0-f375-4f48-a6c3-58cbf3a727c4`
**Date**: 2026-07-15
**Branch**: master

## Goal

Integrate blueletterbible.org (BLB) for verse/passage lookup and study across
the web platform and the Flutter mobile app.

## Key Findings (research phase)

- BLB has **no public REST API**. Its integration offering is **ScriptTagger**,
  a free drop-in JS that scans the rendered page for verse references
  (e.g. "Romans 1:16-18", "Jhn 3:16"), links them, and shows a hover tooltip
  with verse text + deep links into BLB study tools (Strong's, interlinear,
  commentaries). Tooltip caps at 7 verses ("More »" link beyond that).
- Script URL (verified 200): `https://www.blueletterbible.org/assets/scripts/blbToolTip/BLB_ScriptTagger-min.js`
  — note the `blbToolTip/` path segment.
- Config (`BLB.Tagger.*`) must be set **after** the script include — the script
  defines the `BLB` global and applies settings at DOM-ready; setting before
  load throws a ReferenceError.
- BLB deep-link URL shapes, all curl-verified 200:
  - single verse: `https://www.blueletterbible.org/nkjv/jhn/3/16/`
  - range: `.../nkjv/jhn/3/16-18/`
  - whole chapter: `.../nkjv/jhn/3/`
- All 66 BLB book slugs curl-verified. Surprises: Ezekiel = `eze` (not `ezk`),
  Jude = `jde`, Philippians = `phl` vs Philemon = `phm`, Ruth = `rth`,
  Song of Solomon = `sng`.
- ScriptTagger is DOM-based, so it does nothing for Flutter native widgets —
  mobile needs server-side ref parsing + deep links (hence `bibleref`).
- If native in-app verse text is wanted later: pair BLB deep links with a text
  API (API.Bible, ESV API, or bible-api.com; NKJV not on the free ones).

## Work Done

### 1. ScriptTagger on public pages (committed `225f0f5`)

`template/page.html.go` — added at end of `<body>` on the `else` branch of the
`page.IsAdmin` check, so the tagger never rewrites references inside the
Summernote editor DOM (would corrupt content on save). Config: NKJV,
`HyperLinks='all'`, `TargetNewWindow=true`.

### 2. `resource/bibleref` package sketch (this commit)

New domain package: finds verse references in plain text, returns structured
refs with BLB deep links. For the mobile JSON API and any server-side
rendering we control.

- `FindAll(text) []Ref` — `Ref` carries canonical book name, BLB slug,
  chapter, verse range, raw matched text, and **byte offsets** (Start/End) so
  the Flutter app can splice tappable spans without re-parsing. JSON-tagged.
- `Ref.BLBURL(translation)` — deep link; empty translation defaults to nkjv
  (matches web-side ScriptTagger config). `Ref.String()` gives canonical form.
- **Two-stage parse design**: small loose regex finds candidates, then a
  ~230-entry alias table validates the book. The table is the false-positive
  filter ("over 3:16" → dropped). Alternative (one giant regex alternation of
  all aliases) rejected as unmaintainable; inputs are small.
- Handles: abbreviations ("Rom", "Jhn"), roman/ordinal prefixes ("II Sam.",
  "1st John"), en/em-dash ranges, "Song of Solomon/Songs".
- **Single-chapter books** (Jude, Philemon, Obadiah, 2/3 John): "Jude 3"
  normalizes to chapter 1 verse 3.
- **Rescan-on-reject**: a rejected candidate resumes the scan at its chapter
  digits, not at match end. Without this, in "and 2 Timothy 1:7" the failed
  candidate "and 2" eats the "2" and the real ref is lost (bare "timothy" is
  not an alias). Dedicated regression test exists.
- Ambiguous English-word abbreviations `is`, `am`, `re` deliberately excluded
  ("the ratio is 3:16" must not become Isaiah 3:16).
- Backwards ranges ("John 3:18-16") keep the first verse, drop the end.
- Duplicate alias in the table panics at init (would silently mislink verses).
- Tests: `bibleref_test.go`, table-driven, all passing. Covers matches,
  non-matches, offsets, URL shapes, translation param.

### Known limitations (documented in code)

- No cross-chapter ranges ("John 3:16-4:2") or comma lists ("John 3:16, 18").
- Chapter-only matching can misfire ("did the job 3 times" → Job 3); if seen
  in real content, add a `RequireVerse` option.

## Next Steps

- Wire `bibleref.FindAll` into the mobile JSON handlers (sermon/article
  summaries) and attach a `refs` array to responses; Flutter builds tappable
  spans from Start/End offsets.
- Per memory: RevokeAllForUser wiring + device testing + audio_service remain
  the mobile-interop priorities.
