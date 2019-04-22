package session

import (
	"encoding/json"
	"github.com/rohanthewiz/logger"
	"github.com/rohanthewiz/roredis"
	"github.com/rohanthewiz/serr"
	"time"
)

const CookieName = "church_session"
const ttlSeconds = 1800
const KeyNotExists = "Key does not exist"

// Store attributes about the user's session. We will serialize the session under the session key.
// We will have one session per user
// Note: We will treat the actual session store as a simple key - value, so
//	we can easily swap out stores
type Session struct {
	Key string `json:"key"`
	Username             string `json:"username"`
	FormReferrer         string `json:"formReferrer"` // where to return the user to after a form
	LastGivingReceiptURL string `json:"lastGivingReceiptURL"`
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
	sess.Key = key // save the key inside the session also ?? not sure about this
	errorStage := " when saving session"
	strSession, err := sess.Marshal()
	if err != nil {
		return serr.Wrap(err, "Error marshaling session"+errorStage)
	}
	err = roredis.Set(key, strSession, ttlSeconds*time.Second)
	if err != nil {
		return serr.Wrap(err, "Error saving to session store")
	}
	logger.Log("Info", "*** Session saved. Key: " + key + " session: " + strSession)
	return err
}

func (sess Session) Extend(key string) (err error) {
	return sess.Save(key)
}

func GetSession(key string) (sess Session, err error) {
	str, err := roredis.Get(key)
	if err != nil {
		return sess, serr.Wrap(err, "Error obtaining session", "key", key)
	}
	err = json.Unmarshal([]byte(str), &sess)
	if err != nil {
		return sess, serr.Wrap(err, "Error unmarshalling session", "key", key, "rawData", str)
	}
	return
}

func DeleteSession(key string) error {
	return roredis.Del(key)
}

// Given a session cookie name, delete it's session from the store
func DestroySession(sess_val string) (err error) {
	if sess_val != "" {
		err = DeleteSession(sess_val) // Delete the session from the store - it should expire anyway
		if err != nil {
			logger.Log("Info", "Unable to delete session", "session_key", sess_val, "Error", err.Error())
		}
		//Log("Info", "Logout", "stage", "Deleted Session from store")
	}
	return
}
