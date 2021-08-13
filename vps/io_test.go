package vps

import (
	"bytes"
	"encoding/hex"
	"testing"
	"time"
)

func TestLogin(t *testing.T) {
	login("tiechui1994@163.com", "0214.abcd")
	time.Sleep(2 * time.Second)
}

func TestRun(t *testing.T) {
	cs, err := containers()
	if err != nil || len(cs) == 0 {
		t.Log("err", err)
		return
	}

	_, err = run(cs[0])
	if err != nil {
		t.Log("err", err, cs[0])
	}

	time.Sleep(10 * time.Minute)
}

func TestParse(t *testing.T) {
	origin := "000907ff307b22736964223a2234336b5632306f4b4e782d36376f356d4142704a222c227570677261646573223a5b22776562736f636b6574225d2c2270696e67496e74657276616c223a32353030302c2270696e6754696d656f7574223a36303030307d"

	data, _ := hex.DecodeString(origin)
	t.Log("data", string(data))
	t.Log("idx", bytes.IndexRune(data, '0'))
	t.Log("xx", data[:4], len(data), len(data)-4, string(data[:4]))
}
