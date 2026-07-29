package proxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
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

func TestSessionCommonAES128ConcurrentRoundTrip(t *testing.T) {
	common := NewSessionCommonInfo(false, EncryptionAES128, []byte("concurrent-test-key"))
	defer common.Close()
	input := deterministicHighEntropyPayload(4096)

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for worker := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := range 100 {
				encoded, err := common.EncodeDataAndLimit(input)
				if err != nil {
					errs <- fmt.Errorf("worker %d iteration %d encode: %w", worker, iteration, err)
					return
				}
				common.Flow.Release(len(encoded))
				decoded, err := common.DecodeData(encoded)
				if err != nil {
					errs <- fmt.Errorf("worker %d iteration %d decode: %w", worker, iteration, err)
					return
				}
				if !bytes.Equal(decoded, input) {
					errs <- fmt.Errorf("worker %d iteration %d decoded payload mismatch", worker, iteration)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
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

func BenchmarkCompressData(b *testing.B) {
	for _, tc := range []struct {
		name  string
		input []byte
	}{
		{name: "1KiB_Repeated", input: bytes.Repeat([]byte("gpipe"), 1024/5+1)[:1024]},
		{name: "1KiB_HighEntropy", input: deterministicHighEntropyPayload(1024)},
		{name: "64KiB_Repeated", input: bytes.Repeat([]byte("gpipe"), 65535/5)},
		{name: "64KiB_HighEntropy", input: deterministicHighEntropyPayload(65535)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(len(tc.input)))
			b.ReportAllocs()
			for b.Loop() {
				if _, err := CompressData(tc.input); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSessionCommonAES128RoundTrip(b *testing.B) {
	for _, size := range []int{1024, 65535} {
		input := deterministicHighEntropyPayload(size)
		b.Run(fmt.Sprintf("%dB", size), func(b *testing.B) {
			common := NewSessionCommonInfo(false, EncryptionAES128, []byte("benchmark-key"))
			defer common.Close()
			b.SetBytes(int64(size))
			b.ReportAllocs()
			for b.Loop() {
				encoded, err := common.EncodeDataAndLimit(input)
				if err != nil {
					b.Fatal(err)
				}
				common.Flow.Release(len(encoded))
				decoded, err := common.DecodeData(encoded)
				if err != nil {
					b.Fatal(err)
				}
				if len(decoded) != len(input) {
					b.Fatalf("decoded length = %d, want %d", len(decoded), len(input))
				}
			}
		})
	}
}
