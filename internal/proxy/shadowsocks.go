package proxy

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

const (
	ShadowsocksMethodAES128GCM            = "aes-128-gcm"
	ShadowsocksMethodAES256GCM            = "aes-256-gcm"
	ShadowsocksMethodChacha20IETFPoly1305 = "chacha20-ietf-poly1305"
	DefaultShadowsocksMethod              = ShadowsocksMethodChacha20IETFPoly1305

	shadowsocksInfo       = "ss-subkey"
	shadowsocksMaxPayload = 0x3fff
)

type shadowsocksMethodSpec struct {
	name     string
	keySize  int
	saltSize int
	newAEAD  func(key []byte) (cipher.AEAD, error)
}

type shadowsocksStreamReader struct {
	spec        shadowsocksMethodSpec
	masterKey   []byte
	buffer      []byte
	aead        cipher.AEAD
	nonce       []byte
	payloadSize int
}

type shadowsocksStreamWriter struct {
	spec      shadowsocksMethodSpec
	masterKey []byte
	salt      []byte
	aead      cipher.AEAD
	nonce     []byte
	saltSent  bool
}

func IsSupportedShadowsocksMethod(name string) bool {
	_, ok := shadowsocksMethodSpecByName(normalizeShadowsocksMethod(name))
	return ok
}

func newShadowsocksStreamReader(method, password string) (*shadowsocksStreamReader, error) {
	spec, ok := shadowsocksMethodSpecByName(normalizeShadowsocksMethod(method))
	if !ok {
		return nil, fmt.Errorf("unsupported shadowsocks method: %s", method)
	}
	return &shadowsocksStreamReader{
		spec:      spec,
		masterKey: deriveShadowsocksMasterKey(password, spec.keySize),
	}, nil
}

func (r *shadowsocksStreamReader) Feed(data []byte) ([][]byte, error) {
	r.buffer = append(r.buffer, data...)
	if r.aead == nil {
		if len(r.buffer) < r.spec.saltSize {
			return nil, nil
		}
		salt := append([]byte(nil), r.buffer[:r.spec.saltSize]...)
		aead, err := newShadowsocksAEAD(r.spec, r.masterKey, salt)
		if err != nil {
			return nil, err
		}
		r.aead = aead
		r.nonce = make([]byte, aead.NonceSize())
		r.buffer = r.buffer[r.spec.saltSize:]
	}

	var out [][]byte
	for {
		if r.payloadSize == 0 {
			sizeBlockLen := 2 + r.aead.Overhead()
			if len(r.buffer) < sizeBlockLen {
				break
			}
			sizeBlock, err := r.aead.Open(nil, r.nonce, r.buffer[:sizeBlockLen], nil)
			if err != nil {
				return nil, err
			}
			incrementShadowsocksNonce(r.nonce)
			r.buffer = r.buffer[sizeBlockLen:]
			if len(sizeBlock) != 2 {
				return nil, fmt.Errorf("invalid shadowsocks tcp chunk size")
			}
			r.payloadSize = int(binary.BigEndian.Uint16(sizeBlock))
			if r.payloadSize <= 0 || r.payloadSize > shadowsocksMaxPayload {
				return nil, fmt.Errorf("invalid shadowsocks tcp payload size: %d", r.payloadSize)
			}
		}

		payloadBlockLen := r.payloadSize + r.aead.Overhead()
		if len(r.buffer) < payloadBlockLen {
			break
		}
		payload, err := r.aead.Open(nil, r.nonce, r.buffer[:payloadBlockLen], nil)
		if err != nil {
			return nil, err
		}
		incrementShadowsocksNonce(r.nonce)
		r.buffer = r.buffer[payloadBlockLen:]
		r.payloadSize = 0
		out = append(out, payload)
	}

	return out, nil
}

func newShadowsocksStreamWriter(method, password string) (*shadowsocksStreamWriter, error) {
	spec, ok := shadowsocksMethodSpecByName(normalizeShadowsocksMethod(method))
	if !ok {
		return nil, fmt.Errorf("unsupported shadowsocks method: %s", method)
	}

	salt := make([]byte, spec.saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	masterKey := deriveShadowsocksMasterKey(password, spec.keySize)
	aead, err := newShadowsocksAEAD(spec, masterKey, salt)
	if err != nil {
		return nil, err
	}

	return &shadowsocksStreamWriter{
		spec:      spec,
		masterKey: masterKey,
		salt:      salt,
		aead:      aead,
		nonce:     make([]byte, aead.NonceSize()),
	}, nil
}

func (w *shadowsocksStreamWriter) SealData(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	out := make([]byte, 0, len(data)+(len(data)/shadowsocksMaxPayload+1)*(2+w.aead.Overhead()*2)+len(w.salt))
	if !w.saltSent {
		out = append(out, w.salt...)
		w.saltSent = true
	}

	for len(data) > 0 {
		chunkSize := len(data)
		if chunkSize > shadowsocksMaxPayload {
			chunkSize = shadowsocksMaxPayload
		}
		chunk := data[:chunkSize]
		data = data[chunkSize:]

		sizePlain := []byte{byte(chunkSize >> 8), byte(chunkSize)}
		out = append(out, w.aead.Seal(nil, w.nonce, sizePlain, nil)...)
		incrementShadowsocksNonce(w.nonce)
		out = append(out, w.aead.Seal(nil, w.nonce, chunk, nil)...)
		incrementShadowsocksNonce(w.nonce)
	}

	return out, nil
}

func shadowsocksEncryptPacket(method, password string, payload []byte) ([]byte, error) {
	spec, ok := shadowsocksMethodSpecByName(normalizeShadowsocksMethod(method))
	if !ok {
		return nil, fmt.Errorf("unsupported shadowsocks method: %s", method)
	}
	salt := make([]byte, spec.saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	aead, err := newShadowsocksAEAD(spec, deriveShadowsocksMasterKey(password, spec.keySize), salt)
	if err != nil {
		return nil, err
	}
	out := append([]byte(nil), salt...)
	out = append(out, aead.Seal(nil, make([]byte, aead.NonceSize()), payload, nil)...)
	return out, nil
}

func shadowsocksDecryptPacket(method, password string, packet []byte) ([]byte, error) {
	spec, ok := shadowsocksMethodSpecByName(normalizeShadowsocksMethod(method))
	if !ok {
		return nil, fmt.Errorf("unsupported shadowsocks method: %s", method)
	}
	if len(packet) < spec.saltSize {
		return nil, fmt.Errorf("shadowsocks udp packet too short")
	}
	salt := append([]byte(nil), packet[:spec.saltSize]...)
	aead, err := newShadowsocksAEAD(spec, deriveShadowsocksMasterKey(password, spec.keySize), salt)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, make([]byte, aead.NonceSize()), packet[spec.saltSize:], nil)
}

func normalizeShadowsocksMethod(name string) string {
	if name == "" {
		return DefaultShadowsocksMethod
	}
	return name
}

func shadowsocksMethodSpecByName(name string) (shadowsocksMethodSpec, bool) {
	switch name {
	case ShadowsocksMethodAES128GCM:
		return shadowsocksMethodSpec{
			name:     name,
			keySize:  16,
			saltSize: 16,
			newAEAD: func(key []byte) (cipher.AEAD, error) {
				block, err := aes.NewCipher(key)
				if err != nil {
					return nil, err
				}
				return cipher.NewGCM(block)
			},
		}, true
	case ShadowsocksMethodAES256GCM:
		return shadowsocksMethodSpec{
			name:     name,
			keySize:  32,
			saltSize: 32,
			newAEAD: func(key []byte) (cipher.AEAD, error) {
				block, err := aes.NewCipher(key)
				if err != nil {
					return nil, err
				}
				return cipher.NewGCM(block)
			},
		}, true
	case ShadowsocksMethodChacha20IETFPoly1305:
		return shadowsocksMethodSpec{
			name:     name,
			keySize:  chacha20poly1305.KeySize,
			saltSize: chacha20poly1305.KeySize,
			newAEAD: func(key []byte) (cipher.AEAD, error) {
				return chacha20poly1305.New(key)
			},
		}, true
	default:
		return shadowsocksMethodSpec{}, false
	}
}

func deriveShadowsocksMasterKey(password string, keySize int) []byte {
	out := make([]byte, 0, keySize)
	previous := []byte(nil)
	for len(out) < keySize {
		h := md5.New()
		if len(previous) > 0 {
			_, _ = h.Write(previous)
		}
		_, _ = h.Write([]byte(password))
		previous = h.Sum(nil)
		out = append(out, previous...)
	}
	return append([]byte(nil), out[:keySize]...)
}

func newShadowsocksAEAD(spec shadowsocksMethodSpec, masterKey, salt []byte) (cipher.AEAD, error) {
	subkey := make([]byte, spec.keySize)
	reader := hkdf.New(sha1.New, masterKey, salt, []byte(shadowsocksInfo))
	if _, err := io.ReadFull(reader, subkey); err != nil {
		return nil, err
	}
	return spec.newAEAD(subkey)
}

func incrementShadowsocksNonce(nonce []byte) {
	for i := range nonce {
		nonce[i]++
		if nonce[i] != 0 {
			return
		}
	}
}
