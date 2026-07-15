package chat

import (
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/rohanthewiz/logger"
)

// Rule-based moderation for chat posts. Deliberately simple and transparent:
// every rule is a plain check a church admin could explain to a member.
// Pipeline (applied in Moderate, in this order):
//
//	trim/normalize ─► length gate ─► rate limit ─► duplicate gate
//	      ─► banned-words gate ─► link cap ─► shout softening ─► accepted
//
// Rejections return a member-safe reason string; the caller answers 422 with
// it. Rules that can be fixed silently (all-caps shouting, whitespace runs)
// mutate the message instead of rejecting — reject only what we can't repair.

const (
	// MaxMessageLen bounds a single chat message. Long-form content belongs
	// in an article or prayer request, not a live chat line.
	MaxMessageLen = 1000

	// maxLinks caps URLs per message — the classic spam signature. Two
	// allows legitimate "here's the passage and the songsheet" posts.
	maxLinks = 2

	// Rate limit: a sliding window per user. Generous for real typing,
	// hopeless for a flooding script.
	rateWindow  = 30 * time.Second
	rateMaxMsgs = 8

	// duplicateWindow rejects reposting the identical message within this
	// span (double-click sends, copy-paste spam).
	duplicateWindow = 2 * time.Minute
)

// bannedWords is the default deny list, matched case-insensitively on word
// boundaries. Intentionally mild/starter — profanity lists are a losing
// arms race, so sites extend it at startup via AddBannedWords (e.g. from
// their own config) rather than us shipping an exhaustive list.
var bannedWords = []string{
	"damn", "hell no", "shit", "fuck", "bitch", "asshole", "bastard",
	"nigger", "faggot", "cunt", "whore",
}

var bannedRe *regexp.Regexp
var bannedMu sync.Mutex // guards rebuilds via AddBannedWords at startup

func init() {
	rebuildBannedRe()
}

// rebuildBannedRe compiles the deny list into one alternation with word
// boundaries so "hello" never trips on "hell". Called under bannedMu except
// from init.
func rebuildBannedRe() {
	escaped := make([]string, 0, len(bannedWords))
	for _, w := range bannedWords {
		escaped = append(escaped, regexp.QuoteMeta(strings.ToLower(w)))
	}
	bannedRe = regexp.MustCompile(`(?i)\b(` + strings.Join(escaped, "|") + `)\b`)
}

// AddBannedWords extends the deny list — intended for site bootstrap
// (before traffic), which is why a simple mutex-and-rebuild suffices.
func AddBannedWords(words ...string) {
	bannedMu.Lock()
	defer bannedMu.Unlock()
	bannedWords = append(bannedWords, words...)
	rebuildBannedRe()
}

// ContainsBannedWord exposes the deny-list check to sibling resources (the
// prayer wall filters submissions through the same list, so site language
// policy lives in exactly one place).
func ContainsBannedWord(s string) bool {
	return bannedRe.MatchString(s)
}

var linkRe = regexp.MustCompile(`(?i)\bhttps?://|\bwww\.`)

// collapseNewlines keeps paragraph breaks but flattens blank-line walls that
// let one message shove everyone else's off screen.
var collapseNewlines = regexp.MustCompile(`\n{3,}`)

// ---------------------------------------------------------------------------
// Per-user rate limiting — same in-process sliding-window approach as the
// login limiter in resource/apitoken (at church scale a distributed limiter
// is overkill; worst case in a multi-instance deploy is a proportionally
// higher, still tiny, budget).
// ---------------------------------------------------------------------------

type postLimiter struct {
	mu    sync.Mutex
	posts map[int64][]time.Time // recent post times per user id
	last  map[int64]lastPost    // last accepted body per user (duplicate gate)
}

type lastPost struct {
	body string
	at   time.Time
}

var limiter = &postLimiter{posts: map[int64][]time.Time{}, last: map[int64]lastPost{}}

// allow records-and-checks one attempt: prunes the window, then answers
// whether this post may proceed. Recording on success only (not on
// rejections) would let a flooder probe for free, so the attempt itself
// counts.
func (l *postLimiter) allow(userId int64, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	kept := l.posts[userId][:0]
	for _, t := range l.posts[userId] {
		if now.Sub(t) < rateWindow {
			kept = append(kept, t)
		}
	}
	if len(kept) >= rateMaxMsgs {
		l.posts[userId] = kept
		return false
	}
	l.posts[userId] = append(kept, now)
	return true
}

// isDuplicate reports whether body identically repeats the user's last
// accepted post within duplicateWindow, and records body as the new last
// post when it is not.
func (l *postLimiter) isDuplicate(userId int64, body string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	lp, ok := l.last[userId]
	if ok && lp.body == body && now.Sub(lp.at) < duplicateWindow {
		return true
	}
	l.last[userId] = lastPost{body: body, at: now}
	return false
}

// Moderate runs the full rule pipeline. On acceptance it returns the
// (possibly cleaned) message body; on rejection, a member-safe reason.
// userId keys the rate/duplicate state; username is only for the audit log.
func Moderate(userId int64, username, body string) (cleaned string, reason string) {
	now := time.Now()

	cleaned = strings.TrimSpace(body)
	cleaned = collapseNewlines.ReplaceAllString(cleaned, "\n\n")
	if cleaned == "" {
		return "", "Message is empty"
	}
	if len(cleaned) > MaxMessageLen {
		return "", "Message is too long (1000 characters max)"
	}

	if !limiter.allow(userId, now) {
		return "", "You are posting too quickly — please wait a moment"
	}
	if limiter.isDuplicate(userId, cleaned, now) {
		return "", "Duplicate of your last message"
	}

	if loc := bannedRe.FindString(cleaned); loc != "" {
		// Log the hit (not the whole message) so editors can spot repeat
		// offenders without the log itself becoming a profanity archive.
		logger.Info("Chat message rejected by word filter", "username", username, "matched", strings.ToLower(loc))
		return "", "Message contains language that isn't allowed here"
	}

	if len(linkRe.FindAllStringIndex(cleaned, -1)) > maxLinks {
		return "", "Too many links in one message"
	}

	cleaned = softenShouting(cleaned)
	return cleaned, ""
}

// softenShouting lowercases messages that are essentially all caps — a
// repairable offense, so we fix rather than reject. Short messages ("AMEN!")
// are left alone; sustained caps beyond that reads as shouting.
func softenShouting(s string) string {
	letters, uppers := 0, 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				uppers++
			}
		}
	}
	if letters > 12 && float64(uppers)/float64(letters) > 0.8 {
		return strings.ToLower(s)
	}
	return s
}
