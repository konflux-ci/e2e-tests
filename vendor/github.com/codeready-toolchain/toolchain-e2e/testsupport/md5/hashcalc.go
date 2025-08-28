package md5

import (
	"crypto/md5" //nolint:gosec
	"encoding/hex"
)

// CalcMd5 Calculates the md5 hash of a specified value and returns it as a hex-encoded string
func CalcMd5(value string) string {
	md5hash := md5.New() //nolint:gosec
	_, _ = md5hash.Write([]byte(value))
	return hex.EncodeToString(md5hash.Sum(nil))
}
