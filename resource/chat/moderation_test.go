package chat

// Unit tests for the rule-based moderation pipeline. Each test posts as a
// distinct user id — the rate/duplicate limiter is package-global state
// keyed by user, so ids must not collide across tests (same-package tests
// run sequentially, so no locking concerns).

import (
	"strings"
	"testing"
)

func TestModerateAcceptsPlainMessage(t *testing.T) {
	cleaned, reason := Moderate(1001, "kim", "  Praying for the Johnson family tonight  ")
	if reason != "" {
		t.Fatalf("unexpected rejection: %q", reason)
	}
	if cleaned != "Praying for the Johnson family tonight" {
		t.Errorf("should trim whitespace, got %q", cleaned)
	}
}

func TestModerateRejectsEmpty(t *testing.T) {
	if _, reason := Moderate(1002, "kim", "   \n  "); reason == "" {
		t.Error("whitespace-only message should be rejected")
	}
}

func TestModerateRejectsTooLong(t *testing.T) {
	if _, reason := Moderate(1003, "kim", strings.Repeat("a", MaxMessageLen+1)); reason == "" {
		t.Error("over-length message should be rejected")
	}
}

func TestModerateRejectsBannedWords(t *testing.T) {
	if _, reason := Moderate(1004, "kim", "that was a shit sermon"); reason == "" {
		t.Error("banned word should be rejected")
	}
	// Word boundaries: innocent containments must pass ("hello" vs "hell no")
	if _, reason := Moderate(1005, "kim", "hello everyone, welcome!"); reason != "" {
		t.Errorf("word-boundary false positive: %q", reason)
	}
}

func TestModerateRejectsLinkSpam(t *testing.T) {
	msg := "check http://a.com http://b.com http://c.com"
	if _, reason := Moderate(1006, "kim", msg); reason == "" {
		t.Error("more than two links should be rejected")
	}
	if _, reason := Moderate(1007, "kim", "song sheet at http://a.com and passage http://b.com"); reason != "" {
		t.Errorf("two links should pass, got %q", reason)
	}
}

func TestModerateSoftensShouting(t *testing.T) {
	cleaned, reason := Moderate(1008, "kim", "PLEASE EVERYONE LOOK AT THIS RIGHT NOW")
	if reason != "" {
		t.Fatalf("shouting should be repaired, not rejected: %q", reason)
	}
	if cleaned != strings.ToLower(cleaned) {
		t.Errorf("sustained caps should be lowercased, got %q", cleaned)
	}
	// Short exclamations stay untouched
	cleaned, _ = Moderate(1008, "kim", "AMEN!")
	if cleaned != "AMEN!" {
		t.Errorf("short message should keep its case, got %q", cleaned)
	}
}

func TestModerateRateLimit(t *testing.T) {
	const uid = 1009
	var rejected bool
	for i := range rateMaxMsgs + 2 {
		// Vary the body so the duplicate gate doesn't trip first
		_, reason := Moderate(uid, "kim", "message variant "+strings.Repeat("x", i+1))
		if reason != "" {
			rejected = true
			break
		}
	}
	if !rejected {
		t.Errorf("flood of %d messages should trip the rate limit", rateMaxMsgs+2)
	}
}

func TestModerateDuplicateGate(t *testing.T) {
	const uid = 1010
	if _, reason := Moderate(uid, "kim", "please pray for my exam"); reason != "" {
		t.Fatalf("first post should pass: %q", reason)
	}
	if _, reason := Moderate(uid, "kim", "please pray for my exam"); reason == "" {
		t.Error("identical repost should be rejected as duplicate")
	}
}

func TestValidChannel(t *testing.T) {
	for _, good := range []string{"community", "prayer-wall", "article-42"} {
		if !ValidChannel(good) {
			t.Errorf("%q should be a valid channel", good)
		}
	}
	for _, bad := range []string{"", "-lead", "Has Space", "UPPER", "x/../y", strings.Repeat("a", 80)} {
		if ValidChannel(bad) {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestCanModerate(t *testing.T) {
	cases := map[int]bool{
		99: true,  // SuperAdmin (the ordering exception)
		1:  true,  // Admin
		5:  true,  // Publisher
		7:  true,  // Author/Editor
		9:  false, // RegisteredUser
		0:  false, // no role loaded
	}
	for role, want := range cases {
		if got := CanModerate(role); got != want {
			t.Errorf("CanModerate(%d) = %v, want %v", role, got, want)
		}
	}
}
