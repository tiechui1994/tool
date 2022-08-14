package teambition

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"hash"
)

// "pkcs1_oaep"
type OAEP struct {
	alg    string
	hash   hash.Hash
	pubkey string
	prikey string
}

const (
	HASH_SHA1   = "sha1"
	HASH_SHA256 = "sha256"
	HASH_MD5    = "md5"
)

func New(hash string, pubkey, prikey string) *OAEP {
	o := OAEP{pubkey: pubkey, prikey: prikey, alg: hash}
	o.init()
	return &o
}

func (o *OAEP) init() {
	switch o.alg {
	case HASH_SHA1:
		o.hash = sha1.New()
	case HASH_SHA256:
		o.hash = sha256.New()
	case HASH_MD5:
		o.hash = md5.New()
	}
}

func (o *OAEP) Encrypt(msg []byte) string {
	block, _ := pem.Decode([]byte(o.pubkey))
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		fmt.Println("Load public key error")
		panic(err)
	}

	encrypted, err := rsa.EncryptOAEP(o.hash, rand.Reader, pub.(*rsa.PublicKey), msg, nil)
	if err != nil {
		fmt.Println("Encrypt data error")
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(encrypted)
}

func (o *OAEP) Decrypt(encrypted string) []byte {
	block, _ := pem.Decode([]byte(o.prikey))
	var pri *rsa.PrivateKey
	pri, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		fmt.Println("Load private key error")
		panic(err)
	}

	decodedData, err := base64.StdEncoding.DecodeString(encrypted)
	ciphertext, err := rsa.DecryptOAEP(o.hash, rand.Reader, pri, decodedData, nil)
	if err != nil {
		fmt.Println("Decrypt data error")
		panic(err)
	}

	return ciphertext
}
