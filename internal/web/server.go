package web

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pizixi/gpipe"

	"github.com/pizixi/gpipe/internal/config"
	"github.com/pizixi/gpipe/internal/manager"
	"github.com/pizixi/gpipe/internal/model"
)

const authCookieName = "auth-id"
const authCookieTTL = 60 * time.Minute
const maxJSONBodyBytes int64 = 1 << 20

type Service struct {
	cfg          *config.ServerConfig
	rt           *manager.Runtime
	cookieSecret []byte
}

func NewService(cfg *config.ServerConfig, rt *manager.Runtime) *Service {
	return &Service{
		cfg:          cfg,
		rt:           rt,
		cookieSecret: newCookieSecret(cfg),
	}
}

func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", s.login)
	mux.HandleFunc("/api/logout", s.logout)
	mux.HandleFunc("/api/test_auth", s.testAuth)
	mux.HandleFunc("/api/player_list", s.playerList)
	mux.HandleFunc("/api/remove_player", s.removePlayer)
	mux.HandleFunc("/api/add_player", s.addPlayer)
	mux.HandleFunc("/api/update_player", s.updatePlayer)
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
			return http.FileServer(http.Dir(s.cfg.WebBaseDir))
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(gpipe.EmbeddedIndexHTML))
	})
}

func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	var req LoginReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if s.cfg.WebUsername == req.Username && s.cfg.WebPassword == req.Password && req.Username != "" {
		expiresAt := time.Now().Add(authCookieTTL)
		value := s.signedCookieValue(expiresAt)
		http.SetCookie(w, &http.Cookie{
			Name:     authCookieName,
			Value:    value,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Expires:  expiresAt,
		})
		writeJSON(w, http.StatusOK, GeneralResponse{Code: 0, Msg: "Success"})
		return
	}
	writeJSON(w, http.StatusOK, GeneralResponse{Code: -2, Msg: "Incorrect username or password"})
}

func (s *Service) logout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, GeneralResponse{Code: 10086, Msg: "Session expired, please log in again."})
}

func (s *Service) testAuth(w http.ResponseWriter, r *http.Request) {
	id, ok := s.authenticated(r)
	if !ok {
		writeJSON(w, http.StatusOK, GeneralResponse{Code: 10086, Msg: "Session expired, please log in again."})
		return
	}
	writeJSON(w, http.StatusOK, GeneralResponse{Code: 0, Msg: "hello " + id})
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
			ID:     user.ID,
			Remark: user.Remark,
			Key:    user.Key,
			Online: s.rt.Players.IsOnline(user.ID),
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

func (s *Service) cookieSignature(expiresUnix string) []byte {
	mac := hmac.New(sha256.New, s.cookieSecret)
	_, _ = mac.Write([]byte(s.cfg.WebUsername))
	_, _ = mac.Write([]byte{'\n'})
	_, _ = mac.Write([]byte(expiresUnix))
	return mac.Sum(nil)
}

func (s *Service) authenticated(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(authCookieName)
	if err != nil {
		return "", false
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	expiresUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "", false
	}
	if time.Now().After(time.Unix(expiresUnix, 0)) {
		return "", false
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	expected := s.cookieSignature(parts[0])
	if subtle.ConstantTimeCompare(signature, expected) != 1 {
		return "", false
	}
	return s.cfg.WebUsername, true
}

func (s *Service) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := s.authenticated(r); ok {
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
