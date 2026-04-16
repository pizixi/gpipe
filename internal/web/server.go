package web

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pizixi/gpipe"

	"github.com/pizixi/gpipe/internal/clientbuild"
	"github.com/pizixi/gpipe/internal/config"
	"github.com/pizixi/gpipe/internal/manager"
	"github.com/pizixi/gpipe/internal/model"
)

const authCookieName = "auth-id"
const authCookieTTL = 7 * 24 * time.Hour
const authCookieRefreshThreshold = 24 * time.Hour
const maxJSONBodyBytes int64 = 1 << 20

const webIndexTemplateName = "layout/page"

type Service struct {
	cfg          *config.ServerConfig
	rt           *manager.Runtime
	cookieSecret []byte
	// clientBuilder 负责按玩家和目标平台生成可下载的客户端二进制。
	clientBuilder clientbuild.Builder
}

// NewService 创建 Web 管理端服务，并把客户端下载构建器接入运行时配置。
func NewService(cfg *config.ServerConfig, rt *manager.Runtime) *Service {
	return &Service{
		cfg:          cfg,
		rt:           rt,
		cookieSecret: newCookieSecret(cfg),
		clientBuilder: clientbuild.NewBuilder(clientbuild.Options{
			TemplateDir:      cfg.ClientTemplateDir,
			ArtifactCacheDir: cfg.ClientArtifactCacheDir,
		}),
	}
}

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", s.login)
	mux.HandleFunc("/api/logout", s.logout)
	mux.HandleFunc("/api/test_auth", s.testAuth)
	mux.HandleFunc("/api/client_build_settings", s.clientBuildSettings)
	mux.HandleFunc("/api/update_client_build_settings", s.updateClientBuildSettings)
	mux.HandleFunc("/api/player_list", s.playerList)
	mux.HandleFunc("/api/remove_player", s.removePlayer)
	mux.HandleFunc("/api/add_player", s.addPlayer)
	mux.HandleFunc("/api/update_player", s.updatePlayer)
	mux.HandleFunc("/api/generate_client", s.generateClient)
	mux.HandleFunc("/api/tunnel_list", s.tunnelList)
	mux.HandleFunc("/api/remove_tunnel", s.removeTunnel)
	mux.HandleFunc("/api/add_tunnel", s.addTunnel)
	mux.HandleFunc("/api/update_tunnel", s.updateTunnel)
	mux.Handle("/", s.staticHandler())
	return withCORS(mux)
}

func (s *Service) staticHandler() http.Handler {
	if s.cfg.WebBaseDir != "" {
		if stat, err := os.Stat(s.cfg.WebBaseDir); err == nil && stat.IsDir() {
			return newWebUIHandler(os.DirFS(s.cfg.WebBaseDir), true)
		}
	}
	// Try the new React SPA build output (webui/dist/) first, fall back to webui/.
	if subFS, err := fs.Sub(gpipe.EmbeddedWebFS, "webui/dist"); err == nil {
		if _, statErr := fs.Stat(subFS, "index.html"); statErr == nil {
			return newWebUIHandler(subFS, false)
		}
	}
	subFS, err := fs.Sub(gpipe.EmbeddedWebFS, "webui")
	if err != nil {
		return http.NotFoundHandler()
	}
	return newWebUIHandler(subFS, false)
}

func newWebUIHandler(webFS fs.FS, reloadTemplates bool) http.Handler {
	fileServer := http.FileServer(http.FS(webFS))
	var cachedTemplate *template.Template
	var cachedTemplateErr error
	if !reloadTemplates {
		cachedTemplate, cachedTemplateErr = parseWebIndexTemplate(webFS)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			// Serve index.html for root path explicitly.
			if r.URL.Path == "/" || r.URL.Path == "/index.html" {
				serveIndexHTML(w, r, webFS, cachedTemplate, cachedTemplateErr, reloadTemplates)
				return
			}
			// For non-API paths, check if the file exists; if not, serve index.html (SPA fallback).
			if !strings.HasPrefix(r.URL.Path, "/api") {
				cleanPath := strings.TrimPrefix(r.URL.Path, "/")
				if _, err := fs.Stat(webFS, cleanPath); err != nil {
					// Missing asset-like paths should stay 404 instead of falling back to index.html,
					// otherwise browsers may cache HTML as JS/CSS/favicon responses.
					if path.Ext(cleanPath) != "" {
						http.NotFound(w, r)
						return
					}
					serveIndexHTML(w, r, webFS, cachedTemplate, cachedTemplateErr, reloadTemplates)
					return
				}
			}
		}
		fileServer.ServeHTTP(w, r)
	})
}

func serveIndexHTML(w http.ResponseWriter, r *http.Request, webFS fs.FS, cachedTemplate *template.Template, cachedTemplateErr error, reloadTemplates bool) {
	if data, err := fs.ReadFile(webFS, "index.html"); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(data))
		return
	}
	indexHTML, err := renderWebIndexTemplate(webFS, cachedTemplate, cachedTemplateErr, reloadTemplates)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(indexHTML))
}

func renderWebIndexTemplate(webFS fs.FS, cachedTemplate *template.Template, cachedTemplateErr error, reloadTemplates bool) ([]byte, error) {
	tmpl := cachedTemplate
	if reloadTemplates {
		parsed, err := parseWebIndexTemplate(webFS)
		if err != nil {
			return nil, fmt.Errorf("render web ui template: %w", err)
		}
		tmpl = parsed
	} else if cachedTemplateErr != nil {
		return nil, fmt.Errorf("render web ui template: %w", cachedTemplateErr)
	}

	var rendered bytes.Buffer
	if err := tmpl.ExecuteTemplate(&rendered, webIndexTemplateName, nil); err != nil {
		return nil, fmt.Errorf("render web ui template %q: %w", webIndexTemplateName, err)
	}
	return rendered.Bytes(), nil
}

func parseWebIndexTemplate(webFS fs.FS) (*template.Template, error) {
	files, err := listWebTemplateFiles(webFS)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New("webui").ParseFS(webFS, files...)
	if err != nil {
		return nil, fmt.Errorf("parse web ui templates: %w", err)
	}
	if tmpl.Lookup(webIndexTemplateName) == nil {
		return nil, fmt.Errorf("web ui template %q not found", webIndexTemplateName)
	}
	return tmpl, nil
}

func listWebTemplateFiles(webFS fs.FS) ([]string, error) {
	files := make([]string, 0, 8)
	if err := fs.WalkDir(webFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".tmpl") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("list web ui templates: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("list web ui templates: no templates found")
	}
	sort.Strings(files)
	return files, nil
}

func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	var req LoginReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if s.cfg.WebUsername == req.Username && s.cfg.WebPassword == req.Password && req.Username != "" {
		s.issueAuthCookie(w)
		writeJSON(w, http.StatusOK, GeneralResponse{Code: 0, Msg: "Success"})
		return
	}
	writeJSON(w, http.StatusOK, GeneralResponse{Code: -2, Msg: "Incorrect username or password"})
}

func (s *Service) logout(w http.ResponseWriter, _ *http.Request) {
	s.clearAuthCookie(w)
	writeJSON(w, http.StatusOK, GeneralResponse{Code: 10086, Msg: "Session expired, please log in again."})
}

func (s *Service) testAuth(w http.ResponseWriter, r *http.Request) {
	id, ok := s.authenticateAndRefresh(w, r)
	if !ok {
		writeJSON(w, http.StatusOK, GeneralResponse{Code: 10086, Msg: "Session expired, please log in again."})
		return
	}
	writeJSON(w, http.StatusOK, GeneralResponse{Code: 0, Msg: "hello " + id})
}

func (s *Service) clientBuildSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	// 客户端设置是一个单例配置，前端直接整块读取即可。
	settings, err := s.rt.ClientBuildSettings.Get()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, GeneralResponse{Code: -1, Msg: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ClientBuildSettingsResponse{
		Settings: ClientBuildSettingsPayload{
			Server:         settings.Server,
			EnableTLS:      settings.EnableTLS,
			TLSServerName:  settings.TLSServerName,
			UseShadowsocks: settings.UseShadowsocks,
			SSServer:       settings.SSServer,
			SSMethod:       settings.SSMethod,
			SSPassword:     settings.SSPassword,
		},
	})
}

func (s *Service) updateClientBuildSettings(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var req ClientBuildSettingsPayload
	if !decodeJSON(w, r, &req) {
		return
	}
	settings := model.ClientBuildSettings{
		Server:         req.Server,
		EnableTLS:      req.EnableTLS,
		TLSServerName:  req.TLSServerName,
		UseShadowsocks: req.UseShadowsocks,
		SSServer:       req.SSServer,
		SSMethod:       req.SSMethod,
		SSPassword:     req.SSPassword,
	}
	if err := clientbuild.ValidateSettings(settings); err != nil {
		writeJSON(w, http.StatusOK, GeneralResponse{Code: -2, Msg: err.Error()})
		return
	}
	// 只有校验通过后才持久化，避免前端保存一份无法用于生成客户端的坏配置。
	if err := s.rt.ClientBuildSettings.Save(settings); err != nil {
		writeJSON(w, http.StatusInternalServerError, GeneralResponse{Code: -1, Msg: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, GeneralResponse{Code: 0, Msg: "Success"})
}

func (s *Service) playerList(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var req PlayerListRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.PageNumber < 0 {
		req.PageNumber = 0
	}
	var (
		users []model.User
		total int
		err   error
	)
	switch {
	case req.PageSize == 0:
		users, err = s.rt.Players.All()
		total = len(users)
	default:
		pageSize := req.PageSize
		if pageSize > 100 {
			pageSize = 100
		}
		users, total, err = s.rt.Players.List(req.PageNumber, pageSize)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, GeneralResponse{Code: -1, Msg: err.Error()})
		return
	}
	players := make([]PlayerListItem, 0, len(users))
	for _, user := range users {
		players = append(players, PlayerListItem{
			ID:         user.ID,
			Remark:     user.Remark,
			Key:        user.Key,
			CreateTime: user.CreateTime,
			Online:     s.rt.Players.IsOnline(user.ID),
		})
	}
	writeJSON(w, http.StatusOK, PlayerListResponse{
		Players:       players,
		CurPageNumber: req.PageNumber,
		TotalCount:    total,
	})
}

func (s *Service) removePlayer(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var req PlayerRemoveReq
	if !decodeJSON(w, r, &req) {
		return
	}
	for _, tunnel := range s.rt.Tunnel.ByPlayer(req.ID) {
		if err := s.rt.Tunnel.Delete(tunnel.ID); err != nil {
			writeJSON(w, http.StatusOK, GeneralResponse{Code: -1, Msg: err.Error()})
			return
		}
	}
	if err := s.rt.Players.Delete(req.ID); err != nil {
		writeJSON(w, http.StatusOK, GeneralResponse{Code: -1, Msg: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, GeneralResponse{Code: 0, Msg: "Success"})
}

func (s *Service) generateClient(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var req GenerateClientReq
	if !decodeJSON(w, r, &req) {
		return
	}
	player, err := s.rt.Users.FindByID(req.PlayerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, GeneralResponse{Code: -1, Msg: err.Error()})
		return
	}
	if player == nil {
		writeJSON(w, http.StatusOK, GeneralResponse{Code: -2, Msg: "player not found"})
		return
	}
	// 生成时读取的是最新的后台设置，确保下载内容和当前页面保存值一致。
	settings, err := s.rt.ClientBuildSettings.Get()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, GeneralResponse{Code: -1, Msg: err.Error()})
		return
	}
	artifact, err := s.clientBuilder.Build(r.Context(), *player, settings, req.Target)
	if err != nil {
		writeJSON(w, http.StatusOK, GeneralResponse{Code: -3, Msg: err.Error()})
		return
	}
	// 直接回二进制流给浏览器，前端无需再额外拼接下载地址。
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, artifact.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(artifact.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(artifact.Data)
}

func (s *Service) addPlayer(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var req PlayerAddReq
	if !decodeJSON(w, r, &req) {
		return
	}
	code, msg, _, err := s.rt.Players.Add(req.Remark, req.Key)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, GeneralResponse{Code: -1, Msg: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, GeneralResponse{Code: code, Msg: msg})
}

func (s *Service) updatePlayer(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var req PlayerUpdateReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.rt.Players.Update(model.PlayerUpdate{ID: req.ID, Remark: req.Remark, Key: req.Key}); err != nil {
		writeJSON(w, http.StatusOK, GeneralResponse{Code: -2, Msg: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, GeneralResponse{Code: 0, Msg: "Success"})
}

func (s *Service) tunnelList(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var req TunnelListRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.PageNumber < 0 {
		req.PageNumber = 0
	}
	var tunnels []model.Tunnel
	totalCount := 0
	playerID := strings.TrimSpace(req.PlayerID)
	switch {
	case playerID != "":
		id, err := strconv.ParseUint(playerID, 10, 32)
		if err != nil {
			writeJSON(w, http.StatusOK, TunnelListResponse{
				Tunnels:       []TunnelListItem{},
				CurPageNumber: req.PageNumber,
				TotalCount:    0,
			})
			return
		}
		tunnels = s.rt.Tunnel.ByPlayer(uint32(id))
		totalCount = len(tunnels)
	case req.PageSize == 0:
		tunnels = s.rt.Tunnel.All()
		totalCount = len(tunnels)
	default:
		tunnels = s.rt.Tunnel.Query(req.PageNumber, req.PageSize)
		totalCount = s.rt.Tunnel.Count()
	}
	items := make([]TunnelListItem, 0, len(tunnels))
	for _, tunnel := range tunnels {
		items = append(items, TunnelListItem{
			ID:               tunnel.ID,
			Source:           tunnel.Source,
			Endpoint:         tunnel.Endpoint,
			Enabled:          tunnel.Enabled,
			Sender:           tunnel.Sender,
			Receiver:         tunnel.Receiver,
			Description:      tunnel.Description,
			TunnelType:       tunnel.TunnelType,
			Password:         tunnel.Password,
			Username:         tunnel.Username,
			IsCompressed:     tunnel.IsCompressed,
			EncryptionMethod: tunnel.EncryptionMethod,
			CustomMapping:    tunnel.CustomMapping,
		})
	}
	writeJSON(w, http.StatusOK, TunnelListResponse{
		Tunnels:       items,
		CurPageNumber: req.PageNumber,
		TotalCount:    totalCount,
	})
}

func (s *Service) removeTunnel(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var req TunnelRemoveReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.rt.Tunnel.Delete(req.ID); err != nil {
		writeJSON(w, http.StatusOK, GeneralResponse{Code: -1, Msg: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, GeneralResponse{Code: 0, Msg: "Success"})
}

func (s *Service) addTunnel(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var req TunnelAddReq
	if !decodeJSON(w, r, &req) {
		return
	}
	_, err := s.rt.Tunnel.Add(model.Tunnel{
		Source:           req.Source,
		Endpoint:         req.Endpoint,
		Enabled:          req.Enabled == 1,
		Sender:           req.Sender,
		Receiver:         req.Receiver,
		Description:      req.Description,
		TunnelType:       req.TunnelType,
		Password:         req.Password,
		Username:         req.Username,
		IsCompressed:     req.IsCompressed == 1,
		EncryptionMethod: req.EncryptionMethod,
		CustomMapping:    req.CustomMapping,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, GeneralResponse{Code: -1, Msg: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, GeneralResponse{Code: 0, Msg: "Success"})
}

func (s *Service) updateTunnel(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuth(w, r) {
		return
	}
	var req TunnelUpdateReq
	if !decodeJSON(w, r, &req) {
		return
	}
	err := s.rt.Tunnel.Update(model.Tunnel{
		ID:               req.ID,
		Source:           req.Source,
		Endpoint:         req.Endpoint,
		Enabled:          req.Enabled == 1,
		Sender:           req.Sender,
		Receiver:         req.Receiver,
		Description:      req.Description,
		TunnelType:       req.TunnelType,
		Password:         req.Password,
		Username:         req.Username,
		IsCompressed:     req.IsCompressed == 1,
		EncryptionMethod: req.EncryptionMethod,
		CustomMapping:    req.CustomMapping,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, GeneralResponse{Code: -1, Msg: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, GeneralResponse{Code: 0, Msg: "Success"})
}

func newCookieSecret(cfg *config.ServerConfig) []byte {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err == nil {
		return secret
	}
	fallback := sha256.Sum256([]byte(cfg.WebUsername + "\x00" + cfg.WebPassword + "\x00" + strconv.FormatInt(time.Now().UnixNano(), 10)))
	return append([]byte(nil), fallback[:]...)
}

func (s *Service) signedCookieValue(expiresAt time.Time) string {
	expiresUnix := strconv.FormatInt(expiresAt.Unix(), 10)
	signature := s.cookieSignature(expiresUnix)
	return expiresUnix + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func (s *Service) issueAuthCookie(w http.ResponseWriter) {
	expiresAt := time.Now().Add(authCookieTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    s.signedCookieValue(expiresAt),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   int(authCookieTTL / time.Second),
	})
}

func (s *Service) clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func (s *Service) cookieSignature(expiresUnix string) []byte {
	mac := hmac.New(sha256.New, s.cookieSecret)
	_, _ = mac.Write([]byte(s.cfg.WebUsername))
	_, _ = mac.Write([]byte{'\n'})
	_, _ = mac.Write([]byte(expiresUnix))
	return mac.Sum(nil)
}

func (s *Service) authenticated(r *http.Request) (string, time.Time, bool) {
	cookie, err := r.Cookie(authCookieName)
	if err != nil {
		return "", time.Time{}, false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", time.Time{}, false
	}
	expiresUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "", time.Time{}, false
	}
	expiresAt := time.Unix(expiresUnix, 0)
	if time.Now().After(expiresAt) {
		return "", time.Time{}, false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", time.Time{}, false
	}
	expected := s.cookieSignature(parts[0])
	if subtle.ConstantTimeCompare(signature, expected) != 1 {
		return "", time.Time{}, false
	}
	return s.cfg.WebUsername, expiresAt, true
}

func (s *Service) authenticateAndRefresh(w http.ResponseWriter, r *http.Request) (string, bool) {
	id, expiresAt, ok := s.authenticated(r)
	if !ok {
		return "", false
	}
	if time.Until(expiresAt) <= authCookieRefreshThreshold {
		s.issueAuthCookie(w)
	}
	return id, true
}

func (s *Service) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := s.authenticateAndRefresh(w, r); ok {
		return true
	}
	writeJSON(w, http.StatusOK, GeneralResponse{Code: 10086, Msg: "Session expired, please log in again."})
	return false
}

func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	defer r.Body.Close()
	body := http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, GeneralResponse{Code: -1, Msg: err.Error()})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
