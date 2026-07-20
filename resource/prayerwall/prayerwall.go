// Package prayerwall implements a Prayer Wall: members post prayer requests,
// the congregation sees them, and editors can mark them answered ("praise
// report") or remove them. The wall's page module embeds a live chat
// discussion strip (resource/chat) beneath the requests, which is the
// motivating example of chat's embeddable ModuleTypeChatDiscussion.
//
// Prayer requests are durable content — unlike chat messages they never
// expire; they leave only by editor removal or the requester withdrawing
// their own.
//
// Data access is hand-written SQL over db.Executor (no SQLBoiler model) —
// prayer_requests postdates the generated models; same precedent as
// api_tokens and chat_messages.
package prayerwall

import (
	"database/sql"
	"strings"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
)

// Bounds for a submitted request. The title is a short subject line; the
// body carries the request itself. Long-form testimony belongs in an
// article, so the body cap is modest.
const (
	MaxTitleLen = 120
	MaxBodyLen  = 2000
)

// Request is the prayer wall's row type.
type Request struct {
	Id           int64
	UserId       int64
	Username     string
	DisplayName  string
	Title        string
	Body         string
	Answered     bool
	AnsweredNote string
	Published    bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Validate normalizes a member submission and returns a member-safe reason
// when it cannot be accepted. (Rule-based, mirroring chat's philosophy of
// transparent checks; the heavier chat rules — rate limiting, word filter —
// are not duplicated here because submissions flow through chat.Moderate at
// the handler layer.)
func Validate(title, body string) (cleanTitle, cleanBody, reason string) {
	cleanTitle = strings.TrimSpace(title)
	cleanBody = strings.TrimSpace(body)
	if cleanTitle == "" {
		return "", "", "Please give your request a short title"
	}
	if cleanBody == "" {
		return "", "", "Please write your prayer request"
	}
	if len(cleanTitle) > MaxTitleLen {
		return "", "", "Title is too long (120 characters max)"
	}
	if len(cleanBody) > MaxBodyLen {
		return "", "", "Request is too long (2000 characters max)"
	}
	return cleanTitle, cleanBody, ""
}

// InsertRequest stores a new prayer request and returns it with id and
// timestamps filled.
func InsertRequest(exec db.Executor, req Request) (Request, error) {
	now := time.Now()
	req.CreatedAt, req.UpdatedAt = now, now
	req.Published = true
	err := exec.QueryRow(`
		INSERT INTO prayer_requests
			(user_id, username, display_name, title, body, answered, answered_note, published, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, false, '', true, $6, $6)
		RETURNING id`,
		req.UserId, req.Username, req.DisplayName, req.Title, req.Body, now).
		Scan(&req.Id)
	if err != nil {
		return req, serr.Wrap(err, "error inserting prayer request")
	}
	return req, nil
}

// ListRequests returns published requests, newest first (the wall reads like
// a feed). limit/offset page it; callers use the limit+1 probe for has_more.
func ListRequests(exec db.Executor, limit, offset int) (reqs []Request, err error) {
	rows, err := exec.Query(`
		SELECT id, user_id, username, display_name, title, body,
			answered, answered_note, published, created_at, updated_at
		FROM prayer_requests WHERE published = true
		ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, serr.Wrap(err, "error querying prayer requests")
	}
	defer rows.Close()

	for rows.Next() {
		var r Request
		if err = rows.Scan(&r.Id, &r.UserId, &r.Username, &r.DisplayName, &r.Title, &r.Body,
			&r.Answered, &r.AnsweredNote, &r.Published, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, serr.Wrap(err, "error scanning prayer request")
		}
		reqs = append(reqs, r)
	}
	if err = rows.Err(); err != nil {
		return nil, serr.Wrap(err, "error iterating prayer requests")
	}
	return reqs, nil
}

// GetRequest loads one request. found=false (no error) when absent.
func GetRequest(exec db.Executor, id int64) (r Request, found bool, err error) {
	row := exec.QueryRow(`
		SELECT id, user_id, username, display_name, title, body,
			answered, answered_note, published, created_at, updated_at
		FROM prayer_requests WHERE id = $1`, id)
	err = row.Scan(&r.Id, &r.UserId, &r.Username, &r.DisplayName, &r.Title, &r.Body,
		&r.Answered, &r.AnsweredNote, &r.Published, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return r, false, nil
	}
	if err != nil {
		return r, false, serr.Wrap(err, "error loading prayer request")
	}
	return r, true, nil
}

// SetAnswered marks a request answered (or reopens it) with an optional
// public note — the "praise report". Editor-or-above; enforced by callers.
func SetAnswered(exec db.Executor, id int64, answered bool, note string) error {
	_, err := exec.Exec(`
		UPDATE prayer_requests SET answered = $1, answered_note = $2, updated_at = $3
		WHERE id = $4`, answered, strings.TrimSpace(note), time.Now(), id)
	if err != nil {
		return serr.Wrap(err, "error setting prayer request answered")
	}
	return nil
}

// DeleteRequest removes a request. Idempotent. Authorization (editor, or the
// requester withdrawing their own) is the caller's job.
func DeleteRequest(exec db.Executor, id int64) error {
	if _, err := exec.Exec(`DELETE FROM prayer_requests WHERE id = $1`, id); err != nil {
		return serr.Wrap(err, "error deleting prayer request")
	}
	return nil
}
