package upgrade

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pizixi/gpipe/internal/pb"
)

func TestAcceptResumesFullyDownloadedVerifiedArtifactAtApplyStage(t *testing.T) {
	dir := t.TempDir()
	data := []byte("complete-upgrade-artifact")
	offer := &pb.UpgradeOffer{
		TaskID: "0123456789abcdef0123456789abcdef", Version: "1.1.0", Platform: "windows-amd64",
		Size: int64(len(data)), SHA256: SHA256Hex(data), ChunkSize: 1024,
	}
	offer.Signature = SignOffer("secret", offer)
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	candidate := filepath.Join(dir, "candidate-"+offer.TaskID+ext)
	if err := os.WriteFile(candidate, data, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeJSONAtomic(filepath.Join(dir, "download.json"), downloadState{
		TaskID: offer.TaskID, Version: offer.Version, Platform: offer.Platform, Size: offer.Size,
		SHA256: offer.SHA256, Candidate: candidate, Offset: offer.Size,
	}, 0o600); err != nil {
		t.Fatal(err)
	}
	receiver := &Receiver{opts: RuntimeOptions{Enabled: true, Version: "1.0.0", Platform: offer.Platform, Key: "secret"}, updateDir: dir}
	report := receiver.Accept(offer)
	if report.State != "verifying" || report.Offset != offer.Size {
		t.Fatalf("report = %+v", report)
	}
}

func TestCleanupStaleUpdateFilesProtectsActiveDownload(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "gpipe-client")
	if err := os.WriteFile(target, []byte("current"), 0o700); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(dir, "candidate-0123456789abcdef0123456789abcdef")
	stale := filepath.Join(dir, "candidate-fedcba9876543210fedcba9876543210")
	if err := os.WriteFile(active, []byte("partial"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("stale"), 0o700); err != nil {
		t.Fatal(err)
	}
	receiver := &Receiver{
		executable: target,
		updateDir:  dir,
		offer:      &pb.UpgradeOffer{TaskID: "0123456789abcdef0123456789abcdef", Size: 100},
		candidate:  active,
		offset:     7,
	}
	receiver.cleanupStaleUpdateFiles()
	if _, err := os.Stat(active); err != nil {
		t.Fatalf("active candidate was removed: %v", err)
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale candidate still exists: %v", err)
	}
}
