package clientbuild

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/pizixi/gpipe/internal/client"
	"github.com/pizixi/gpipe/internal/clientbin"
	"github.com/pizixi/gpipe/internal/model"
)

// Options 控制客户端下载构建器的运行方式。
// 发布环境通常只需要 TemplateDir 和 ArtifactCacheDir。
type Options struct {
	TemplateDir      string
	ArtifactCacheDir string
	GoBinary         string
	RepoRoot         string
}

// Target 描述一个可下载的客户端平台版本。
type Target struct {
	ID           string
	GOOS         string
	GOARCH       string
	GOARM        string
	Filename     string
	TemplateName string
}

// Artifact 是最终返回给浏览器下载的二进制产物。
type Artifact struct {
	Filename string
	Data     []byte
}

// Builder 统一抽象客户端下载构建过程，便于 Web 层替换实现或测试注入。
type Builder interface {
	Build(ctx context.Context, player model.User, settings model.ClientBuildSettings, targetID string) (*Artifact, error)
}

// RuntimeBuilder 支持“模板补丁优先，源码编译回退”的双模式构建。
type RuntimeBuilder struct {
	options Options
}

var supportedTargets = []Target{
	{
		ID:           "windows-amd64",
		GOOS:         "windows",
		GOARCH:       "amd64",
		Filename:     "gpipe-client.exe",
		TemplateName: "gpipe-client-template-windows-amd64.exe",
	},
	{
		ID:           "windows-arm64",
		GOOS:         "windows",
		GOARCH:       "arm64",
		Filename:     "gpipe-client.exe",
		TemplateName: "gpipe-client-template-windows-arm64.exe",
	},
	{
		ID:           "linux-amd64",
		GOOS:         "linux",
		GOARCH:       "amd64",
		Filename:     "gpipe-client",
		TemplateName: "gpipe-client-template-linux-amd64",
	},
	{
		ID:           "linux-arm64",
		GOOS:         "linux",
		GOARCH:       "arm64",
		Filename:     "gpipe-client",
		TemplateName: "gpipe-client-template-linux-arm64",
	},
	{
		ID:           "linux-armv7",
		GOOS:         "linux",
		GOARCH:       "arm",
		GOARM:        "7",
		Filename:     "gpipe-client",
		TemplateName: "gpipe-client-template-linux-armv7",
	},
}

// NewBuilder 创建一个支持模板目录和缓存目录的运行时构建器。
func NewBuilder(options Options) *RuntimeBuilder {
	return &RuntimeBuilder{options: options}
}

// NewGoBuilder 保留旧入口，默认不传任何额外选项。
func NewGoBuilder() *RuntimeBuilder {
	return NewBuilder(Options{})
}

// SupportedTargets 返回后台可供前端展示的目标平台列表。
func SupportedTargets() []Target {
	return slices.Clone(supportedTargets)
}

// LookupTarget 根据前端传入的 target id 查找平台定义。
func LookupTarget(id string) (Target, bool) {
	for _, target := range supportedTargets {
		if target.ID == strings.TrimSpace(id) {
			return target, true
		}
	}
	return Target{}, false
}

// ValidateSettings 校验后台“客户端设置”页面保存的生成参数是否合法。
// 这里会同时约束模板模式和回退编译模式都需要满足的公共规则。
func ValidateSettings(settings model.ClientBuildSettings) error {
	settings = normalizeSettings(settings)
	if settings.Server == "" {
		return errors.New("server address is required")
	}
	rawURIs := splitServerURIs(settings.Server)
	if len(rawURIs) == 0 {
		return errors.New("at least one server address is required")
	}
	for _, raw := range rawURIs {
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("invalid server uri %q: %w", raw, err)
		}
		switch strings.ToLower(strings.TrimSpace(u.Scheme)) {
		case "tcp", "ws", "quic", "kcp":
		default:
			return fmt.Errorf("unsupported server scheme %q", u.Scheme)
		}
		if strings.TrimSpace(u.Host) == "" {
			return fmt.Errorf("server uri %q is missing host", raw)
		}
		if u.Scheme == "quic" && !settings.EnableTLS {
			return errors.New("quic server requires TLS to be enabled")
		}
	}
	if !settings.UseShadowsocks {
		return nil
	}
	if settings.SSServer == "" || settings.SSPassword == "" {
		return errors.New("shadowsocks server and password are required")
	}
	if settings.SSMethod == "" {
		return errors.New("shadowsocks method is required")
	}
	if err := client.ValidateStreamDialServerList(settings.Server); err != nil {
		return err
	}
	return nil
}

// Build 为指定玩家和平台生成一个可下载的客户端。
// 优先走模板补丁；只有模板缺失时才回退到 go build。
func (b *RuntimeBuilder) Build(ctx context.Context, player model.User, settings model.ClientBuildSettings, targetID string) (*Artifact, error) {
	target, ok := LookupTarget(targetID)
	if !ok {
		return nil, fmt.Errorf("unsupported client target %q", targetID)
	}
	if err := ValidateSettings(settings); err != nil {
		return nil, err
	}

	encodedConfig, err := clientbin.Encode(clientbin.EmbeddedConfig{
		Server:         settings.Server,
		Key:            player.Key,
		EnableTLS:      settings.EnableTLS,
		TLSServerName:  settings.TLSServerName,
		UseShadowsocks: settings.UseShadowsocks,
		SSServer:       settings.SSServer,
		SSMethod:       settings.SSMethod,
		SSPassword:     settings.SSPassword,
	})
	if err != nil {
		return nil, err
	}

	if templatePath, ok := b.findTemplatePath(target); ok {
		templateData, err := os.ReadFile(templatePath)
		if err != nil {
			return nil, err
		}

		// 缓存键同时包含模板内容和内置配置，避免旧模板或旧参数命中错误产物。
		cacheKey := artifactCacheKey(target.ID, templateData, encodedConfig)
		if data, ok, err := b.loadCachedArtifact(target, cacheKey); err == nil && ok {
			return &Artifact{Filename: downloadFilename(player.ID, target), Data: data}, nil
		}

		patched, err := patchTemplateBinary(templateData, encodedConfig)
		if err != nil {
			return nil, err
		}
		_ = b.storeCachedArtifact(target, cacheKey, patched)
		return &Artifact{
			Filename: downloadFilename(player.ID, target),
			Data:     patched,
		}, nil
	}

	return b.compileWithGoBuild(ctx, player, target, encodedConfig)
}

// patchTemplateBinary 把模板二进制里的固定占位串替换成真实配置。
func patchTemplateBinary(templateData []byte, encodedConfig string) ([]byte, error) {
	placeholder := []byte(clientbin.PlaceholderValue())
	if len(encodedConfig) > len(placeholder) {
		return nil, fmt.Errorf("embedded config payload is too large: %d > %d", len(encodedConfig), len(placeholder))
	}

	count := bytes.Count(templateData, placeholder)
	if count == 0 {
		if bytes.Contains(templateData, []byte("__GPIPE_EMBEDDED_CLIENT_CONFIG_BEGIN__")) {
			return nil, errors.New("embedded config placeholder not found in client template; rebuild templates so the linker injects the full placeholder payload")
		}
		return nil, errors.New("embedded config placeholder not found in client template")
	}

	replacement := []byte(encodedConfig + strings.Repeat(" ", len(placeholder)-len(encodedConfig)))
	patched := append([]byte(nil), templateData...)
	searchFrom := 0
	for {
		index := bytes.Index(patched[searchFrom:], placeholder)
		if index < 0 {
			break
		}
		index += searchFrom
		copy(patched[index:index+len(placeholder)], replacement)
		searchFrom = index + len(placeholder)
	}
	return patched, nil
}

// compileWithGoBuild 是开发环境兜底逻辑。
// 当发布目录里没有模板时，仍可依赖源码和 Go 工具链现场编译。
func (b *RuntimeBuilder) compileWithGoBuild(ctx context.Context, player model.User, target Target, encodedConfig string) (*Artifact, error) {
	repoRoot, err := b.resolveRepoRoot()
	if err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "gpipe-client-build-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	outputPath := filepath.Join(tmpDir, target.Filename)
	args := []string{
		"build",
		"-trimpath",
		"-buildvcs=false",
		"-ldflags",
		fmt.Sprintf(
			"-s -w -X main.embeddedClientConfig=%s",
			encodedConfig,
		),
		"-o",
		outputPath,
		"./cmd/client",
	}
	cmd := exec.CommandContext(ctx, b.resolveGoBinary(), args...)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS="+target.GOOS,
		"GOARCH="+target.GOARCH,
	)
	if target.GOARM != "" {
		cmd.Env = append(cmd.Env, "GOARM="+target.GOARM)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("build client failed: %s", message)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, err
	}
	return &Artifact{
		Filename: downloadFilename(player.ID, target),
		Data:     data,
	}, nil
}

// loadCachedArtifact 从缓存目录读取已经补丁完成的二进制。
func (b *RuntimeBuilder) loadCachedArtifact(target Target, cacheKey string) ([]byte, bool, error) {
	cacheDir := b.resolveArtifactCacheDir()
	if cacheDir == "" {
		return nil, false, nil
	}
	path := filepath.Join(cacheDir, artifactCacheFilename(target, cacheKey))
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

// storeCachedArtifact 把专属客户端下载结果落盘，减少重复生成成本。
func (b *RuntimeBuilder) storeCachedArtifact(target Target, cacheKey string, data []byte) error {
	cacheDir := b.resolveArtifactCacheDir()
	if cacheDir == "" {
		return nil
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(cacheDir, artifactCacheFilename(target, cacheKey))
	return os.WriteFile(path, data, 0o755)
}

// artifactCacheKey 同时基于目标平台、模板内容和真实配置生成稳定摘要。
func artifactCacheKey(targetID string, templateData []byte, encodedConfig string) string {
	sum := sha256.New()
	_, _ = sum.Write([]byte(targetID))
	_, _ = sum.Write([]byte{0})
	_, _ = sum.Write(templateData)
	_, _ = sum.Write([]byte{0})
	_, _ = sum.Write([]byte(encodedConfig))
	return hex.EncodeToString(sum.Sum(nil))
}

// artifactCacheFilename 复用模板扩展名，保证 Windows/Linux 产物后缀正确。
func artifactCacheFilename(target Target, cacheKey string) string {
	ext := filepath.Ext(target.TemplateName)
	return cacheKey + ext
}

// resolveArtifactCacheDir 返回可选的缓存目录；未配置时表示禁用缓存。
func (b *RuntimeBuilder) resolveArtifactCacheDir() string {
	if strings.TrimSpace(b.options.ArtifactCacheDir) != "" {
		return b.options.ArtifactCacheDir
	}
	return ""
}

// findTemplatePath 在候选目录里查找当前目标平台对应的模板文件。
func (b *RuntimeBuilder) findTemplatePath(target Target) (string, bool) {
	for _, dir := range b.templateDirCandidates() {
		path := filepath.Join(dir, target.TemplateName)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

// templateDirCandidates 组合工作目录和可执行文件目录下的常见模板位置。
func (b *RuntimeBuilder) templateDirCandidates() []string {
	if strings.TrimSpace(b.options.TemplateDir) != "" {
		return []string{b.options.TemplateDir}
	}

	var candidates []string
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates,
			filepath.Join(cwd, "client-templates"),
			filepath.Join(cwd, "dist", "client-templates"),
		)
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(exeDir, "client-templates"),
			filepath.Join(exeDir, "dist", "client-templates"),
		)
	}

	seen := make(map[string]struct{}, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

// resolveGoBinary 返回回退编译时使用的 go 可执行文件。
func (b *RuntimeBuilder) resolveGoBinary() string {
	if strings.TrimSpace(b.options.GoBinary) != "" {
		return b.options.GoBinary
	}
	return "go"
}

// resolveRepoRoot 尝试定位源码根目录，仅在需要回退编译时使用。
func (b *RuntimeBuilder) resolveRepoRoot() (string, error) {
	if strings.TrimSpace(b.options.RepoRoot) != "" {
		return b.options.RepoRoot, nil
	}
	candidates := make([]string, 0, 4)
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Dir(file))
	}
	for _, candidate := range candidates {
		if root, ok := findRepoRoot(candidate); ok {
			b.options.RepoRoot = root
			return root, nil
		}
	}
	return "", errors.New("cannot locate project source root for fallback client build; configure client_template_dir or ship prebuilt client templates")
}

// findRepoRoot 从起始目录向上查找仓库根目录。
func findRepoRoot(start string) (string, bool) {
	dir := filepath.Clean(start)
	for {
		if fileExists(filepath.Join(dir, "go.mod")) && fileExists(filepath.Join(dir, "cmd", "client", "main.go")) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// fileExists 用于区分候选路径是否存在且为普通文件。
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// splitServerURIs 把逗号分隔的服务端地址列表拆成单项。
func splitServerURIs(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// normalizeSettings 对设置做统一裁剪和默认值补齐，避免后续重复判空。
func normalizeSettings(settings model.ClientBuildSettings) model.ClientBuildSettings {
	settings.Server = strings.TrimSpace(settings.Server)
	settings.TLSServerName = strings.TrimSpace(settings.TLSServerName)
	settings.SSServer = strings.TrimSpace(settings.SSServer)
	settings.SSMethod = strings.TrimSpace(settings.SSMethod)
	settings.SSPassword = strings.TrimSpace(settings.SSPassword)
	if settings.SSMethod == "" {
		settings.SSMethod = clientbin.DefaultShadowsocksMethod
	}
	if !settings.UseShadowsocks {
		settings.SSServer = ""
		settings.SSMethod = clientbin.DefaultShadowsocksMethod
		settings.SSPassword = ""
	}
	return settings
}

// downloadFilename 统一生成最终下载文件名，便于前端直接保存。
func downloadFilename(playerID uint32, target Target) string {
	ext := ""
	if target.GOOS == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("gpipe-client-%d-%s%s", playerID, target.ID, ext)
}
