package manager

import (
	"errors"
	"testing"
	"time"

	"github.com/pizixi/gpipe/internal/db"
	"github.com/pizixi/gpipe/internal/model"
	"github.com/pizixi/gpipe/internal/proto"
	"github.com/pizixi/gpipe/internal/util"
)

type playerSessionStub struct {
	closeFn func() error
}

func (s *playerSessionStub) Close() error {
	if s.closeFn != nil {
		return s.closeFn()
	}
	return nil
}

func (s *playerSessionStub) SendPush(message proto.Message) error {
	_ = message
	return nil
}

func TestBindReplacesExistingSessionWithoutDeadlock(t *testing.T) {
	manager := NewPlayerManager(nil)

	first := &playerSessionStub{}
	closeDone := make(chan struct{})
	first.closeFn = func() error {
		manager.Unbind(1, first)
		close(closeDone)
		return nil
	}
	manager.Bind(1, first)

	second := &playerSessionStub{}
	bindDone := make(chan struct{})
	go func() {
		manager.Bind(1, second)
		close(bindDone)
	}()

	select {
	case <-bindDone:
	case <-time.After(time.Second):
		t.Fatalf("Bind blocked while replacing existing session")
	}

	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatalf("expected old session to be closed")
	}

	if session := manager.Session(1); session != second {
		t.Fatalf("expected replacement session to stay bound")
	}
	if !manager.IsOnline(1) {
		t.Fatalf("expected player to remain online")
	}
}

func TestAddWithoutKeyGeneratesPlayerKey(t *testing.T) {
	database, err := db.Open("sqlite://file:test_player_manager_generate_key?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, id, err := rt.Players.Add("测试备注", "")
	if err != nil {
		t.Fatalf("add player: %v", err)
	}

	user, err := rt.Users.FindByRemark("测试备注")
	if err != nil {
		t.Fatalf("find by remark: %v", err)
	}
	if user == nil {
		t.Fatalf("expected player to be stored")
	}
	if user.ID != id {
		t.Fatalf("player id = %d, want %d", user.ID, id)
	}
	if !util.IsValidPlayerKey(user.Key) {
		t.Fatalf("generated key is invalid: %q", user.Key)
	}
	if len(user.Key) != playerKeyLength {
		t.Fatalf("generated key length = %d, want %d", len(user.Key), playerKeyLength)
	}
}

func TestUpdateAllowsChangingRemarkAndKey(t *testing.T) {
	database, err := db.Open("sqlite://file:test_player_manager_update_player?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, id, err := rt.Players.Add("原备注", "oldKey")
	if err != nil {
		t.Fatalf("add player: %v", err)
	}

	if err := rt.Players.Update(model.PlayerUpdate{
		ID:     id,
		Remark: "新备注",
		Key:    "newKey123",
	}); err != nil {
		t.Fatalf("update player: %v", err)
	}

	user, err := rt.Users.FindByRemark("新备注")
	if err != nil {
		t.Fatalf("find by remark: %v", err)
	}
	if user == nil {
		t.Fatalf("expected updated player")
	}
	if user.Key != "newKey123" {
		t.Fatalf("player key = %q, want %q", user.Key, "newKey123")
	}
}

func TestUpdateRejectsChangingKeyWhilePlayerOnline(t *testing.T) {
	database, err := db.Open("sqlite://file:test_player_manager_online_key_change?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, id, err := rt.Players.Add("在线玩家", "oldKey")
	if err != nil {
		t.Fatalf("add player: %v", err)
	}
	rt.Players.Bind(id, &playerSessionStub{})

	err = rt.Players.Update(model.PlayerUpdate{
		ID:     id,
		Remark: "在线玩家",
		Key:    "newKey123",
	})
	if !errors.Is(err, ErrPlayerOnlineKeyUpdate) {
		t.Fatalf("update error = %v, want %v", err, ErrPlayerOnlineKeyUpdate)
	}
}

func TestUpdateAllowsChangingRemarkWhilePlayerOnline(t *testing.T) {
	database, err := db.Open("sqlite://file:test_player_manager_online_remark_change?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, id, err := rt.Players.Add("原备注", "sameKey")
	if err != nil {
		t.Fatalf("add player: %v", err)
	}
	rt.Players.Bind(id, &playerSessionStub{})

	if err := rt.Players.Update(model.PlayerUpdate{
		ID:     id,
		Remark: "新备注",
		Key:    "sameKey",
	}); err != nil {
		t.Fatalf("update player: %v", err)
	}

	user, err := rt.Users.FindByRemark("新备注")
	if err != nil {
		t.Fatalf("find by remark: %v", err)
	}
	if user == nil {
		t.Fatalf("expected updated player")
	}
}

func TestDeleteRejectsOnlinePlayer(t *testing.T) {
	database, err := db.Open("sqlite://file:test_player_manager_online_delete?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer database.Close()

	rt, err := NewRuntime(database, nil)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}

	_, _, id, err := rt.Players.Add("在线玩家", "keepKey")
	if err != nil {
		t.Fatalf("add player: %v", err)
	}
	rt.Players.Bind(id, &playerSessionStub{})

	err = rt.Players.Delete(id)
	if !errors.Is(err, ErrPlayerOnlineDelete) {
		t.Fatalf("delete error = %v, want %v", err, ErrPlayerOnlineDelete)
	}
	if !rt.Players.Exists(id) {
		t.Fatalf("expected player to remain after rejected delete")
	}
}
