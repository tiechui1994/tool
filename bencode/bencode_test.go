package bencode

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestDecode(t *testing.T) {
	var d = "64383a616e6e6f756e636532383a7773733a2f2f776f726d686f6c652e6170702f776562736f636b657431333a616e6e6f756e63652d6c6973746c6c32383a7773733a2f2f776f726d686f6c652e6170702f776562736f636b6574656531303a6372656174656420627931353a576562546f7272656e742f3031303831333a6372656174696f6e2064617465693137323533323731303765343a696e666f64363a6c656e677468693538373732353865343a6e616d6532313a52656469732d7836342d332e302e3530342e7a6970353a6e6f6e636533323a393630333665336332613561346162353631356266333464656662653939396131323a7069656365206c656e677468693530313335303465363a70696563657334303af49cdde653b194c16e479f23bc246f74880bebfb4671b9bbe79172afc781345b1689f16c40a3d727373a7072697661746569316565373a70726976617465693165383a75726c2d6c6973746c6565"
	raw, _ := hex.DecodeString(d)
	t.Log("raw:", string(raw))

	data, err := Decode(bytes.NewBuffer(raw))
	if err != nil {
		fmt.Println(err)
		return
	}

	if v, ok := data.(map[string]interface{}); ok {
		if vv, ok := v["info"].(map[string]interface{}); ok {
			t.Logf("===: %+v", hex.EncodeToString(vv["pieces"].([]byte)))
		}
	}

	var buf bytes.Buffer
	en := json.NewEncoder(&buf)
	en.SetIndent("", " ")
	en.Encode(data)
	t.Logf("%v", buf.String())
	t.Log("====================")

	// hash info 信息
	// f49cdde653b194c16e479f23bc246f74880bebfb
	d = "64363a6c656e677468693538373732353865343a6e616d6532313a52656469732d7836342d332e302e3530342e7a6970353a6e6f6e636533323a626339333865323263613437393331643035323963666663326162396638363131323a7069656365206c656e677468693530313335303465363a70696563657334303aa7811188cd3ea43ef7355e8623d8d19feea9cb1a8c757d3f959d948796936d0b0154ffb1261ce39f373a7072697661746569316565"
	raw, _ = hex.DecodeString(d)
	t.Log("raw:", string(raw))

	t.Log("hash data:", string(raw))
	sum := sha1.New()
	sum.Write(raw)
	t.Log("sum:",hex.EncodeToString(sum.Sum(nil)))

	data, err = Decode(bytes.NewBuffer(raw))
	if err != nil {
		fmt.Println(err)
		return
	}

	buf.Reset()
	en = json.NewEncoder(&buf)
	en.SetIndent("", " ")
	en.Encode(data)
	t.Logf("%v", buf.String())
}

func TestStream(t *testing.T) {
	fd, err := os.Open("/home/quinn/Downloads/Redis-x64-3.0.504.zip")
	infoHash, torrentBase64, err:= FileTorrentCompute(fd)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	t.Log("infoHash:", infoHash)
	t.Log("torrentBase64:", torrentBase64)
}