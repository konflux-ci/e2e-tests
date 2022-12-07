package gohooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
)

// IsGoHookValid checks the sha256 of the data matches the one given on the signature.
func IsGoHookValid(data interface{}, signature, secret string) bool {
	h := hmac.New(sha256.New, []byte(secret))

	preparedData, err := json.Marshal(data)
	if err != nil {
		log.Println(err.Error())
		return false
	}

	_, err = h.Write(preparedData)
	if err != nil {
		log.Println(err.Error())
		return false
	}

	sha := hex.EncodeToString(h.Sum(nil))

	return sha == signature
}
