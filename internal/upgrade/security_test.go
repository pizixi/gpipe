package upgrade

import (
	"testing"

	"github.com/pizixi/gpipe/internal/pb"
)

func TestOfferSignatureBindsAllSecurityFields(t *testing.T) {
	offer := &pb.UpgradeOffer{TaskID: "0123456789abcdef0123456789abcdef", Version: "1.2.3", Platform: "windows-amd64", Size: 42, SHA256: SHA256Hex([]byte("artifact")), ChunkSize: 1024}
	offer.Signature = SignOffer("player-key", offer)
	if !VerifyOffer("player-key", offer) {
		t.Fatal("expected signature to verify")
	}
	offer.Size++
	if VerifyOffer("player-key", offer) {
		t.Fatal("tampered offer unexpectedly verified")
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{"1.0.0", "1.0.0", 0}, {"1.0.0", "1.0.1", -1}, {"2.0.0", "1.9.9", 1},
		{"1.0.0-rc.1", "1.0.0", -1}, {"1.0.0-rc.2", "1.0.0-rc.10", -1},
	}
	for _, test := range tests {
		got, ok := CompareVersions(test.left, test.right)
		if !ok || got != test.want {
			t.Fatalf("CompareVersions(%q, %q) = %d, %v; want %d, true", test.left, test.right, got, ok, test.want)
		}
	}
	if _, ok := CompareVersions("dev", "1.0.0"); ok {
		t.Fatal("invalid version must not be comparable")
	}
}
