package bibleref

import "testing"

func TestFindAllSingleRefs(t *testing.T) {
	cases := []struct {
		in   string
		want string // canonical String() of the single expected ref; "" = expect no match
		url  string // expected BLBURL("") when want != ""
	}{
		{"For God so loved the world (John 3:16)", "John 3:16",
			"https://www.blueletterbible.org/nkjv/jhn/3/16/"},
		{"see Rom 1:16-18 for Paul's thesis", "Romans 1:16-18",
			"https://www.blueletterbible.org/nkjv/rom/1/16-18/"},
		{"an en-dash range: John 3:16–18 works too", "John 3:16-18",
			"https://www.blueletterbible.org/nkjv/jhn/3/16-18/"},
		{"the covenant in II Sam. 7:12", "2 Samuel 7:12",
			"https://www.blueletterbible.org/nkjv/2sa/7/12/"},
		{"love is patient, 1 Corinthians 13:4", "1 Corinthians 13:4",
			"https://www.blueletterbible.org/nkjv/1co/13/4/"},
		{"1st John 4:8 says God is love", "1 John 4:8",
			"https://www.blueletterbible.org/nkjv/1jo/4/8/"},
		{"Song of Solomon 2:1, the rose of Sharon", "Song of Solomon 2:1",
			"https://www.blueletterbible.org/nkjv/sng/2/1/"},
		// Chapter-only ref links to the whole chapter.
		{"read Psalm 23 tonight", "Psalms 23",
			"https://www.blueletterbible.org/nkjv/psa/23/"},
		// Single-chapter book: "Jude 3" means chapter 1, verse 3.
		{"contend for the faith, Jude 3", "Jude 1:3",
			"https://www.blueletterbible.org/nkjv/jde/1/3/"},
		// Backwards range is a typo; keep the first verse only.
		{"typo range John 3:18-16 here", "John 3:18",
			"https://www.blueletterbible.org/nkjv/jhn/3/18/"},
		// Non-references must not match.
		{"the ratio is 3:16 overall", "", ""},
		{"we won 21:7 at halftime", "", ""},
		{"meet in room 4:15 pm", "", ""},
	}

	for _, c := range cases {
		refs := FindAll(c.in)
		if c.want == "" {
			if len(refs) != 0 {
				t.Errorf("FindAll(%q) = %v; want none", c.in, refs)
			}
			continue
		}
		if len(refs) != 1 {
			t.Errorf("FindAll(%q) returned %d refs; want 1", c.in, len(refs))
			continue
		}
		if got := refs[0].String(); got != c.want {
			t.Errorf("FindAll(%q) = %q; want %q", c.in, got, c.want)
		}
		if got := refs[0].BLBURL(""); got != c.url {
			t.Errorf("BLBURL for %q = %q; want %q", c.in, got, c.url)
		}
	}
}

// A rejected candidate ("and 2") must not swallow the "2" that begins the
// real reference "2 Timothy 1:7" — this is the rescan-at-chapter-digits case.
func TestFindAllRejectedCandidateDoesNotEatNextRef(t *testing.T) {
	refs := FindAll("Luke 1:1-4 and 2 Timothy 1:7 were read")
	if len(refs) != 2 {
		t.Fatalf("got %d refs (%v); want 2", len(refs), refs)
	}
	if refs[0].String() != "Luke 1:1-4" {
		t.Errorf("first ref = %q; want Luke 1:1-4", refs[0].String())
	}
	if refs[1].String() != "2 Timothy 1:7" {
		t.Errorf("second ref = %q; want 2 Timothy 1:7", refs[1].String())
	}
}

// Offsets must point at the raw matched text so the mobile client can build
// tappable spans by splicing on Start/End.
func TestFindAllOffsets(t *testing.T) {
	text := "Compare Gen 1:1 with Jhn 1:1 for the parallel."
	refs := FindAll(text)
	if len(refs) != 2 {
		t.Fatalf("got %d refs (%v); want 2", len(refs), refs)
	}
	for _, r := range refs {
		if text[r.Start:r.End] != r.Raw {
			t.Errorf("offsets [%d:%d] give %q; Raw is %q",
				r.Start, r.End, text[r.Start:r.End], r.Raw)
		}
	}
	if refs[0].Slug != "gen" || refs[1].Slug != "jhn" {
		t.Errorf("slugs = %s, %s; want gen, jhn", refs[0].Slug, refs[1].Slug)
	}
}

func TestBLBURLTranslationParam(t *testing.T) {
	r := Ref{Book: "John", Slug: "jhn", Chapter: 3, VerseStart: 16}
	if got := r.BLBURL("ESV"); got != "https://www.blueletterbible.org/esv/jhn/3/16/" {
		t.Errorf("BLBURL(ESV) = %q", got)
	}
}
