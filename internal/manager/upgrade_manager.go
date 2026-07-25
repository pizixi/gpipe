package manager

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/pizixi/gpipe/internal/pb"
	"github.com/pizixi/gpipe/internal/upgrade"
)

const upgradeChunkSize = 128 * 1024
const upgradeTaskTTL = 45 * time.Minute
const maxUpgradeArtifactSize = 256 * 1024 * 1024

type UpgradeSnapshot struct {
	TaskID    string
	Version   string
	State     string
	Progress  int
	Error     string
	UpdatedAt time.Time
}

type upgradeTask struct {
	playerID uint32
	offer    pb.UpgradeOffer
	// artifactPath keeps large player-specific binaries out of the Go heap.
	// Only one transfer chunk is read into memory for each acknowledgement.
	artifactPath string
	state        string
	offset       int64
	err          string
	created      time.Time
	updated      time.Time
}

type UpgradeManager struct {
	mu    sync.Mutex
	tasks map[uint32]*upgradeTask
}

func NewUpgradeManager() *UpgradeManager {
	return &UpgradeManager{tasks: make(map[uint32]*upgradeTask)}
}

func (m *UpgradeManager) Start(playerID uint32, version, platform, key string, data []byte) (*pb.UpgradeOffer, error) {
	if playerID == 0 || len(data) == 0 {
		return nil, errors.New("invalid empty upgrade")
	}
	if len(data) > maxUpgradeArtifactSize {
		return nil, fmt.Errorf("upgrade artifact exceeds %d bytes", maxUpgradeArtifactSize)
	}
	taskID, err := randomTaskID()
	if err != nil {
		return nil, err
	}
	offer := pb.UpgradeOffer{
		TaskID: taskID, Version: version, Platform: platform, Size: int64(len(data)),
		SHA256: upgrade.SHA256Hex(data), ChunkSize: upgradeChunkSize,
	}
	offer.Signature = upgrade.SignOffer(key, &offer)
	artifactPath, err := stageUpgradeArtifact(taskID, data)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneExpiredLocked(now)
	if current := m.tasks[playerID]; current != nil && !terminalUpgradeState(current.state) && now.Sub(current.updated) < upgradeTaskTTL {
		_ = os.Remove(artifactPath)
		return nil, errors.New("an upgrade is already in progress")
	}
	m.tasks[playerID] = &upgradeTask{playerID: playerID, offer: offer, artifactPath: artifactPath, state: "offered", created: now, updated: now}
	result := offer
	return &result, nil
}

func (m *UpgradeManager) Offer(playerID uint32) *pb.UpgradeOffer {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneExpiredLocked(time.Now().UTC())
	task := m.tasks[playerID]
	if task == nil || terminalUpgradeState(task.state) {
		return nil
	}
	result := task.offer
	return &result
}

func (m *UpgradeManager) HandleStatus(playerID uint32, report *pb.UpgradeStatusReport) (*pb.UpgradeChunk, error) {
	if report == nil {
		return nil, errors.New("missing upgrade status")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	m.pruneExpiredLocked(now)
	task := m.tasks[playerID]
	if task == nil || task.offer.TaskID != report.TaskID {
		return nil, errors.New("upgrade task not found")
	}
	if terminalUpgradeState(task.state) {
		if task.err != "" {
			return nil, errors.New(task.err)
		}
		return nil, nil
	}
	task.updated = now
	task.state = report.State
	task.err = report.Error
	if report.Offset >= 0 && report.Offset <= task.offer.Size {
		task.offset = report.Offset
	}
	if report.State == "failed" || report.State == "rolled_back" {
		m.removeArtifactLocked(task)
		return nil, nil
	}
	if report.State != "accepted" && report.State != "downloading" {
		return nil, nil
	}
	if report.Offset < 0 || report.Offset > task.offer.Size {
		return nil, errors.New("invalid upgrade offset")
	}
	if report.Offset == task.offer.Size {
		return nil, nil
	}
	end := report.Offset + int64(task.offer.ChunkSize)
	if end > task.offer.Size {
		end = task.offer.Size
	}
	data, err := readUpgradeArtifactChunk(task.artifactPath, report.Offset, end)
	if err != nil {
		task.state, task.err, task.updated = "failed", fmt.Sprintf("read staged upgrade artifact: %v", err), now
		m.removeArtifactLocked(task)
		return nil, errors.New(task.err)
	}
	return &pb.UpgradeChunk{
		TaskID: report.TaskID, Offset: report.Offset, Data: data,
		SHA256: upgrade.SHA256Hex(data), EOF: end == task.offer.Size,
	}, nil
}

func (m *UpgradeManager) CompleteOnLogin(playerID uint32, version string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task := m.tasks[playerID]
	if task == nil || task.offer.Version != version {
		return
	}
	task.state, task.offset, task.err, task.updated = "succeeded", task.offer.Size, "", time.Now().UTC()
	m.removeArtifactLocked(task)
}

func (m *UpgradeManager) Fail(playerID uint32, taskID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task := m.tasks[playerID]
	if task == nil || (taskID != "" && task.offer.TaskID != taskID) {
		return
	}
	task.state, task.updated = "failed", time.Now().UTC()
	m.removeArtifactLocked(task)
	if err != nil {
		task.err = err.Error()
	}
}

func (m *UpgradeManager) Snapshot(playerID uint32) UpgradeSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pruneExpiredLocked(time.Now().UTC())
	task := m.tasks[playerID]
	if task == nil {
		return UpgradeSnapshot{}
	}
	progress := 0
	if task.offer.Size > 0 {
		progress = int(task.offset * 100 / task.offer.Size)
	}
	if task.state == "succeeded" {
		progress = 100
	}
	return UpgradeSnapshot{TaskID: task.offer.TaskID, Version: task.offer.Version, State: task.state, Progress: progress, Error: task.err, UpdatedAt: task.updated}
}

// Remove forgets all transient upgrade state for a player that has been
// deleted and removes any staged artifact immediately.
func (m *UpgradeManager) Remove(playerID uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if task := m.tasks[playerID]; task != nil {
		m.removeArtifactLocked(task)
		delete(m.tasks, playerID)
	}
}

func (m *UpgradeManager) pruneExpiredLocked(now time.Time) {
	for playerID, task := range m.tasks {
		if now.Sub(task.updated) < upgradeTaskTTL {
			continue
		}
		if terminalUpgradeState(task.state) {
			m.removeArtifactLocked(task)
			delete(m.tasks, playerID)
			continue
		}
		task.state = "failed"
		task.err = "upgrade task expired"
		task.updated = now
		m.removeArtifactLocked(task)
	}
}

func (m *UpgradeManager) removeArtifactLocked(task *upgradeTask) {
	if task.artifactPath == "" {
		return
	}
	_ = os.Remove(task.artifactPath)
	task.artifactPath = ""
}

func stageUpgradeArtifact(taskID string, data []byte) (string, error) {
	file, err := os.CreateTemp("", "gpipe-upgrade-"+taskID+"-*")
	if err != nil {
		return "", fmt.Errorf("create staged upgrade artifact: %w", err)
	}
	path := file.Name()
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(path)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return "", fmt.Errorf("protect staged upgrade artifact: %w", err)
	}
	if _, err := io.Copy(file, bytes.NewReader(data)); err != nil {
		return "", fmt.Errorf("write staged upgrade artifact: %w", err)
	}
	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("persist staged upgrade artifact: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close staged upgrade artifact: %w", err)
	}
	keep = true
	return path, nil
}

func readUpgradeArtifactChunk(path string, start, end int64) ([]byte, error) {
	if path == "" || start < 0 || end <= start || end-start > upgradeChunkSize {
		return nil, errors.New("invalid staged upgrade artifact range")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data := make([]byte, int(end-start))
	n, err := file.ReadAt(data, start)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if n != len(data) {
		return nil, io.ErrUnexpectedEOF
	}
	return data, nil
}

func terminalUpgradeState(state string) bool {
	return state == "succeeded" || state == "failed" || state == "rolled_back"
}

func randomTaskID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate upgrade task id: %w", err)
	}
	return hex.EncodeToString(value), nil
}
