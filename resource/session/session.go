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
	Key                  string `json:"key"` // keep the key with the session so we have a means of updating it later
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
	if key == "" {
		err = serr.NewSErr("key is empty while saving session")
		logger.LogErr(err)
		return
	}
	if sess.Key != "" && sess.Key != key {
		logger.Log("Warn", "We have an inconsistency problem. Key in session: " + sess.Key, ", Key for store: " + key)
	}
	sess.Key = key // save the key inside the session also for easy access to update
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

func (sess Session) Extend() (err error) {
	return sess.Save(sess.Key)
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
	if sess.Key != "" && sess.Key != key {
		logger.Log("Warn", "We have an inconsistency problem. Key in session: " + sess.Key, ", Key for store: " + key)
	}
	sess.Key = key // ensure key is stored in the session, so we have a means of update
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
