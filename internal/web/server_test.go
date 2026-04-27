package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pizixi/gpipe/internal/clientbuild"
	"github.com/pizixi/gpipe/internal/config"
	"github.com/pizixi/gpipe/internal/db"
	"github.com/pizixi/gpipe/internal/manager"
	"github.com/pizixi/gpipe/internal/model"
	"github.com/pizixi/gpipe/internal/proto"
	"github.com/pizixi/gpipe/internal/store"
)

type playerSessionStub struct{}

func (playerSessionStub) Close() error { return nil }

func (playerSessionStub) SendPush(message proto.Message) error {
	_ = message
	return nil
}

// clientBuilderStub 用于隔离真实构建过程，只验证 Web 层参数传递是否正确。
type clientBuilderStub struct {
	artifact     *clientbuild.Artifact
	err          error
	lastPlayer   model.User
	lastSettings model.ClientBuildSettings
	lastTarget   string
}

func (s *clientBuilderStub) Build(_ context.Context, player model.User, settings model.ClientBuildSettings, targetID string) (*clientbuild.Artifact, error) {
	s.lastPlayer = player
	s.lastSettings = settings
	s.lastTarget = targetID
	if s.err != nil {
		return nil, s.err
	}
	return s.artifact, nil
}

func TestLoginIssuesSignedCookieAcceptedByAuthCheck(t *testing.T) {
	service := NewService(&config.ServerConfig{
		WebUsername: "admin",
		WebPassword: "secret",
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"username":"admin","password":"secret"}`))
	recorder := httptest.NewRecorder()
	service.login(recorder, req)

	resp := recorder.Result()
	cookies := resp.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one auth cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != authCookieName {
		t.Fatalf("unexpected cookie name: %s", cookie.Name)
	}
	if !strings.Contains(cookie.Value, ".") {
		t.Fatalf("expected signed cookie value")
	}
	if cookie.MaxAge != int(authCookieTTL/time.Second) {
		t.Fatalf("cookie max age = %d, want %d", cookie.MaxAge, int(authCookieTTL/time.Second))
	}

	authReq := httptest.NewRequest(http.MethodGet, "/api/test_auth", nil)
	authReq.AddCookie(cookie)
	id, expiresAt, ok := service.authenticated(authReq)
	if !ok {
		t.Fatalf("expected signed cookie to authenticate")
	}
	if id != "admin" {
		t.Fatalf("authenticated id = %q, want %q", id, "admin")
	}
	if time.Until(expiresAt) < 6*24*time.Hour {
		t.Fatalf("cookie lifetime too short, expires at %v", expiresAt)
	}
}

func TestAuthenticatedRejectsTamperedOrExpiredCookie(t *testing.T) {
	service := NewService(&config.ServerConfig{
		WebUsername: "admin",
		WebPassword: "secret",
	}, nil)

	validCookie := &http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(time.Minute)),
	}
	tamperedReq := httptest.NewRequest(http.MethodGet, "/api/test_auth", nil)
	tamperedReq.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: validCookie.Value + "tampered",
	})
	if _, _, ok := service.authenticated(tamperedReq); ok {
		t.Fatalf("expected tampered cookie to be rejected")
	}

	expiredReq := httptest.NewRequest(http.MethodGet, "/api/test_auth", nil)
	expiredReq.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(-time.Minute)),
	})
	if _, _, ok := service.authenticated(expiredReq); ok {
		t.Fatalf("expected expired cookie to be rejected")
	}
}

func TestTestAuthRefreshesCookieNearExpiry(t *testing.T) {
	service := NewService(&config.ServerConfig{
		WebUsername: "admin",
		WebPassword: "secret",
	}, nil)

	nearExpiry := time.Now().Add(30 * time.Minute)
	req := httptest.NewRequest(http.MethodGet, "/api/test_auth", nil)
	req.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(nearExpiry),
	})
	recorder := httptest.NewRecorder()

	service.testAuth(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	resp := recorder.Result()
	cookies := resp.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected refreshed auth cookie, got %d", len(cookies))
	}
	refreshed := cookies[0]
	if refreshed.Expires.Before(time.Now().Add(6 * 24 * time.Hour)) {
		t.Fatalf("refreshed cookie expires too soon: %v", refreshed.Expires)
	}
	if refreshed.MaxAge != int(authCookieTTL/time.Second) {
		t.Fatalf("refreshed cookie max age = %d, want %d", refreshed.MaxAge, int(authCookieTTL/time.Second))
	}
}

func TestHandlerServesEmbeddedIndexWhenWebBaseDirIsEmpty(t *testing.T) {
	handler := NewService(&config.ServerConfig{}, nil).Handler()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	resp := recorder.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "<div id=\"root\"></div>") || !strings.Contains(body, "<script type=\"module\"") {
		t.Fatalf("expected embedded index html response")
	}
}

func TestHandlerServesEmbeddedAssetsWhenWebBaseDirIsEmpty(t *testing.T) {
	handler := NewService(&config.ServerConfig{}, nil).Handler()

	indexReq := httptest.NewRequest(http.MethodGet, "/", nil)
	indexRecorder := httptest.NewRecorder()
	handler.ServeHTTP(indexRecorder, indexReq)

	if indexRecorder.Code != http.StatusOK {
		t.Fatalf("index status = %d, want %d", indexRecorder.Code, http.StatusOK)
	}

	matches := regexp.MustCompile(`(?:src|href)="\./(assets/[^"]+\.js)"`).FindStringSubmatch(indexRecorder.Body.String())
	if len(matches) != 2 {
		t.Fatalf("expected embedded asset path in index html")
	}

	req := httptest.NewRequest(http.MethodGet, "/"+matches[1], nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	resp := recorder.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if contentType := resp.Header.Get("Content-Type"); !strings.Contains(contentType, "javascript") {
		t.Fatalf("content type = %q, want javascript asset", contentType)
	}
	if recorder.Body.Len() == 0 {
		t.Fatalf("expected embedded javascript asset response")
	}
}

func TestHandlerPrefersDiskStaticFilesWhenWebBaseDirExists(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(indexPath, []byte("custom-index"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	handler := NewService(&config.ServerConfig{
		WebBaseDir: dir,
	}, nil).Handler()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "custom-index") {
		t.Fatalf("expected disk index html to be served, got %q", body)
	}
}

func TestHandlerReturnsNotFoundForMissingStaticAsset(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(indexPath, []byte("custom-index"), 0o644); err != nil {
		t.Fatalf("write index: %v", err)
	}

	handler := NewService(&config.ServerConfig{
		WebBaseDir: dir,
	}, nil).Handler()

	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if strings.Contains(recorder.Body.String(), "custom-index") {
		t.Fatalf("missing asset should not fall back to index html")
	}
}

func TestHandlerBuildsIndexFromDiskTemplatesWhenIndexHTMLMissing(t *testing.T) {
	dir := t.TempDir()
	templates := map[string]string{
		"templates/layout/page.tmpl": `{{define "layout/page"}}<!doctype html>
<html><body>
{{template "sections/login" .}}
{{template "sections/main_content" .}}
{{template "modals/add_player" .}}
{{template "modals/edit_player" .}}
{{template "modals/generate_client" .}}
{{template "modals/add_tunnel" .}}
{{template "modals/edit_tunnel" .}}
</body></html>{{end}}`,
		"templates/sections/login.tmpl":         `{{define "sections/login"}}<section id="login">login</section>{{end}}`,
		"templates/sections/main_content.tmpl":  `{{define "sections/main_content"}}<main id="main">main</main>{{end}}`,
		"templates/modals/add_player.tmpl":      `{{define "modals/add_player"}}<div id="add-player"></div>{{end}}`,
		"templates/modals/edit_player.tmpl":     `{{define "modals/edit_player"}}<div id="edit-player"></div>{{end}}`,
		"templates/modals/generate_client.tmpl": `{{define "modals/generate_client"}}<div id="generate-client"></div>{{end}}`,
		"templates/modals/add_tunnel.tmpl":      `{{define "modals/add_tunnel"}}<div id="add-tunnel"></div>{{end}}`,
		"templates/modals/edit_tunnel.tmpl":     `{{define "modals/edit_tunnel"}}<div id="edit-tunnel"></div>{{end}}`,
	}
	for path, content := range templates {
		fullPath := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", fullPath, err)
		}
	}

	handler := NewService(&config.ServerConfig{
		WebBaseDir: dir,
	}, nil).Handler()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	for _, expected := range []string{"<!doctype html>", "id=\"login\"", "id=\"edit-tunnel\""} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected rendered disk template content %q, got %q", expected, body)
		}
	}
}

func TestTunnelListFiltersByPlayerID(t *testing.T) {
	database, err := db.Open("sqlite://file:test_web_tunnel_list?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := manager.NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, playerA, err := rt.Players.Add("alice", "secret1")
	if err != nil {
		t.Fatalf("add player alice: %v", err)
	}
	_, _, playerB, err := rt.Players.Add("bob", "secret2")
	if err != nil {
		t.Fatalf("add player bob: %v", err)
	}

	for _, tunnel := range []model.Tunnel{
		{
			Source:           "127.0.0.1:10080",
			Endpoint:         "127.0.0.1:18080",
			Enabled:          true,
			Sender:           playerA,
			Receiver:         playerB,
			Description:      "sender match",
			TunnelType:       uint32(model.TunnelTypeTCP),
			IsCompressed:     true,
			EncryptionMethod: "None",
		},
		{
			Source:           "127.0.0.1:10081",
			Endpoint:         "127.0.0.1:18081",
			Enabled:          true,
			Sender:           0,
			Receiver:         playerA,
			Description:      "receiver match",
			TunnelType:       uint32(model.TunnelTypeTCP),
			IsCompressed:     true,
			EncryptionMethod: "None",
		},
		{
			Source:           "127.0.0.1:10082",
			Endpoint:         "127.0.0.1:18082",
			Enabled:          true,
			Sender:           playerB,
			Receiver:         0,
			Description:      "no match",
			TunnelType:       uint32(model.TunnelTypeTCP),
			IsCompressed:     true,
			EncryptionMethod: "None",
		},
	} {
		if _, err := rt.Tunnel.Add(tunnel); err != nil {
			t.Fatalf("add tunnel: %v", err)
		}
	}

	service := NewService(&config.ServerConfig{
		WebUsername: "admin",
		WebPassword: "secret",
	}, rt)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/tunnel_list",
		strings.NewReader(`{"page_number":0,"page_size":0,"player_id":"`+strconv.FormatUint(uint64(playerA), 10)+`"}`),
	)
	req.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(time.Minute)),
	})
	recorder := httptest.NewRecorder()

	service.tunnelList(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var resp TunnelListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.TotalCount != 2 {
		t.Fatalf("total_count = %d, want %d", resp.TotalCount, 2)
	}
	if len(resp.Tunnels) != 2 {
		t.Fatalf("len(tunnels) = %d, want %d", len(resp.Tunnels), 2)
	}
	for _, tunnel := range resp.Tunnels {
		if tunnel.Sender != playerA && tunnel.Receiver != playerA {
			t.Fatalf("unexpected tunnel in filtered result: %+v", tunnel)
		}
	}
}

func TestTunnelListReturnsRuntimeStatus(t *testing.T) {
	database, err := db.Open("sqlite://file:test_web_tunnel_runtime_status?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := manager.NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, senderID, err := rt.Players.Add("sender", "sender-key")
	if err != nil {
		t.Fatalf("add sender: %v", err)
	}
	rt.Players.Bind(senderID, playerSessionStub{})
	tunnel, err := rt.Tunnel.Add(model.Tunnel{
		Source:           "127.0.0.1:10080",
		Endpoint:         "127.0.0.1:18080",
		Enabled:          true,
		Sender:           senderID,
		Receiver:         0,
		Description:      "server inlet failed",
		TunnelType:       uint32(model.TunnelTypeTCP),
		IsCompressed:     true,
		EncryptionMethod: "None",
	})
	if err != nil {
		t.Fatalf("add tunnel: %v", err)
	}
	rt.TunnelRuntime.SetInlet(tunnel.ID, false, "listen tcp 127.0.0.1:10080: bind: address already in use")

	service := NewService(&config.ServerConfig{
		WebUsername: "admin",
		WebPassword: "secret",
	}, rt)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/tunnel_list",
		strings.NewReader(`{"page_number":0,"page_size":0}`),
	)
	req.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(time.Minute)),
	})
	recorder := httptest.NewRecorder()

	service.tunnelList(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var resp TunnelListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Tunnels) != 1 {
		t.Fatalf("len(tunnels) = %d, want %d", len(resp.Tunnels), 1)
	}
	item := resp.Tunnels[0]
	if item.RuntimeStatus != tunnelRuntimeFailed {
		t.Fatalf("runtime status = %q, want %q", item.RuntimeStatus, tunnelRuntimeFailed)
	}
	if item.RuntimeRunning {
		t.Fatalf("expected runtime_running=false for failed inlet")
	}
	if !strings.Contains(item.RuntimeMessage, "address already in use") {
		t.Fatalf("runtime message = %q, want bind error", item.RuntimeMessage)
	}
}

func TestPlayerListPageSizeZeroReturnsAllPlayers(t *testing.T) {
	database, err := db.Open("sqlite://file:test_web_player_list?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := manager.NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	for _, player := range []struct {
		username string
		password string
	}{
		{username: "alice", password: "secret1"},
		{username: "bob", password: "secret2"},
		{username: "carol", password: "secret3"},
	} {
		if _, _, _, err := rt.Players.Add(player.username, player.password); err != nil {
			t.Fatalf("add player %s: %v", player.username, err)
		}
	}

	service := NewService(&config.ServerConfig{
		WebUsername: "admin",
		WebPassword: "secret",
	}, rt)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/player_list",
		strings.NewReader(`{"page_number":0,"page_size":0}`),
	)
	req.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(time.Minute)),
	})
	recorder := httptest.NewRecorder()

	service.playerList(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var resp PlayerListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.TotalCount != 3 {
		t.Fatalf("total_count = %d, want %d", resp.TotalCount, 3)
	}
	if len(resp.Players) != 3 {
		t.Fatalf("len(players) = %d, want %d", len(resp.Players), 3)
	}
}

func TestPlayerListReturnsCreateTimeAndNewestFirst(t *testing.T) {
	database, err := db.Open("sqlite://file:test_web_player_list_order?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	userStore := store.NewUserStore(database)
	users := []model.User{
		{
			ID:         10000001,
			Remark:     "oldest",
			Key:        "secret-oldest",
			CreateTime: time.Date(2026, time.April, 8, 8, 0, 0, 0, time.UTC),
		},
		{
			ID:         10000002,
			Remark:     "newest",
			Key:        "secret-newest",
			CreateTime: time.Date(2026, time.April, 8, 10, 0, 0, 0, time.UTC),
		},
		{
			ID:         10000003,
			Remark:     "middle",
			Key:        "secret-middle",
			CreateTime: time.Date(2026, time.April, 8, 9, 0, 0, 0, time.UTC),
		},
	}
	for _, user := range users {
		if err := userStore.Insert(user); err != nil {
			t.Fatalf("insert user %s: %v", user.Remark, err)
		}
	}

	rt, err := manager.NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	service := NewService(&config.ServerConfig{
		WebUsername: "admin",
		WebPassword: "secret",
	}, rt)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/player_list",
		strings.NewReader(`{"page_number":0,"page_size":0}`),
	)
	req.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(time.Minute)),
	})
	recorder := httptest.NewRecorder()

	service.playerList(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var resp PlayerListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Players) != 3 {
		t.Fatalf("len(players) = %d, want %d", len(resp.Players), 3)
	}
	if got, want := resp.Players[0].Remark, "newest"; got != want {
		t.Fatalf("players[0].remark = %q, want %q", got, want)
	}
	if got, want := resp.Players[1].Remark, "middle"; got != want {
		t.Fatalf("players[1].remark = %q, want %q", got, want)
	}
	if got, want := resp.Players[2].Remark, "oldest"; got != want {
		t.Fatalf("players[2].remark = %q, want %q", got, want)
	}
	if resp.Players[0].CreateTime.IsZero() {
		t.Fatalf("expected create_time to be returned")
	}
}

func TestAddPlayerEndpointGeneratesKeyAndReturnsRemarkFields(t *testing.T) {
	database, err := db.Open("sqlite://file:test_web_add_player_generate_key?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := manager.NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	service := NewService(&config.ServerConfig{
		WebUsername: "admin",
		WebPassword: "secret",
	}, rt)

	addReq := httptest.NewRequest(
		http.MethodPost,
		"/api/add_player",
		strings.NewReader(`{"remark":"测试备注","key":""}`),
	)
	addReq.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(time.Minute)),
	})
	addRecorder := httptest.NewRecorder()
	service.addPlayer(addRecorder, addReq)

	if addRecorder.Code != http.StatusOK {
		t.Fatalf("add status = %d, want %d", addRecorder.Code, http.StatusOK)
	}

	listReq := httptest.NewRequest(
		http.MethodPost,
		"/api/player_list",
		strings.NewReader(`{"page_number":0,"page_size":0}`),
	)
	listReq.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(time.Minute)),
	})
	listRecorder := httptest.NewRecorder()
	service.playerList(listRecorder, listReq)

	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRecorder.Code, http.StatusOK)
	}

	var resp PlayerListResponse
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Players) != 1 {
		t.Fatalf("len(players) = %d, want %d", len(resp.Players), 1)
	}
	if resp.Players[0].Remark != "测试备注" {
		t.Fatalf("remark = %q, want %q", resp.Players[0].Remark, "测试备注")
	}
	if resp.Players[0].Key == "" {
		t.Fatalf("expected generated key to be returned")
	}
}

// 验证客户端生成设置可以通过接口完整保存并再次读取。
func TestClientBuildSettingsEndpointsRoundTrip(t *testing.T) {
	database, err := db.Open("sqlite://file:test_web_client_build_settings?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := manager.NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	service := NewService(&config.ServerConfig{
		WebUsername: "admin",
		WebPassword: "secret",
	}, rt)

	updateReq := httptest.NewRequest(
		http.MethodPost,
		"/api/update_client_build_settings",
		strings.NewReader(`{"server":"tcp://127.0.0.1:8118","enable_tls":true,"tls_server_name":"demo.local","use_shadowsocks":true,"ss_server":"127.0.0.1:8388","ss_method":"aes-128-gcm","ss_password":"ss-secret"}`),
	)
	updateReq.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(time.Minute)),
	})
	updateRecorder := httptest.NewRecorder()
	service.updateClientBuildSettings(updateRecorder, updateReq)

	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d", updateRecorder.Code, http.StatusOK)
	}

	getReq := httptest.NewRequest(http.MethodPost, "/api/client_build_settings", strings.NewReader(`{}`))
	getReq.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(time.Minute)),
	})
	getRecorder := httptest.NewRecorder()
	service.clientBuildSettings(getRecorder, getReq)

	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", getRecorder.Code, http.StatusOK)
	}

	var resp ClientBuildSettingsResponse
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Settings.Server != "tcp://127.0.0.1:8118" {
		t.Fatalf("server = %q, want %q", resp.Settings.Server, "tcp://127.0.0.1:8118")
	}
	if !resp.Settings.EnableTLS || !resp.Settings.UseShadowsocks {
		t.Fatalf("expected TLS and Shadowsocks settings to persist: %+v", resp.Settings)
	}
	if resp.Settings.SSServer != "127.0.0.1:8388" {
		t.Fatalf("ss_server = %q, want %q", resp.Settings.SSServer, "127.0.0.1:8388")
	}
}

// 验证下载接口会把玩家信息和后台设置传给构建器，并返回二进制响应。
func TestGenerateClientEndpointReturnsBinaryArtifact(t *testing.T) {
	database, err := db.Open("sqlite://file:test_web_generate_client?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := manager.NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, playerID, err := rt.Players.Add("alice", "player-secret")
	if err != nil {
		t.Fatalf("add player: %v", err)
	}
	if err := rt.ClientBuildSettings.Save(model.ClientBuildSettings{
		Server: "tcp://127.0.0.1:8118",
	}); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	service := NewService(&config.ServerConfig{
		WebUsername: "admin",
		WebPassword: "secret",
	}, rt)
	builder := &clientBuilderStub{
		artifact: &clientbuild.Artifact{
			Filename: "gpipe-client-1-windows-amd64.exe",
			Data:     []byte("binary-data"),
		},
	}
	service.clientBuilder = builder

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/generate_client",
		strings.NewReader(`{"player_id":`+strconv.FormatUint(uint64(playerID), 10)+`,"target":"windows-amd64"}`),
	)
	req.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(time.Minute)),
	})
	recorder := httptest.NewRecorder()
	service.generateClient(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("content type = %q, want %q", got, "application/octet-stream")
	}
	if body := recorder.Body.String(); body != "binary-data" {
		t.Fatalf("body = %q, want %q", body, "binary-data")
	}
	if builder.lastTarget != "windows-amd64" {
		t.Fatalf("target = %q, want %q", builder.lastTarget, "windows-amd64")
	}
	if builder.lastPlayer.Key != "player-secret" {
		t.Fatalf("player key = %q, want %q", builder.lastPlayer.Key, "player-secret")
	}
	if builder.lastSettings.Server != "tcp://127.0.0.1:8118" {
		t.Fatalf("settings server = %q, want %q", builder.lastSettings.Server, "tcp://127.0.0.1:8118")
	}
}

func TestGenerateClientPersistsPlayerBuildSettings(t *testing.T) {
	database, err := db.Open("sqlite://file:test_web_generate_client_player_settings?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := manager.NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, playerID, err := rt.Players.Add("alice", "player-secret")
	if err != nil {
		t.Fatalf("add player: %v", err)
	}
	if err := rt.ClientBuildSettings.Save(model.ClientBuildSettings{
		Server: "tcp://127.0.0.1:8118",
	}); err != nil {
		t.Fatalf("save global settings: %v", err)
	}

	service := NewService(&config.ServerConfig{
		WebUsername: "admin",
		WebPassword: "secret",
	}, rt)
	builder := &clientBuilderStub{
		artifact: &clientbuild.Artifact{
			Filename: "gpipe-client.exe",
			Data:     []byte("binary-data"),
		},
	}
	service.clientBuilder = builder

	getReq := httptest.NewRequest(
		http.MethodPost,
		"/api/player_client_build_settings",
		strings.NewReader(`{"player_id":`+strconv.FormatUint(uint64(playerID), 10)+`}`),
	)
	getReq.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(time.Minute)),
	})
	getRecorder := httptest.NewRecorder()
	service.playerClientBuildSettings(getRecorder, getReq)

	var initialResp PlayerClientBuildSettingsResponse
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &initialResp); err != nil {
		t.Fatalf("decode initial settings response: %v", err)
	}
	if initialResp.Customized {
		t.Fatalf("expected initial player settings to use global defaults")
	}
	if initialResp.Settings.Server != "tcp://127.0.0.1:8118" {
		t.Fatalf("initial server = %q, want global default", initialResp.Settings.Server)
	}

	customServer := "tcp://127.0.0.1:9118"
	generateReq := httptest.NewRequest(
		http.MethodPost,
		"/api/generate_client",
		strings.NewReader(`{"player_id":`+strconv.FormatUint(uint64(playerID), 10)+`,"target":"windows-amd64","settings":{"server":"`+customServer+`","enable_tls":false,"tls_server_name":"","use_shadowsocks":false,"ss_server":"","ss_method":"chacha20-ietf-poly1305","ss_password":""}}`),
	)
	generateReq.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(time.Minute)),
	})
	generateRecorder := httptest.NewRecorder()
	service.generateClient(generateRecorder, generateReq)

	if generateRecorder.Code != http.StatusOK {
		t.Fatalf("generate status = %d, want %d", generateRecorder.Code, http.StatusOK)
	}
	if builder.lastSettings.Server != customServer {
		t.Fatalf("generated server = %q, want %q", builder.lastSettings.Server, customServer)
	}

	stored, customized, err := rt.ClientBuildSettings.GetForPlayer(playerID)
	if err != nil {
		t.Fatalf("get stored player settings: %v", err)
	}
	if !customized {
		t.Fatalf("expected player settings to be customized after generate")
	}
	if stored.Server != customServer {
		t.Fatalf("stored server = %q, want %q", stored.Server, customServer)
	}

	builder.lastSettings = model.ClientBuildSettings{}
	reuseReq := httptest.NewRequest(
		http.MethodPost,
		"/api/generate_client",
		strings.NewReader(`{"player_id":`+strconv.FormatUint(uint64(playerID), 10)+`,"target":"linux-amd64"}`),
	)
	reuseReq.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(time.Minute)),
	})
	reuseRecorder := httptest.NewRecorder()
	service.generateClient(reuseRecorder, reuseReq)

	if reuseRecorder.Code != http.StatusOK {
		t.Fatalf("reuse status = %d, want %d", reuseRecorder.Code, http.StatusOK)
	}
	if builder.lastSettings.Server != customServer {
		t.Fatalf("reused server = %q, want %q", builder.lastSettings.Server, customServer)
	}
}

func TestUpdatePlayerEndpointRejectsKeyChangeWhenPlayerOnline(t *testing.T) {
	database, err := db.Open("sqlite://file:test_web_update_online_player?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := manager.NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, id, err := rt.Players.Add("alice", "oldKey")
	if err != nil {
		t.Fatalf("add player: %v", err)
	}
	rt.Players.Bind(id, playerSessionStub{})

	service := NewService(&config.ServerConfig{
		WebUsername: "admin",
		WebPassword: "secret",
	}, rt)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/update_player",
		strings.NewReader(`{"id":`+strconv.FormatUint(uint64(id), 10)+`,"remark":"alice","key":"newKey"}`),
	)
	req.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(time.Minute)),
	})
	recorder := httptest.NewRecorder()

	service.updatePlayer(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var resp GeneralResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != -2 {
		t.Fatalf("code = %d, want %d", resp.Code, -2)
	}
	if !strings.Contains(resp.Msg, "online players cannot change key") {
		t.Fatalf("msg = %q, want online key restriction", resp.Msg)
	}
}

func TestRemovePlayerEndpointRejectsOnlinePlayer(t *testing.T) {
	database, err := db.Open("sqlite://file:test_web_delete_online_player?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := manager.NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, id, err := rt.Players.Add("alice", "oldKey")
	if err != nil {
		t.Fatalf("add player: %v", err)
	}
	rt.Players.Bind(id, playerSessionStub{})

	service := NewService(&config.ServerConfig{
		WebUsername: "admin",
		WebPassword: "secret",
	}, rt)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/remove_player",
		strings.NewReader(`{"id":`+strconv.FormatUint(uint64(id), 10)+`}`),
	)
	req.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(time.Minute)),
	})
	recorder := httptest.NewRecorder()

	service.removePlayer(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var resp GeneralResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != -1 {
		t.Fatalf("code = %d, want %d", resp.Code, -1)
	}
	if !strings.Contains(resp.Msg, "online players cannot be deleted") {
		t.Fatalf("msg = %q, want online delete restriction", resp.Msg)
	}
}
