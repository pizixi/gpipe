package proxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/pierrec/lz4/v4"
)

func TestCompressDataHighEntropyOutputIsValidLZ4Block(t *testing.T) {
	input := deterministicHighEntropyPayload(65535)
	encoded, err := CompressData(input)
	if err != nil {
		t.Fatalf("CompressData failed: %v", err)
	}
	if len(encoded) < 4 {
		t.Fatalf("compressed data length = %d, want at least 4", len(encoded))
	}

	out := make([]byte, len(input))
	n, err := lz4.UncompressBlock(encoded[4:], out)
	if err != nil {
		t.Fatalf("compressed payload is not a valid LZ4 block: %v", err)
	}
	if n != len(input) || !bytes.Equal(out[:n], input) {
		t.Fatalf("decoded payload mismatch, got len=%d want len=%d", n, len(input))
	}
}

func TestLZ4LiteralFallbackRoundTrip(t *testing.T) {
	for _, size := range []int{1, 14, 15, 16, 255, 256, 4096, 65535} {
		input := deterministicHighEntropyPayload(size)
		block := appendLZ4LiteralBlock(nil, input)
		out := make([]byte, len(input))
		n, err := lz4.UncompressBlock(block, out)
		if err != nil {
			t.Fatalf("literal fallback size=%d is not valid LZ4: %v", size, err)
		}
		if n != len(input) || !bytes.Equal(out[:n], input) {
			t.Fatalf("literal fallback size=%d decoded len=%d want=%d", size, n, len(input))
		}
	}
}

func TestDecompressDataAcceptsLegacyRawIncompressibleBlock(t *testing.T) {
	input := []byte("legacy raw block")
	encoded := make([]byte, 4+len(input))
	binary.LittleEndian.PutUint32(encoded[:4], uint32(len(input)))
	copy(encoded[4:], input)

	decoded, err := DecompressData(encoded)
	if err != nil {
		t.Fatalf("DecompressData failed: %v", err)
	}
	if !bytes.Equal(decoded, input) {
		t.Fatalf("decoded payload = %q, want %q", string(decoded), string(input))
	}
}

func TestSessionCommonCompressedRoundTripHighEntropy(t *testing.T) {
	common := NewSessionCommonInfo(true, ParseEncryptionMethod("None"), nil)
	defer common.Close()

	input := deterministicHighEntropyPayload(65535)
	encoded, err := common.EncodeDataAndLimit(input)
	if err != nil {
		t.Fatalf("EncodeDataAndLimit failed: %v", err)
	}
	decoded, err := common.DecodeData(encoded)
	if err != nil {
		t.Fatalf("DecodeData failed: %v", err)
	}
	if !bytes.Equal(decoded, input) {
		t.Fatalf("decoded payload mismatch, got len=%d want len=%d", len(decoded), len(input))
	}
}

func TestCompressedTCPReadChunkRoundTripWithEncryption(t *testing.T) {
	common := NewSessionCommonInfo(true, EncryptionAES128, []byte("test-key"))
	defer common.Close()

	input := deterministicHighEntropyPayload(proxyTCPReadBufferSize)
	encoded, err := common.EncodeDataAndLimit(input)
	if err != nil {
		t.Fatalf("EncodeDataAndLimit failed: %v", err)
	}
	decoded, err := common.DecodeData(encoded)
	if err != nil {
		t.Fatalf("DecodeData failed: %v", err)
	}
	if !bytes.Equal(decoded, input) {
		t.Fatalf("decoded payload mismatch, got len=%d want len=%d", len(decoded), len(input))
	}
}

func TestCompressDataEmptyPayload(t *testing.T) {
	encoded, err := CompressData(nil)
	if err != nil {
		t.Fatalf("CompressData failed: %v", err)
	}
	decoded, err := DecompressData(encoded)
	if err != nil {
		t.Fatalf("DecompressData failed: %v", err)
	}
	if len(decoded) != 0 {
		t.Fatalf("decoded length = %d, want 0", len(decoded))
	}
}

func deterministicHighEntropyPayload(size int) []byte {
	out := make([]byte, size)
	var counter uint64
	for offset := 0; offset < len(out); {
		var seed [8]byte
		binary.LittleEndian.PutUint64(seed[:], counter)
		sum := sha256.Sum256(seed[:])
		offset += copy(out[offset:], sum[:])
		counter++
	}
	return out
}
