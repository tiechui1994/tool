package aes

import (
	"encoding/hex"
	"testing"
)

func TestAes(t *testing.T) {
	e := Aes{
		Mode:    MODECTR,
		Padding: PadingPKCS7,
	}

	e.Key, _ = hex.DecodeString("b0f0ba4bd7f04c828f80bb044773d897")
	ans, err := e.Encrypt([]byte("Hello World!!"))
	t.Log("enc ans:", hex.EncodeToString(ans))
	t.Log("enc err:", err)

	ans, err = e.Decrypt(ans)
	t.Log("dec ans:", string(ans))
	t.Log("dec err:", err)
}
