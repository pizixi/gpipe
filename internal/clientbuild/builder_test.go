package clientbuild

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pizixi/gpipe/internal/clientbin"
	"github.com/pizixi/gpipe/internal/model"
)

// 验证模板补丁逻辑会正确替换掉内置占位串。
func TestPatchTemplateBinaryReplacesPlaceholder(t *testing.T) {
	template := append([]byte("prefix"), []byte(clientbin.PlaceholderValue())...)
	template = append(template, []byte("suffix")...)

	patched, err := patchTemplateBinary(template, "abc123")
	if err != nil {
		t.Fatalf("patchTemplateBinary() error = %v", err)
	}
	if bytes.Contains(patched, []byte(clientbin.PlaceholderValue())) {
		t.Fatalf("patched template should not contain placeholder bytes")
	}

	index := bytes.Index(patched, []byte("prefix"))
	if index < 0 {
		t.Fatalf("prefix not found")
	}
	start := index + len("prefix")
	end := start + len(clientbin.PlaceholderValue())
	value := string(bytes.TrimSpace(patched[start:end]))
	if value != "abc123" {
		t.Fatalf("patched embedded value = %q, want %q", value, "abc123")
	}
}

// 验证模板模式下会优先生成专属客户端，并把结果写入缓存目录。
func TestRuntimeBuilderBuildUsesTemplateAndWritesCache(t *testing.T) {
	tmpDir := t.TempDir()
	templateDir := filepath.Join(tmpDir, "client-templates")
	cacheDir := filepath.Join(tmpDir, "client-cache")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("mkdir template dir: %v", err)
	}

	target, ok := LookupTarget("linux-amd64")
	if !ok {
		t.Fatalf("target not found")
	}
	templatePath := filepath.Join(templateDir, target.TemplateName)
	templateData := append([]byte("template-begin"), []byte(clientbin.PlaceholderValue())...)
	templateData = append(templateData, []byte("template-end")...)
	if err := os.WriteFile(templatePath, templateData, 0o755); err != nil {
		t.Fatalf("write template: %v", err)
	}

	builder := NewBuilder(Options{
		TemplateDir:      templateDir,
		ArtifactCacheDir: cacheDir,
	})
	artifact, err := builder.Build(context.Background(), model.User{
		ID:  1001,
		Key: "player-secret",
	}, model.ClientBuildSettings{
		Server: "tcp://127.0.0.1:8118",
	}, target.ID)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if artifact.Filename != "gpipe-client-1001-linux-amd64" {
		t.Fatalf("filename = %q, want %q", artifact.Filename, "gpipe-client-1001-linux-amd64")
	}
	if bytes.Contains(artifact.Data, []byte(clientbin.PlaceholderValue())) {
		t.Fatalf("artifact data should not contain placeholder bytes")
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected cached artifact to be written")
	}
}

// 验证模板构建命令通过 -ldflags 注入完整占位串，服务端后续才能在二进制里定位并替换。
func TestGoBuildTemplateContainsPatchablePlaceholder(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime caller unavailable")
	}

	repoRoot, ok := findRepoRoot(filepath.Dir(file))
	if !ok {
		t.Fatalf("repo root not found")
	}

	outputPath := filepath.Join(t.TempDir(), "gpipe-client-template")
	if runtime.GOOS == "windows" {
		outputPath += ".exe"
	}
	ldflags := "-s -w -X main.embeddedClientConfig=" + clientbin.PlaceholderValue()
	cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags", ldflags, "-o", outputPath, "./cmd/client")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build template failed: %v\n%s", err, strings.TrimSpace(string(output)))
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read built template: %v", err)
	}
	if bytes.Count(data, []byte(clientbin.PlaceholderValue())) == 0 {
		t.Fatalf("expected built template to contain full embedded placeholder bytes")
	}

	patched, err := patchTemplateBinary(data, "abc123")
	if err != nil {
		t.Fatalf("patchTemplateBinary() error = %v", err)
	}
	if bytes.Contains(patched, []byte(clientbin.PlaceholderValue())) {
		t.Fatalf("patched template should not contain placeholder bytes")
	}
}
