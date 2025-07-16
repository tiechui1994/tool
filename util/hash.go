package util

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"hash/fnv"
)

func Hash(key, msg string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(msg))
	return string(h.Sum(nil))
}

func Sha256(msg string) string {
	sha := sha256.New()
	sha.Write([]byte(msg))
	return hex.EncodeToString(sha.Sum(nil))
}

func MD5(msg string) []byte {
	sha := md5.New()
	sha.Write([]byte(msg))
	return sha.Sum(nil)
}

func Fnv(msg string) uint64 {
	sha := fnv.New64a()
	sha.Write([]byte(msg))
	return sha.Sum64()
}
