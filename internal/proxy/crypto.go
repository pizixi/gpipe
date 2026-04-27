package proxy

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	aessiv "github.com/jedisct1/go-aes-siv"
	"github.com/pierrec/lz4/v4"
)

// lz4HashTablePool 复用 LZ4 压缩用的 hashtable，避免每次压缩都重新分配，
// 同时绕开旧版本 pierrec/lz4 在传 nil hashTable 路径上对低可压缩数据的边界 bug。
var lz4HashTablePool = sync.Pool{
	New: func() any {
		// pierrec/lz4 v4 默认 hashtable 大小为 1<<16。
		table := make([]int, 1<<16)
		return &table
	},
}

// lz4MaxDecompressedSize 是单个块允许的最大解压尺寸（与 LZ4 块格式上限一致）。
// 在生产中我们的 TCP/UDP 单次读取上限为 64KB，留较大冗余以兜底潜在编码端缺陷。
const lz4MaxDecompressedSize = 8 * 1024 * 1024

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
	if len(input) == 0 {
		return []byte{0, 0, 0, 0}, nil
	}
	maxSize := lz4.CompressBlockBound(len(input))
	out := make([]byte, 4+maxSize)
	binary.LittleEndian.PutUint32(out[:4], uint32(len(input)))

	tablePtr := lz4HashTablePool.Get().(*[]int)
	table := *tablePtr
	for i := range table {
		table[i] = 0
	}
	n, err := lz4.CompressBlock(input, out[4:], table)
	lz4HashTablePool.Put(tablePtr)
	if err != nil {
		// 编码失败时退回纯 literal 块，保证整条链路稳定。
		return appendLZ4LiteralBlock(out[:4], input), nil
	}
	// n == 0 表示不可压缩；n >= len(input) 表示压缩没收益且更易触发部分版本的解码越界。
	// 两种情况都退回 literal 块。
	if n <= 0 || n >= len(input) {
		return appendLZ4LiteralBlock(out[:4], input), nil
	}
	return out[:4+n], nil
}

// DecompressData 对齐 Rust 的 decompress_size_prepended。
// 为兼容历史客户端及防御编码端潜在的越界写入，这里在常规路径之外提供两层兜底：
//  1. 如果 lz4 返回 ErrInvalidSourceShortBuffer，扩大输出缓冲再试一次；
//  2. 仍失败则按原始（未压缩）块兼容处理。
func DecompressData(input []byte) ([]byte, error) {
	if len(input) < 4 {
		return nil, fmt.Errorf("压缩数据长度不足")
	}
	size := int(binary.LittleEndian.Uint32(input[:4]))
	if size == 0 {
		return []byte{}, nil
	}
	if size < 0 || size > lz4MaxDecompressedSize {
		return nil, fmt.Errorf("LZ4 解压长度异常: size=%d", size)
	}
	payload := input[4:]

	out := make([]byte, size)
	n, err := lz4.UncompressBlock(payload, out)
	if err == nil && n == size {
		return out[:n], nil
	}

	// 兼容历史客户端：不可压缩场景下直接发送原始字节。
	if len(payload) == size {
		copy(out, payload)
		return out, nil
	}

	if err != nil {
		// 防御性兜底：若编码端因历史 bug 产生略大于 size 的输出，
		// 用更大的缓冲区再尝试一次，并截断至 size。
		large := make([]byte, lz4MaxDecompressedSize)
		if n2, err2 := lz4.UncompressBlock(payload, large); err2 == nil && n2 >= size {
			return append([]byte(nil), large[:size]...), nil
		}
		return nil, err
	}
	return nil, fmt.Errorf("LZ4 解压长度不匹配: got=%d want=%d", n, size)
}

func appendLZ4LiteralBlock(out []byte, input []byte) []byte {
	literalLen := len(input)
	if literalLen < 15 {
		out = append(out, byte(literalLen<<4))
		return append(out, input...)
	}

	out = append(out, 15<<4)
	remaining := literalLen - 15
	for remaining >= 255 {
		out = append(out, 255)
		remaining -= 255
	}
	out = append(out, byte(remaining))
	return append(out, input...)
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
