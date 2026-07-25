package manager

import (
	"bytes"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/pizixi/gpipe/internal/pb"
	"github.com/pizixi/gpipe/internal/upgrade"
)

func TestUpgradeManagerTransfersOneVerifiedChunkPerAcknowledgement(t *testing.T) {
	m := NewUpgradeManager()
	data := bytes.Repeat([]byte("gpipe"), upgradeChunkSize/5+100)
	offer, err := m.Start(7, "1.1.0", "linux-amd64", "secret", data)
	if err != nil {
		t.Fatal(err)
	}
	if !upgrade.VerifyOffer("secret", offer) {
		t.Fatal("offer signature is invalid")
	}
	artifactPath := m.tasks[7].artifactPath
	if artifactPath == "" {
		t.Fatal("upgrade artifact should be staged on disk")
	}
	offset := int64(0)
	for offset < int64(len(data)) {
		chunk, err := m.HandleStatus(7, &pb.UpgradeStatusReport{TaskID: offer.TaskID, State: "downloading", Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		if chunk == nil || chunk.Offset != offset || upgrade.SHA256Hex(chunk.Data) != chunk.SHA256 {
			t.Fatalf("bad chunk at offset %d", offset)
		}
		offset += int64(len(chunk.Data))
	}
	if _, err := m.HandleStatus(7, &pb.UpgradeStatusReport{TaskID: offer.TaskID, State: "verifying", Offset: offset}); err != nil {
		t.Fatal(err)
	}
	if got := m.Snapshot(7).Progress; got != 100 {
		t.Fatalf("progress = %d, want 100", got)
	}
	m.CompleteOnLogin(7, "1.1.0")
	if got := m.Snapshot(7).State; got != "succeeded" {
		t.Fatalf("state = %q", got)
	}
	if _, err := os.Stat(artifactPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged artifact was not removed after success: %v", err)
	}
}

func TestUpgradeManagerSnapshotExpiresStalledTask(t *testing.T) {
	m := NewUpgradeManager()
	offer, err := m.Start(9, "2.0.0", "windows-amd64", "secret", []byte("artifact"))
	if err != nil {
		t.Fatal(err)
	}
	artifactPath := m.tasks[9].artifactPath
	m.tasks[9].updated = time.Now().Add(-upgradeTaskTTL - time.Second)

	snapshot := m.Snapshot(9)
	if snapshot.TaskID != offer.TaskID || snapshot.State != "failed" || snapshot.Error != "upgrade task expired" {
		t.Fatalf("expired snapshot = %+v", snapshot)
	}
	if _, err := os.Stat(artifactPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expired artifact was not removed: %v", err)
	}
	if m.Offer(9) != nil {
		t.Fatal("expired task must not be offered again")
	}
}
