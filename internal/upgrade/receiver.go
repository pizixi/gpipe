package upgrade

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pizixi/gpipe/internal/pb"
)

const maxArtifactSize int64 = 256 * 1024 * 1024

var taskIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

var ErrApplyStarted = errors.New("client upgrade apply helper started")

type RuntimeOptions struct {
	Enabled     bool
	Version     string
	Platform    string
	Key         string
	Mode        string
	RestartArgs []string
	ServiceName string
	Logger      *log.Logger
}

type Receiver struct {
	mu         sync.Mutex
	opts       RuntimeOptions
	executable string
	updateDir  string
	offer      *pb.UpgradeOffer
	candidate  string
	offset     int64
}

type downloadState struct {
	TaskID    string `json:"task_id"`
	Version   string `json:"version"`
	Platform  string `json:"platform"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	Candidate string `json:"candidate"`
	Offset    int64  `json:"offset"`
}

type ApplyState struct {
	TaskID          string    `json:"task_id"`
	Target          string    `json:"target"`
	Candidate       string    `json:"candidate"`
	Backup          string    `json:"backup"`
	Mode            string    `json:"mode"`
	RestartArgs     []string  `json:"restart_args"`
	ServiceName     string    `json:"service_name"`
	ExpectedSHA256  string    `json:"expected_sha256"`
	ExpectedVersion string    `json:"expected_version"`
	ParentPID       int       `json:"parent_pid"`
	HealthPath      string    `json:"health_path"`
	ResultPath      string    `json:"result_path"`
	CreatedAt       time.Time `json:"created_at"`
}

type applyResult struct {
	TaskID string `json:"task_id"`
	State  string `json:"state"`
	Error  string `json:"error"`
}

func NewReceiver(opts RuntimeOptions) (*Receiver, error) {
	if opts.Logger == nil {
		opts.Logger = log.Default()
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
		executable = resolved
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return nil, err
	}
	updateDir := filepath.Join(filepath.Dir(executable), ".gpipe-update")
	receiver := &Receiver{opts: opts, executable: executable, updateDir: updateDir}
	receiver.cleanupStaleUpdateFiles()
	return receiver, nil
}

func (r *Receiver) Accept(offer *pb.UpgradeOffer) *pb.UpgradeStatusReport {
	r.mu.Lock()
	defer r.mu.Unlock()

	fail := func(err error) *pb.UpgradeStatusReport {
		taskID := ""
		if offer != nil {
			taskID = offer.TaskID
		}
		return &pb.UpgradeStatusReport{TaskID: taskID, State: "failed", Error: err.Error()}
	}
	if !r.opts.Enabled {
		return fail(errors.New("self update is disabled for this client mode"))
	}
	if offer == nil || !taskIDPattern.MatchString(offer.TaskID) {
		return fail(errors.New("invalid upgrade task"))
	}
	if offer.Platform != r.opts.Platform {
		return fail(fmt.Errorf("platform mismatch: %s", offer.Platform))
	}
	if comparison, ok := CompareVersions(r.opts.Version, offer.Version); !ok || comparison >= 0 {
		return fail(errors.New("target version is not newer than current version"))
	}
	if offer.Size <= 0 || offer.Size > maxArtifactSize {
		return fail(errors.New("invalid upgrade artifact size"))
	}
	if offer.ChunkSize == 0 || offer.ChunkSize > 1024*1024 {
		return fail(errors.New("invalid upgrade chunk size"))
	}
	if len(offer.SHA256) != sha256.Size*2 {
		return fail(errors.New("invalid upgrade digest"))
	}
	if !VerifyOffer(r.opts.Key, offer) {
		return fail(errors.New("upgrade signature verification failed"))
	}
	if err := os.MkdirAll(r.updateDir, 0o700); err != nil {
		return fail(fmt.Errorf("create update directory: %w", err))
	}
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	candidate := filepath.Join(r.updateDir, "candidate-"+offer.TaskID+ext)
	offset := int64(0)
	var previous downloadState
	if readJSON(filepath.Join(r.updateDir, "download.json"), &previous) == nil &&
		previous.TaskID == offer.TaskID && previous.SHA256 == offer.SHA256 && previous.Size == offer.Size && previous.Candidate == candidate {
		if info, err := os.Stat(candidate); err == nil && info.Size() == previous.Offset && previous.Offset <= offer.Size {
			offset = previous.Offset
		}
	}
	if offset == 0 {
		file, err := os.OpenFile(candidate, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
		if err != nil {
			return fail(fmt.Errorf("prepare upgrade file: %w", err))
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			return fail(err)
		}
		_ = file.Close()
	}
	r.offer, r.candidate, r.offset = offer, candidate, offset
	if err := r.saveDownloadState(); err != nil {
		return fail(err)
	}
	if offset == offer.Size {
		if digest, err := fileSHA256(candidate); err == nil && digest == strings.ToLower(offer.SHA256) {
			return &pb.UpgradeStatusReport{TaskID: offer.TaskID, State: "verifying", Offset: offset}
		}
		file, err := os.OpenFile(candidate, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
		if err != nil {
			return fail(err)
		}
		if err := file.Sync(); err != nil {
			_ = file.Close()
			return fail(err)
		}
		_ = file.Close()
		r.offset, offset = 0, 0
		if err := r.saveDownloadState(); err != nil {
			return fail(err)
		}
	}
	return &pb.UpgradeStatusReport{TaskID: offer.TaskID, State: "accepted", Offset: offset}
}

func (r *Receiver) HandleChunk(chunk *pb.UpgradeChunk) (*pb.UpgradeStatusReport, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	fail := func(err error) (*pb.UpgradeStatusReport, bool) {
		taskID := ""
		if r.offer != nil {
			taskID = r.offer.TaskID
		}
		return &pb.UpgradeStatusReport{TaskID: taskID, State: "failed", Offset: r.offset, Error: err.Error()}, false
	}
	if r.offer == nil || chunk == nil || chunk.TaskID != r.offer.TaskID {
		return fail(errors.New("unexpected upgrade chunk"))
	}
	if chunk.Offset != r.offset {
		return fail(fmt.Errorf("unexpected chunk offset %d, want %d", chunk.Offset, r.offset))
	}
	if len(chunk.Data) == 0 || len(chunk.Data) > int(r.offer.ChunkSize) {
		return fail(errors.New("invalid upgrade chunk length"))
	}
	if SHA256Hex(chunk.Data) != strings.ToLower(chunk.SHA256) {
		return fail(errors.New("upgrade chunk checksum mismatch"))
	}
	if r.offset+int64(len(chunk.Data)) > r.offer.Size {
		return fail(errors.New("upgrade chunk exceeds artifact size"))
	}
	file, err := os.OpenFile(r.candidate, os.O_WRONLY, 0o700)
	if err != nil {
		return fail(err)
	}
	if _, err = file.WriteAt(chunk.Data, r.offset); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return fail(fmt.Errorf("write upgrade chunk: %w", err))
	}
	r.offset += int64(len(chunk.Data))
	if err := r.saveDownloadState(); err != nil {
		return fail(err)
	}
	if !chunk.EOF {
		return &pb.UpgradeStatusReport{TaskID: r.offer.TaskID, State: "downloading", Offset: r.offset}, false
	}
	if r.offset != r.offer.Size {
		return fail(errors.New("upgrade ended before declared size"))
	}
	digest, err := fileSHA256(r.candidate)
	if err != nil {
		return fail(err)
	}
	if digest != strings.ToLower(r.offer.SHA256) {
		return fail(errors.New("upgrade artifact checksum mismatch"))
	}
	if err := os.Chmod(r.candidate, 0o700); err != nil {
		return fail(err)
	}
	return &pb.UpgradeStatusReport{TaskID: r.offer.TaskID, State: "verifying", Offset: r.offset}, true
}

func (r *Receiver) StartApply() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.offer == nil || r.candidate == "" || r.offset != r.offer.Size {
		return errors.New("upgrade is not ready to apply")
	}
	state := ApplyState{
		TaskID: r.offer.TaskID, Target: r.executable, Candidate: r.candidate,
		Backup: r.executable + ".backup-" + r.offer.TaskID, Mode: r.opts.Mode,
		RestartArgs: append([]string(nil), r.opts.RestartArgs...), ServiceName: r.opts.ServiceName,
		ExpectedSHA256: r.offer.SHA256, ExpectedVersion: r.offer.Version, ParentPID: os.Getpid(),
		HealthPath: filepath.Join(r.updateDir, "health-"+r.offer.TaskID),
		ResultPath: filepath.Join(r.updateDir, "last-result.json"), CreatedAt: time.Now().UTC(),
	}
	statePath := filepath.Join(r.updateDir, "pending.json")
	_ = os.Remove(state.HealthPath)
	_ = os.Remove(state.ResultPath)
	if err := writeJSONAtomic(statePath, state, 0o600); err != nil {
		return err
	}
	// The verified candidate runs the helper so the old executable never has
	// to replace itself while it is still mapped (which Windows forbids).
	cmd := exec.Command(r.candidate, "apply-update", "--state", statePath)
	prepareDetachedCommand(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start upgrade helper: %w", err)
	}
	return ErrApplyStarted
}

func (r *Receiver) MarkHealthy() {
	r.mu.Lock()
	defer r.mu.Unlock()

	statePath := filepath.Join(r.updateDir, "pending.json")
	var state ApplyState
	if readJSON(statePath, &state) != nil || state.Target != r.executable || state.ExpectedVersion != r.opts.Version {
		return
	}
	_ = writeJSONAtomic(state.HealthPath, map[string]any{"task_id": state.TaskID, "healthy_at": time.Now().UTC()}, 0o600)
	time.AfterFunc(5*time.Second, r.cleanupStaleUpdateFiles)
}

func (r *Receiver) PendingResult() *pb.UpgradeStatusReport {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pendingResultLocked()
}

func (r *Receiver) pendingResultLocked() *pb.UpgradeStatusReport {
	resultPath := filepath.Join(r.updateDir, "last-result.json")
	var result applyResult
	if readJSON(resultPath, &result) != nil || result.TaskID == "" {
		return nil
	}
	return &pb.UpgradeStatusReport{TaskID: result.TaskID, State: result.State, Error: result.Error}
}

func (r *Receiver) AcknowledgeResult(taskID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := r.pendingResultLocked()
	if result == nil || result.TaskID != taskID {
		return
	}
	_ = os.Remove(filepath.Join(r.updateDir, "last-result.json"))
	time.AfterFunc(5*time.Second, r.cleanupStaleUpdateFiles)
}

func (r *Receiver) cleanupStaleUpdateFiles() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cleanupStaleUpdateFilesLocked()
}

func (r *Receiver) cleanupStaleUpdateFilesLocked() {
	var pending ApplyState
	pendingPath := filepath.Join(r.updateDir, "pending.json")
	hasPending := readJSON(pendingPath, &pending) == nil
	_, resultErr := os.Stat(filepath.Join(r.updateDir, "last-result.json"))
	protectPending := hasPending && errors.Is(resultErr, os.ErrNotExist)
	entries, err := os.ReadDir(r.updateDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasPrefix(entry.Name(), "candidate-") {
				continue
			}
			path := filepath.Join(r.updateDir, entry.Name())
			// A health/result cleanup timer can fire while a later upgrade is being
			// downloaded. Keep the receiver's current candidate even before
			// pending.json exists, including the verified-to-apply handoff window.
			if r.offer != nil && r.candidate != "" && filepath.Clean(path) == filepath.Clean(r.candidate) {
				continue
			}
			if protectPending && filepath.Clean(path) == filepath.Clean(pending.Candidate) {
				continue
			}
			_ = os.Remove(path)
		}
	}
	if protectPending {
		return
	}
	base := filepath.Base(r.executable)
	targetEntries, err := os.ReadDir(filepath.Dir(r.executable))
	if err != nil {
		return
	}
	for _, entry := range targetEntries {
		if entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), base+".backup-") || strings.HasPrefix(entry.Name(), base+".failed-") {
			_ = os.Remove(filepath.Join(filepath.Dir(r.executable), entry.Name()))
		}
	}
}

func (r *Receiver) saveDownloadState() error {
	return writeJSONAtomic(filepath.Join(r.updateDir, "download.json"), downloadState{
		TaskID: r.offer.TaskID, Version: r.offer.Version, Platform: r.offer.Platform,
		Size: r.offer.Size, SHA256: r.offer.SHA256, Candidate: r.candidate, Offset: r.offset,
	}, 0o600)
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func writeJSONAtomic(path string, value any, mode os.FileMode) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := atomicReplace(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
