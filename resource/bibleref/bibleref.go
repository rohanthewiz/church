// Package bibleref finds Bible verse references in plain text and turns them
// into Blue Letter Bible (BLB) deep links.
//
// The web side already gets hover tooltips from BLB's ScriptTagger (see
// template/page.html.go), which does its own scanning in the browser DOM.
// This package exists for surfaces ScriptTagger can't reach — chiefly the
// mobile JSON API, where the Flutter app needs reference positions to build
// tappable spans — and for any server-side rendering we want to control.
//
// Pipeline:
//
//	raw text ──> refPattern (loose candidate match)
//	                 │
//	                 ▼
//	          normalizeBook ("II Sam." -> "2sam")
//	                 │
//	                 ▼
//	          alias lookup ──> unknown? drop candidate
//	                 │
//	                 ▼
//	              []Ref (canonical book, chapter/verses, byte offsets)
//
// The two-stage design (loose regex, then table validation) keeps the regex
// small and readable: the pattern happily matches "over 3:16" or "job 3
// times", and the alias table is what decides "over" is not a book while
// "Job" is. The alternative — compiling all ~250 book aliases into one giant
// alternation — is faster per scan but much harder to maintain, and our
// inputs (sermon summaries, article bodies) are small enough that it doesn't
// matter.
package bibleref

import (
	"fmt"
	"regexp"
	"strings"
)

// Ref is one resolved scripture reference. Start/End are byte offsets into
// the scanned text so clients (e.g. the Flutter app) can splice in tappable
// spans without re-parsing.
type Ref struct {
	Book       string `json:"book"`       // canonical display name, e.g. "1 Samuel"
	Slug       string `json:"slug"`       // BLB URL segment, e.g. "1sa"
	Chapter    int    `json:"chapter"`    // 1-based
	VerseStart int    `json:"verseStart"` // 0 = whole chapter
	VerseEnd   int    `json:"verseEnd"`   // 0 = single verse (or whole chapter)
	Raw        string `json:"raw"`        // text as matched, e.g. "II Sam. 7:12-16"
	Start      int    `json:"start"`      // byte offset of Raw in the scanned text
	End        int    `json:"end"`
}

// BLBURL returns a Blue Letter Bible deep link for the reference.
// translation is a BLB version code (nkjv, kjv, esv, ...); empty defaults to
// NKJV to match the ScriptTagger config on the web side.
//
// Verified URL shapes (all return 200 as of 2026-07):
//
//	https://www.blueletterbible.org/nkjv/jhn/3/16/     single verse
//	https://www.blueletterbible.org/nkjv/jhn/3/16-18/  verse range
//	https://www.blueletterbible.org/nkjv/jhn/3/        whole chapter
func (r Ref) BLBURL(translation string) string {
	if translation == "" {
		translation = "nkjv"
	}
	base := fmt.Sprintf("https://www.blueletterbible.org/%s/%s/%d/",
		strings.ToLower(translation), r.Slug, r.Chapter)
	if r.VerseStart == 0 {
		return base
	}
	if r.VerseEnd > r.VerseStart {
		return fmt.Sprintf("%s%d-%d/", base, r.VerseStart, r.VerseEnd)
	}
	return fmt.Sprintf("%s%d/", base, r.VerseStart)
}

// String renders the canonical human-readable form, e.g. "1 Samuel 7:12-16".
func (r Ref) String() string {
	s := fmt.Sprintf("%s %d", r.Book, r.Chapter)
	if r.VerseStart > 0 {
		s += fmt.Sprintf(":%d", r.VerseStart)
		if r.VerseEnd > r.VerseStart {
			s += fmt.Sprintf("-%d", r.VerseEnd)
		}
	}
	return s
}

// refPattern matches reference *candidates*; book validation happens after.
// Anatomy (case-insensitive):
//
//	(?:(1|2|3|i{1,3}|1st|2nd|3rd|first|second|third)[\s.]*)?  numbered-book prefix
//	([a-z]+(?:\s+of\s+[a-z]+)?)\.?                            book word(s); "of" arm is for Song of Solomon/Songs
//	\s+(\d{1,3})                                              chapter
//	(?::(\d{1,3})                                             optional :verse
//	(?:\s*[-–—]\s*(\d{1,3}))?)?                               optional -endVerse (ASCII or en/em dash)
//
// Deliberately NOT handled (keep the sketch honest):
//   - cross-chapter ranges ("John 3:16-4:2") — the range tail parses as verses only
//   - comma lists ("John 3:16, 18") — each segment after the comma is dropped
//   - chapter-only false positives ("did the job 3 times" -> Job 3); if these
//     show up in real content, add a RequireVerse option rather than trying to
//     out-clever the regex
var refPattern = regexp.MustCompile(
	`(?i)\b(?:(1|2|3|i{1,3}|1st|2nd|3rd|first|second|third)[\s.]*)?` +
		`([a-z]+(?:\s+of\s+[a-z]+)?)\.?` +
		`\s+(\d{1,3})(?::(\d{1,3})(?:\s*[-–—]\s*(\d{1,3}))?)?`)

// FindAll scans text and returns every resolvable scripture reference in
// document order. Candidates whose book part isn't in the alias table are
// silently dropped — that is the false-positive filter, not an error.
//
// The scan is a manual loop rather than FindAllStringSubmatchIndex because a
// rejected candidate must not consume text that belongs to the next real
// reference. Example: in "and 2 Timothy 1:7" the first candidate is
// book "and" + chapter "2"; with non-overlapping matching that swallows the
// "2", and "Timothy 1:7" alone resolves to nothing (bare "timothy" is not an
// alias — the numbered books only exist with their prefix). So on rejection
// we resume the scan at the candidate's chapter digits, which is exactly
// where a real reference's numbered-book prefix would begin.
func FindAll(text string) (refs []Ref) {
	for pos := 0; pos < len(text); {
		m := refPattern.FindStringSubmatchIndex(text[pos:])
		if m == nil {
			break
		}
		// Shift submatch offsets from the slice back into text coordinates.
		// Pairs: 0 whole, 1 prefix, 2 book, 3 chapter, 4 verse, 5 endVerse
		for i := range m {
			if m[i] >= 0 {
				m[i] += pos
			}
		}
		grp := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return text[m[2*i]:m[2*i+1]]
		}

		bk, ok := lookupBook(grp(1), grp(2))
		if !ok {
			pos = m[6] // chapter-group start: strictly past the match start, so the loop advances
			continue
		}
		pos = m[1]

		ref := Ref{
			Book:       bk.name,
			Slug:       bk.slug,
			Chapter:    atoiSafe(grp(3)),
			VerseStart: atoiSafe(grp(4)),
			VerseEnd:   atoiSafe(grp(5)),
			Raw:        text[m[0]:m[1]],
			Start:      m[0],
			End:        m[1],
		}

		// Single-chapter books (Jude, Philemon, ...) are conventionally cited
		// by verse alone: "Jude 3" means chapter 1 verse 3, not chapter 3.
		if bk.singleChapter && ref.VerseStart == 0 {
			ref.VerseStart = ref.Chapter
			ref.Chapter = 1
		}

		// A backwards range ("John 3:18-16") is a typo; keep the first verse
		// rather than emitting a URL BLB would reject.
		if ref.VerseEnd != 0 && ref.VerseEnd <= ref.VerseStart {
			ref.VerseEnd = 0
		}

		refs = append(refs, ref)
	}
	return refs
}

// lookupBook resolves a matched (prefix, word) pair against the alias table.
// Normalization collapses the many ways people write the same book —
// "II Sam.", "2 Sam", "2Samuel" — onto one key: roman numerals and ordinals
// become digits, periods and spaces vanish, everything lowercases.
func lookupBook(prefix, word string) (bk book, ok bool) {
	key := strings.ToLower(strings.TrimSpace(prefix))
	switch key {
	case "i", "1st", "first":
		key = "1"
	case "ii", "2nd", "second":
		key = "2"
	case "iii", "3rd", "third":
		key = "3"
	}
	word = strings.ToLower(word)
	word = strings.ReplaceAll(word, ".", "")
	word = strings.ReplaceAll(word, " ", "")
	// Multi-space "song  of  songs" collapses via ReplaceAll above since the
	// regex only permits spaces between the words; tabs/newlines won't match
	// the pattern in the first place.
	bk, ok = bookAliases[key+word]
	return bk, ok
}

func atoiSafe(s string) (n int) {
	// The regex guarantees 1-3 digits, so a hand-rolled loop avoids the
	// strconv error path entirely.
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

// book is one canonical Bible book. slug values are BLB's URL codes — all 66
// verified against live BLB URLs (2026-07); note the surprises: Ezekiel is
// "eze" (not "ezk"), Jude is "jde", Philippians is "phl" vs Philemon "phm".
type book struct {
	name          string
	slug          string
	singleChapter bool
	aliases       []string // normalized: lowercase, no spaces/periods, digit prefixes
}

var books = []book{
	{name: "Genesis", slug: "gen", aliases: []string{"genesis", "gen", "ge", "gn"}},
	{name: "Exodus", slug: "exo", aliases: []string{"exodus", "exod", "exo", "ex"}},
	{name: "Leviticus", slug: "lev", aliases: []string{"leviticus", "lev", "le", "lv"}},
	{name: "Numbers", slug: "num", aliases: []string{"numbers", "num", "nu", "nm"}},
	{name: "Deuteronomy", slug: "deu", aliases: []string{"deuteronomy", "deut", "deu", "dt"}},
	{name: "Joshua", slug: "jos", aliases: []string{"joshua", "josh", "jos"}},
	{name: "Judges", slug: "jdg", aliases: []string{"judges", "judg", "jdg", "jgs"}},
	{name: "Ruth", slug: "rth", aliases: []string{"ruth", "rth", "ru"}},
	{name: "1 Samuel", slug: "1sa", aliases: []string{"1samuel", "1sam", "1sa", "1sm"}},
	{name: "2 Samuel", slug: "2sa", aliases: []string{"2samuel", "2sam", "2sa", "2sm"}},
	{name: "1 Kings", slug: "1ki", aliases: []string{"1kings", "1kgs", "1kin", "1ki"}},
	{name: "2 Kings", slug: "2ki", aliases: []string{"2kings", "2kgs", "2kin", "2ki"}},
	{name: "1 Chronicles", slug: "1ch", aliases: []string{"1chronicles", "1chron", "1chr", "1ch"}},
	{name: "2 Chronicles", slug: "2ch", aliases: []string{"2chronicles", "2chron", "2chr", "2ch"}},
	{name: "Ezra", slug: "ezr", aliases: []string{"ezra", "ezr"}},
	{name: "Nehemiah", slug: "neh", aliases: []string{"nehemiah", "neh", "ne"}},
	{name: "Esther", slug: "est", aliases: []string{"esther", "esth", "est"}},
	{name: "Job", slug: "job", aliases: []string{"job", "jb"}},
	{name: "Psalms", slug: "psa", aliases: []string{"psalms", "psalm", "psa", "pss", "ps"}},
	{name: "Proverbs", slug: "pro", aliases: []string{"proverbs", "prov", "pro", "prv", "pr"}},
	{name: "Ecclesiastes", slug: "ecc", aliases: []string{"ecclesiastes", "eccles", "eccl", "ecc", "ec"}},
	{name: "Song of Solomon", slug: "sng", aliases: []string{"songofsolomon", "songofsongs", "song", "sos", "sng", "canticles", "cant"}},
	// "is" (Isaiah), "am" (Amos), and "re" (Revelation) are real SBL-style
	// abbreviations but also common English words followed by numbers ("the
	// ratio is 3:16"), so they are deliberately excluded from the alias lists.
	{name: "Isaiah", slug: "isa", aliases: []string{"isaiah", "isa"}},
	{name: "Jeremiah", slug: "jer", aliases: []string{"jeremiah", "jer", "je"}},
	{name: "Lamentations", slug: "lam", aliases: []string{"lamentations", "lam", "la"}},
	{name: "Ezekiel", slug: "eze", aliases: []string{"ezekiel", "ezek", "eze", "ezk"}},
	{name: "Daniel", slug: "dan", aliases: []string{"daniel", "dan", "dn", "da"}},
	{name: "Hosea", slug: "hos", aliases: []string{"hosea", "hos", "ho"}},
	{name: "Joel", slug: "joe", aliases: []string{"joel", "joe", "jl"}},
	{name: "Amos", slug: "amo", aliases: []string{"amos", "amo"}},
	{name: "Obadiah", slug: "oba", singleChapter: true, aliases: []string{"obadiah", "obad", "oba", "ob"}},
	{name: "Jonah", slug: "jon", aliases: []string{"jonah", "jon", "jnh"}},
	{name: "Micah", slug: "mic", aliases: []string{"micah", "mic", "mc"}},
	{name: "Nahum", slug: "nah", aliases: []string{"nahum", "nah", "na"}},
	{name: "Habakkuk", slug: "hab", aliases: []string{"habakkuk", "hab", "hb"}},
	{name: "Zephaniah", slug: "zep", aliases: []string{"zephaniah", "zeph", "zep", "zp"}},
	{name: "Haggai", slug: "hag", aliases: []string{"haggai", "hag", "hg"}},
	{name: "Zechariah", slug: "zec", aliases: []string{"zechariah", "zech", "zec", "zc"}},
	{name: "Malachi", slug: "mal", aliases: []string{"malachi", "mal", "ml"}},
	{name: "Matthew", slug: "mat", aliases: []string{"matthew", "matt", "mat", "mt"}},
	{name: "Mark", slug: "mar", aliases: []string{"mark", "mrk", "mar", "mk"}},
	{name: "Luke", slug: "luk", aliases: []string{"luke", "luk", "lk"}},
	{name: "John", slug: "jhn", aliases: []string{"john", "jhn", "jn"}},
	{name: "Acts", slug: "act", aliases: []string{"acts", "act", "ac"}},
	{name: "Romans", slug: "rom", aliases: []string{"romans", "rom", "ro", "rm"}},
	{name: "1 Corinthians", slug: "1co", aliases: []string{"1corinthians", "1cor", "1co"}},
	{name: "2 Corinthians", slug: "2co", aliases: []string{"2corinthians", "2cor", "2co"}},
	{name: "Galatians", slug: "gal", aliases: []string{"galatians", "gal", "ga"}},
	{name: "Ephesians", slug: "eph", aliases: []string{"ephesians", "eph"}},
	{name: "Philippians", slug: "phl", aliases: []string{"philippians", "phil", "php", "phl"}},
	{name: "Colossians", slug: "col", aliases: []string{"colossians", "col"}},
	{name: "1 Thessalonians", slug: "1th", aliases: []string{"1thessalonians", "1thess", "1thes", "1th"}},
	{name: "2 Thessalonians", slug: "2th", aliases: []string{"2thessalonians", "2thess", "2thes", "2th"}},
	{name: "1 Timothy", slug: "1ti", aliases: []string{"1timothy", "1tim", "1ti"}},
	{name: "2 Timothy", slug: "2ti", aliases: []string{"2timothy", "2tim", "2ti"}},
	{name: "Titus", slug: "tit", aliases: []string{"titus", "tit"}},
	{name: "Philemon", slug: "phm", singleChapter: true, aliases: []string{"philemon", "philem", "phlm", "phm"}},
	{name: "Hebrews", slug: "heb", aliases: []string{"hebrews", "heb"}},
	{name: "James", slug: "jas", aliases: []string{"james", "jas", "jam", "jms"}},
	{name: "1 Peter", slug: "1pe", aliases: []string{"1peter", "1pet", "1pe", "1pt"}},
	{name: "2 Peter", slug: "2pe", aliases: []string{"2peter", "2pet", "2pe", "2pt"}},
	{name: "1 John", slug: "1jo", aliases: []string{"1john", "1jhn", "1jn", "1jo"}},
	{name: "2 John", slug: "2jo", singleChapter: true, aliases: []string{"2john", "2jhn", "2jn", "2jo"}},
	{name: "3 John", slug: "3jo", singleChapter: true, aliases: []string{"3john", "3jhn", "3jn", "3jo"}},
	{name: "Jude", slug: "jde", singleChapter: true, aliases: []string{"jude", "jud", "jde"}},
	{name: "Revelation", slug: "rev", aliases: []string{"revelation", "revelations", "rev"}},
}

// bookAliases is the flat lookup table built once at init. Keys are the
// normalized alias forms produced by lookupBook.
var bookAliases = func() map[string]book {
	m := make(map[string]book, len(books)*4)
	for _, bk := range books {
		for _, a := range bk.aliases {
			if _, dup := m[a]; dup {
				// A duplicate alias would silently shadow another book —
				// fail loudly at startup instead of mislinking verses.
				panic("bibleref: duplicate book alias " + a)
			}
			m[a] = bk
		}
	}
	return m
}()
