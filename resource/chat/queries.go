package chat

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/rohanthewiz/church/db"
	"github.com/rohanthewiz/serr"
)

// Query functions take the executor first (db.Executor convention — see
// db/executor.go) so they run equally against the live handle, a
// transaction, or sqlmock in the contract tests.

// InsertMessage stores a moderated message and returns it with its assigned
// id and timestamp (the caller broadcasts exactly what was stored, so SSE
// listeners and later page loads can never disagree).
func InsertMessage(exec db.Executor, msg Message) (Message, error) {
	msg.CreatedAt = time.Now()
	err := exec.QueryRow(`
		INSERT INTO chat_messages (channel, user_id, username, display_name, body, keep, created_at)
		VALUES ($1, $2, $3, $4, $5, false, $6)
		RETURNING id`,
		msg.Channel, msg.UserId, msg.Username, msg.DisplayName, msg.Body, msg.CreatedAt).
		Scan(&msg.Id)
	if err != nil {
		return msg, serr.Wrap(err, "error inserting chat message", "channel", msg.Channel)
	}
	return msg, nil
}

// RecentMessages returns up to limit messages of a channel in ascending id
// order (chat renders oldest → newest top to bottom).
//
// Two access patterns share this function:
//   - initial window load: afterId = 0 → the NEWEST limit messages
//     (selected descending, then reversed here so callers always get
//     ascending order)
//   - incremental poll: afterId > 0 → messages strictly newer than afterId,
//     oldest first, which is exactly the append order a poller wants
func RecentMessages(exec db.Executor, channel string, afterId int64, limit int) (msgs []Message, err error) {
	var rows *sql.Rows
	// LIMIT is interpolated, not bound: bytdb rejects placeholders in LIMIT,
	// and a typed int can't carry injection. Postgres accepts either form.
	if afterId > 0 {
		rows, err = exec.Query(fmt.Sprintf(`
			SELECT id, channel, user_id, username, display_name, body, keep, created_at
			FROM chat_messages WHERE channel = $1 AND id > $2
			ORDER BY id ASC LIMIT %d`, limit), channel, afterId)
	} else {
		rows, err = exec.Query(fmt.Sprintf(`
			SELECT id, channel, user_id, username, display_name, body, keep, created_at
			FROM chat_messages WHERE channel = $1
			ORDER BY id DESC LIMIT %d`, limit), channel)
	}
	if err != nil {
		return nil, serr.Wrap(err, "error querying chat messages", "channel", channel)
	}
	defer rows.Close()

	for rows.Next() {
		var m Message
		if err = rows.Scan(&m.Id, &m.Channel, &m.UserId, &m.Username, &m.DisplayName,
			&m.Body, &m.Keep, &m.CreatedAt); err != nil {
			return nil, serr.Wrap(err, "error scanning chat message")
		}
		msgs = append(msgs, m)
	}
	if err = rows.Err(); err != nil {
		return nil, serr.Wrap(err, "error iterating chat messages")
	}

	if afterId == 0 { // newest-first window → reverse into display order
		for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
			msgs[i], msgs[j] = msgs[j], msgs[i]
		}
	}
	return msgs, nil
}

// GetMessage loads one message. found=false (no error) when it doesn't
// exist — deleted-vs-never-existed is not a distinction moderation needs.
func GetMessage(exec db.Executor, id int64) (m Message, found bool, err error) {
	row := exec.QueryRow(`
		SELECT id, channel, user_id, username, display_name, body, keep, created_at
		FROM chat_messages WHERE id = $1`, id)
	err = row.Scan(&m.Id, &m.Channel, &m.UserId, &m.Username, &m.DisplayName,
		&m.Body, &m.Keep, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return m, false, nil
	}
	if err != nil {
		return m, false, serr.Wrap(err, "error loading chat message")
	}
	return m, true, nil
}

// SetKeep pins or unpins a message (keep = true exempts it from the
// retention sweep). Editor-or-above only — enforced by callers via
// CanModerate; the query itself stays policy-free.
func SetKeep(exec db.Executor, id int64, keep bool) error {
	if _, err := exec.Exec(`UPDATE chat_messages SET keep = $1 WHERE id = $2`, keep, id); err != nil {
		return serr.Wrap(err, "error setting chat message keep flag")
	}
	return nil
}

// DeleteMessage removes one message (moderation). Idempotent — deleting an
// already-gone message is success.
func DeleteMessage(exec db.Executor, id int64) error {
	if _, err := exec.Exec(`DELETE FROM chat_messages WHERE id = $1`, id); err != nil {
		return serr.Wrap(err, "error deleting chat message")
	}
	return nil
}

// DeleteExpired implements the retention policy: everything older than
// cutoff goes, unless an editor pinned it. Returns rows deleted for the
// sweep's log line.
func DeleteExpired(exec db.Executor, cutoff time.Time) (int64, error) {
	res, err := exec.Exec(`DELETE FROM chat_messages WHERE keep = false AND created_at < $1`, cutoff)
	if err != nil {
		return 0, serr.Wrap(err, "error deleting expired chat messages")
	}
	n, _ := res.RowsAffected()
	return n, nil
}
