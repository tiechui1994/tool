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
)

type Aes struct {
	Mode    string
	Padding string
	Key     []byte
	IV      []byte
}

/*
ANSI X.923, 先是填充0x00, 最后一个字节填充 padded 的字节个数.

ISO 10126, 先是填充随机值, 最后一个字节填充 padded 的字节个数.
*/

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

// PKCS#5 填充与 PKCS#7 填充相同, 不同之处在于它仅针对使用 64 位(8字节) 块大小的块密码进行了定义. 在实践中, 这两者可以互换使用
// PKCS7Padding 基础上, blockSize 为固定值 8
func PKCS5Padding(data []byte, blockSize int) []byte {
	return PKCS7Padding(data, blockSize)
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
func (e *Aes) Encrypt(raw []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.Key)
	if err != nil {
		return nil, err
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
		enc, dst, iv []byte
	)
	if len(e.IV) != blockSize {
		// 初始向量IV必须是唯一, 但不需要保密
		enc = make([]byte, blockSize+len(raw))
		iv = enc[:blockSize]
		if _, err := io.ReadFull(rand.Reader, iv); err != nil {
			return nil, err
		}
		dst = enc[blockSize:]
	} else {
		// 初始向量IV唯一
		enc = make([]byte, len(raw))
		iv = make([]byte, blockSize)
		copy(iv, e.IV)
		dst = enc
	}

	// block大小和初始向量大小一定要一致
	switch e.Mode {
	case MODECBC:
		mode := cipher.NewCBCEncrypter(block, iv)
		mode.CryptBlocks(dst, raw)
	case MODECFB:
		mode := cipher.NewCFBEncrypter(block, iv)
		mode.XORKeyStream(dst, raw)
	case MODECTR:
		mode := cipher.NewCTR(block, iv)
		mode.XORKeyStream(dst, raw)
	}

	return enc, nil
}

func (e *Aes) Decrypt(raw []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.Key)
	if err != nil {
		return nil, err
	}

	blockSize := block.BlockSize()
	if len(raw) < blockSize {
		panic("ciphertext too short")
	}

	var iv []byte
	if len(e.IV) != blockSize {
		iv = raw[:blockSize]
		raw = raw[blockSize:]
	} else {
		iv = make([]byte, blockSize)
		copy(iv, e.IV)
	}

	// CBC mode always works in whole blocks.
	if len(raw)%blockSize != 0 {
		panic("ciphertext is not a multiple of the block size")
	}

	switch e.Mode {
	case MODECBC:
		mode := cipher.NewCBCDecrypter(block, iv)
		mode.CryptBlocks(raw, raw)
	case MODECFB:
		mode := cipher.NewCFBDecrypter(block, iv)
		mode.XORKeyStream(raw, raw)
	case MODECTR:
		mode := cipher.NewCTR(block, iv)
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
