package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pizixi/gpipe/internal/config"
	"github.com/pizixi/gpipe/internal/db"
	"github.com/pizixi/gpipe/internal/manager"
	"github.com/pizixi/gpipe/internal/model"
	"github.com/pizixi/gpipe/internal/proto"
)

type playerSessionStub struct{}

func (playerSessionStub) Close() error { return nil }

func (playerSessionStub) SendPush(message proto.Message) error {
	_ = message
	return nil
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

	authReq := httptest.NewRequest(http.MethodGet, "/api/test_auth", nil)
	authReq.AddCookie(cookie)
	id, ok := service.authenticated(authReq)
	if !ok {
		t.Fatalf("expected signed cookie to authenticate")
	}
	if id != "admin" {
		t.Fatalf("authenticated id = %q, want %q", id, "admin")
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
	if _, ok := service.authenticated(tamperedReq); ok {
		t.Fatalf("expected tampered cookie to be rejected")
	}

	expiredReq := httptest.NewRequest(http.MethodGet, "/api/test_auth", nil)
	expiredReq.AddCookie(&http.Cookie{
		Name:  authCookieName,
		Value: service.signedCookieValue(time.Now().Add(-time.Minute)),
	})
	if _, ok := service.authenticated(expiredReq); ok {
		t.Fatalf("expected expired cookie to be rejected")
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
	if !strings.Contains(body, "<!doctype html>") {
		t.Fatalf("expected embedded index html response")
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
