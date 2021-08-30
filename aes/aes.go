package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
)

const (
	MODECBC = "CBC"
	MODECFB = "CFB"
	MODECTR = "CTR"

	MODEOCF = "OCF"
	MODEECB = "ECB"
)

const (
	PadingPKCS7 = "pkcs7padding"
	PadingPKCS5 = "pkcs5padding"
	PadingZERO  = "zeropadding"
	PadingISO   = "iso10126"
	PadingANSIX = "ansix923"
)

const (
	BASE64 = "base64"
	HEX    = "hex"
)

type Aes struct {
	Mode    string
	Padding string
	Output  string
	Key     []byte
	IV      []byte
}

// 1) 对齐, 填充一个长度为 blockSize 且每个字节为 blockSize 的数据
// 2) 未对齐, 需要填充 n 个字节, 则填充一个长度为 n 且每个字节为 n 的数据
func PKCS7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padtext...)
}

func PKCS7UnPadding(data []byte) []byte {
	length := len(data)
	unpadding := int(data[length-1])
	return data[:(length - unpadding)]
}

// PKCS7Padding 基础上, blockSize 为固定值 8
func PKCS5Padding(data []byte, blockSize int) []byte {
	return PKCS7Padding(data, 8)
}

func PKCS5Unpadding(data []byte) []byte {
	padding := data[len(data)-1]
	return data[:len(data)-int(padding)]
}

// 1) 对齐, 不操作
// 2) 未对齐, 需要填充 n 个字节, 则填充一个长度为 n, 每个字节值为 0 的数据
func ZeroPadding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	if padding != blockSize {
		padtext := bytes.Repeat([]byte{0}, padding)
		return append(data, padtext...)
	}
	return data
}

func ZeroUnpadding(data []byte) []byte {
	for i := len(data) - 1; i >= 0; i-- {
		if data[i] != 0 {
			return data[:i+1]
		}
	}
	return nil
}

// aes加密, 填充秘钥key的 16, 24, 32位 分别对应AES-128, AES-192, or AES-256.
func (e *Aes) Encrypt(raw, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	// 填充原文
	blockSize := block.BlockSize()
	switch e.Padding {
	case PadingPKCS7:
		raw = PKCS7Padding(raw, blockSize)
	case PadingPKCS5:
		raw = PKCS5Padding(raw, blockSize)
	case PadingZERO:
		raw = ZeroPadding(raw, blockSize)
	}

	var (
		enc  []byte
		data []byte
	)
	if len(e.IV) == 0 || len(e.IV) != blockSize {
		// 初始向量IV必须是唯一, 但不需要保密
		enc = make([]byte, blockSize+len(raw))
		e.IV = enc[:blockSize]
		if _, err := io.ReadFull(rand.Reader, e.IV); err != nil {
			panic(err)
		}
		data = enc[blockSize:]
	} else {
		// 初始向量IV唯一
		enc = make([]byte, len(raw))
		data = enc
	}

	// block大小和初始向量大小一定要一致
	switch e.Mode {
	case MODECBC:
		mode := cipher.NewCBCEncrypter(block, e.IV)
		mode.CryptBlocks(data, raw)
	case MODECFB:
		mode := cipher.NewCFBEncrypter(block, e.IV)
		mode.XORKeyStream(data, raw)
	case MODECTR:
		mode := cipher.NewCTR(block, e.IV)
		mode.XORKeyStream(data, raw)
	}

	return enc, nil
}

func (e *Aes) Decrypt(raw, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	blockSize := block.BlockSize()
	if len(raw) < blockSize {
		panic("ciphertext too short")
	}

	if len(e.IV) == 0 || len(e.IV) != blockSize {
		e.IV = raw[:blockSize]
		raw = raw[blockSize:]
	}

	// CBC mode always works in whole blocks.
	if len(raw)%blockSize != 0 {
		panic("ciphertext is not a multiple of the block size")
	}

	switch e.Mode {
	case MODECBC:
		mode := cipher.NewCBCDecrypter(block, e.IV)
		mode.CryptBlocks(raw, raw)
	case MODECFB:
		mode := cipher.NewCFBDecrypter(block, e.IV)
		mode.XORKeyStream(raw, raw)
	case MODECTR:
		mode := cipher.NewCTR(block, e.IV)
		mode.XORKeyStream(raw, raw)
	}

	switch e.Padding {
	case PadingPKCS7:
		raw = PKCS7UnPadding(raw)
	case PadingPKCS5:
		raw = PKCS5Unpadding(raw)
	case PadingZERO:
		raw = ZeroUnpadding(raw)
	}

	return raw, nil
}
