package proxy

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"

	aessiv "github.com/jedisct1/go-aes-siv"
	"github.com/pierrec/lz4/v4"
)

// EncryptionMethod 对齐 Rust 中的加密方法枚举。
type EncryptionMethod string

const (
	EncryptionNone   EncryptionMethod = "None"
	EncryptionAES128 EncryptionMethod = "Aes128"
	EncryptionXor    EncryptionMethod = "Xor"
)

func ParseEncryptionMethod(name string) EncryptionMethod {
	switch name {
	case string(EncryptionAES128):
		return EncryptionAES128
	case string(EncryptionXor):
		return EncryptionXor
	default:
		return EncryptionNone
	}
}

// GenerateKey 对齐 Rust 侧的随机密钥生成策略。
func GenerateKey(method EncryptionMethod) ([]byte, error) {
	switch method {
	case EncryptionNone:
		return []byte("None"), nil
	case EncryptionAES128:
		key := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, err
		}
		for i := range key {
			key[i] = 33 + key[i]%94
		}
		return key, nil
	case EncryptionXor:
		sizeBuf := make([]byte, 1)
		if _, err := io.ReadFull(rand.Reader, sizeBuf); err != nil {
			return nil, err
		}
		size := 8 + int(sizeBuf[0])%56
		key := make([]byte, size)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, err
		}
		for i := range key {
			if key[i] == 0 {
				key[i] = 1
			}
		}
		return key, nil
	default:
		return []byte("None"), nil
	}
}

// CompressData 使用 LZ4 带长度前缀的块格式，对齐 Rust 的 lz4_flex::compress_prepend_size。
func CompressData(input []byte) ([]byte, error) {
	maxSize := lz4.CompressBlockBound(len(input))
	out := make([]byte, 4+maxSize)
	binary.LittleEndian.PutUint32(out[:4], uint32(len(input)))
	n, err := lz4.CompressBlock(input, out[4:], nil)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		copy(out[4:], input)
		return out[:4+len(input)], nil
	}
	return out[:4+n], nil
}

// DecompressData 对齐 Rust 的 decompress_size_prepended。
func DecompressData(input []byte) ([]byte, error) {
	if len(input) < 4 {
		return nil, fmt.Errorf("压缩数据长度不足")
	}
	size := int(binary.LittleEndian.Uint32(input[:4]))
	out := make([]byte, size)
	n, err := lz4.UncompressBlock(input[4:], out)
	if err != nil {
		// Rust 在某些极端情况下会保留原始块，这里兼容该行为。
		if len(input[4:]) == size {
			copy(out, input[4:])
			return out, nil
		}
		return nil, err
	}
	return out[:n], nil
}

// Encrypt 对齐 Rust 中的加密逻辑。
func Encrypt(method EncryptionMethod, key []byte, data []byte) ([]byte, error) {
	switch method {
	case EncryptionNone:
		return append([]byte(nil), data...), nil
	case EncryptionXor:
		return xorData(data, key), nil
	case EncryptionAES128:
		return encryptAES128SIV(key, data)
	default:
		return append([]byte(nil), data...), nil
	}
}

// Decrypt 与 Encrypt 保持对称。
func Decrypt(method EncryptionMethod, key []byte, data []byte) ([]byte, error) {
	switch method {
	case EncryptionNone:
		return append([]byte(nil), data...), nil
	case EncryptionXor:
		return xorData(data, key), nil
	case EncryptionAES128:
		return decryptAES128SIV(key, data)
	default:
		return append([]byte(nil), data...), nil
	}
}

func xorData(data []byte, key []byte) []byte {
	if len(key) == 0 {
		return append([]byte(nil), data...)
	}
	out := append([]byte(nil), data...)
	for i := range out {
		out[i] ^= key[i%len(key)]
	}
	return out
}

// encryptAES128SIV 对齐 simplestcrypt::encrypt_and_serialize：
// 1. 把 password 拷贝到 32 字节 key 缓冲区
// 2. 生成 16 字节随机 nonce
// 3. 用 AES-SIV 加密
// 4. 以 [16 字节 nonce][8 字节长度][密文] 的形式序列化
func encryptAES128SIV(password []byte, data []byte) ([]byte, error) {
	key := make([]byte, 32)
	copy(key, password)
	nonce := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	cipher, err := aessiv.New(key)
	if err != nil {
		return nil, err
	}
	ciphertext := cipher.Seal(nil, nonce, data, nil)
	out := make([]byte, 16+8+len(ciphertext))
	copy(out[:16], nonce)
	binary.LittleEndian.PutUint64(out[16:24], uint64(len(ciphertext)))
	copy(out[24:], ciphertext)
	return out, nil
}

// decryptAES128SIV 对齐 simplestcrypt::deserialize_and_decrypt。
func decryptAES128SIV(password []byte, serialized []byte) ([]byte, error) {
	if len(serialized) < 24 {
		return nil, fmt.Errorf("Aes128 数据长度不足")
	}
	key := make([]byte, 32)
	copy(key, password)
	nonce := append([]byte(nil), serialized[:16]...)
	size := binary.LittleEndian.Uint64(serialized[16:24])
	if int(size) != len(serialized[24:]) {
		return nil, fmt.Errorf("Aes128 密文长度不匹配")
	}
	cipher, err := aessiv.New(key)
	if err != nil {
		return nil, err
	}
	return cipher.Open(nil, nonce, serialized[24:], nil)
}
