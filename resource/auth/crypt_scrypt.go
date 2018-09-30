package auth

import (
	"fmt"
	"golang.org/x/crypto/scrypt"
	. "github.com/rohanthewiz/logger"
	"time"
)

func example() { // todo - make into a test
	password := "abcde"
	salt := GenSalt("$&@randomness!!$$$")
	// todo - Save salt in db

	fmt.Println("Password:", password, " Salt:", salt, " Hash:",
		PasswordHash(password, salt))
}

func PasswordHash(password, salt string) string {
	return cryptit(8, 32, password, salt)
}

func GenSalt(in string) string {
	str := fmt.Sprintf("sa*~%szoq;lnesh)^%diecp2#@", in, time.Now().Unix())
	return cryptit(12, 24, str)
}

func cryptit(r int, key_len int, password string, salts ...string) string {
	salt := ""
	if salts == nil {
		salt = fmt.Sprintf("00--%x", time.Now().Unix())
	} else {
		salt = salts[0]
	}
	dk, err := scrypt.Key([]byte(password), []byte(salt), 16384, r, 1, key_len)
	if err != nil {
		Log("Error", "Hash function failed")
	}
	return fmt.Sprintf("%x", dk)
}
