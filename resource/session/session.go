package session

import (
	"encoding/json"
	"github.com/rohanthewiz/roredis"
	"github.com/rohanthewiz/serr"
	"time"
)

const CookieSession = "church_session"
const ttlSeconds = 1800

// Store attributes about the user's session. We will serialize the session under the session key.
// We will have one session per user
// Note: We will treat the actual session store as a simple key - value, so
//	we can easily swap out stores
type Session struct {
	Username string `json:"username"`
	FormReferrer string `json:"formReferrer"` // where to return the user to after a form
}

func (sess Session) Marshal() (data string, err error) {
	dat, err := json.Marshal(sess)
	if err != nil {
		return "", serr.NewSErr("Error marshalling user session")
	}
	return string(dat), err
}

// Key here is the value of the user's session cookie
func (sess Session) Save(key string) (err error) {
	errorStage := " when saving session"
	data, err := sess.Marshal()
	if err != nil { return serr.Wrap(err, "Error" + errorStage) }
	err = roredis.Set(key, data, ttlSeconds * time.Second)
	if err != nil { return serr.Wrap(err, "Error saving to session store") }
	return err
}

func (sess Session) Extend(key string) (err error) {
	return sess.Save(key)
}

func GetSession(key string) (sess Session, err error) {
	str, err := roredis.Get(key)
	if err != nil { return sess, serr.Wrap(err, "Error obtaining session for key: " + key) }
	err = json.Unmarshal([]byte(str), &sess)
	if err != nil {
		return sess, serr.Wrap(err, "Error unmarshalling session", "key", key, "rawData", str)
	}
	return
}

func DeleteSession(key string) error {
	return roredis.Del(key)
}
