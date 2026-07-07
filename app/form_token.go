package app

import (
	"time"

	"github.com/rohanthewiz/church/core/kvstore"
	"github.com/rohanthewiz/church/resource/auth"
	"github.com/rohanthewiz/serr"
)

// Form tokens are single-purpose CSRF tokens: a random key is stored in the
// kvstore when a form is rendered, then checked (and implicitly aged out by
// TTL) when the form posts back. They are transport-agnostic — nothing here
// depends on the HTTP stack — which is why they live in their own file
// rather than alongside the RWeb handlers.

// GenerateFormToken generates and persists a form token
func GenerateFormToken() (token string, err error) {
	tokenLifetime := 3600 * time.Second
	token = auth.RandomKey()
	err = kvstore.Set(token, "true", tokenLifetime)
	if err != nil {
		return token, serr.Wrap(err)
	}
	return
}

// VerifyFormToken checks that the token is present and valid in the kvstore
func VerifyFormToken(token string) bool {
	str, err := kvstore.Get(token)
	if err != nil {
		return false
	}
	if str == "true" {
		return true
	}
	return false
}
