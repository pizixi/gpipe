package manager

import (
	cryptorand "crypto/rand"
	"errors"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/pizixi/gpipe/internal/model"
	"github.com/pizixi/gpipe/internal/proto"
	"github.com/pizixi/gpipe/internal/store"
	"github.com/pizixi/gpipe/internal/util"
)

const playerKeyCharset = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
const playerKeyLength = 20

var (
	ErrPlayerOnlineKeyUpdate = errors.New("online players cannot change key")
	ErrPlayerOnlineDelete    = errors.New("online players cannot be deleted")
)

type PlayerSession interface {
	Close() error
	SendPush(message proto.Message) error
}

type PlayerState struct {
	ID      uint32
	Online  bool
	Session PlayerSession
}

type PlayerManager struct {
	store   *store.UserStore
	opMu    sync.Mutex
	mu      sync.RWMutex
	players map[uint32]*PlayerState
}

func NewPlayerManager(userStore *store.UserStore) *PlayerManager {
	return &PlayerManager{
		store:   userStore,
		players: map[uint32]*PlayerState{},
	}
}

func (m *PlayerManager) LoadAll() error {
	users, err := m.store.FindAll()
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, user := range users {
		m.players[user.ID] = &PlayerState{ID: user.ID}
	}
	return nil
}

func (m *PlayerManager) Exists(id uint32) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.players[id]
	return ok
}

func (m *PlayerManager) IsOnline(id uint32) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	player, ok := m.players[id]
	return ok && player.Online
}

func (m *PlayerManager) Session(id uint32) PlayerSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	player := m.players[id]
	if player == nil {
		return nil
	}
	return player.Session
}

func (m *PlayerManager) Bind(id uint32, session PlayerSession) {
	m.opMu.Lock()
	m.mu.Lock()
	player := m.players[id]
	if player == nil {
		player = &PlayerState{ID: id}
		m.players[id] = player
	}
	oldSession := player.Session
	player.Session = session
	player.Online = true
	m.mu.Unlock()
	m.opMu.Unlock()

	if oldSession != nil && oldSession != session {
		_ = oldSession.Close()
	}
}

func (m *PlayerManager) Unbind(id uint32, session PlayerSession) {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	player := m.players[id]
	if player == nil {
		return
	}
	if player.Session == session {
		player.Session = nil
		player.Online = false
	}
}

func (m *PlayerManager) Add(remark, key string) (int32, string, uint32, error) {
	m.opMu.Lock()
	defer m.opMu.Unlock()

	remark = strings.TrimSpace(remark)
	key = strings.TrimSpace(key)
	if !util.IsValidPlayerRemark(remark) {
		return -1, "remark may not be empty and must be within 30 characters.", 0, nil
	}
	if key == "" {
		var err error
		key, err = m.generateUniqueKey()
		if err != nil {
			return 0, "", 0, err
		}
	}
	if !util.IsValidPlayerKey(key) {
		return -1, "key must be 2-64 ASCII characters without spaces.", 0, nil
	}
	existing, err := m.store.FindByRemark(remark)
	if err != nil {
		return 0, "", 0, err
	}
	if existing != nil {
		return -2, "remark already exists", 0, nil
	}
	existing, err = m.store.FindByKey(key)
	if err != nil {
		return 0, "", 0, err
	}
	if existing != nil {
		return -3, "key already exists", 0, nil
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 10000; i++ {
		id := uint32(rng.Intn(89999999) + 10000000)
		if m.Exists(id) {
			continue
		}
		user := model.User{
			ID:         id,
			Remark:     remark,
			Key:        key,
			CreateTime: time.Now().UTC(),
		}
		if err := m.store.Insert(user); err != nil {
			return 0, "", 0, err
		}
		m.mu.Lock()
		m.players[id] = &PlayerState{ID: id}
		m.mu.Unlock()
		return 0, "", id, nil
	}
	return -4, "too many cycles", 0, nil
}

func (m *PlayerManager) Update(update model.PlayerUpdate) error {
	m.opMu.Lock()
	defer m.opMu.Unlock()

	update.Remark = strings.TrimSpace(update.Remark)
	update.Key = strings.TrimSpace(update.Key)
	if !util.IsValidPlayerRemark(update.Remark) {
		return errors.New("remark may not be empty and must be within 30 characters.")
	}
	if !util.IsValidPlayerKey(update.Key) {
		return errors.New("key must be 2-64 ASCII characters without spaces.")
	}
	existingRemark, err := m.store.FindByRemark(update.Remark)
	if err != nil {
		return err
	}
	if existingRemark != nil && existingRemark.ID != update.ID {
		return errors.New("remark already exists")
	}
	existingKey, err := m.store.FindByKey(update.Key)
	if err != nil {
		return err
	}
	if existingKey != nil && existingKey.ID != update.ID {
		return errors.New("key already exists")
	}
	current, err := m.store.FindByID(update.ID)
	if err != nil {
		return err
	}
	if current != nil && current.Key != update.Key {
		m.mu.RLock()
		player := m.players[update.ID]
		isOnline := player != nil && player.Online
		m.mu.RUnlock()
		if isOnline {
			return ErrPlayerOnlineKeyUpdate
		}
	}
	return m.store.Update(update)
}

func (m *PlayerManager) Delete(id uint32) error {
	m.opMu.Lock()
	defer m.opMu.Unlock()

	m.mu.RLock()
	player := m.players[id]
	isOnline := player != nil && player.Online
	m.mu.RUnlock()
	if isOnline {
		return ErrPlayerOnlineDelete
	}
	if err := m.store.Delete(id); err != nil {
		return err
	}
	m.mu.Lock()
	online := m.players[id]
	delete(m.players, id)
	m.mu.Unlock()
	if online != nil && online.Session != nil {
		_ = online.Session.Close()
	}
	return nil
}

func (m *PlayerManager) List(pageNumber, pageSize int) ([]model.User, int, error) {
	return m.store.List(pageNumber, pageSize)
}

func (m *PlayerManager) All() ([]model.User, error) {
	return m.store.FindAll()
}

func (m *PlayerManager) generateUniqueKey() (string, error) {
	for i := 0; i < 1000; i++ {
		key, err := generateRandomPlayerKey()
		if err != nil {
			return "", err
		}
		existing, err := m.store.FindByKey(key)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return key, nil
		}
	}
	return "", errors.New("failed to generate unique player key")
}

func generateRandomPlayerKey() (string, error) {
	randomBytes := make([]byte, playerKeyLength)
	if _, err := cryptorand.Read(randomBytes); err != nil {
		return "", err
	}
	out := make([]byte, playerKeyLength)
	for i, b := range randomBytes {
		out[i] = playerKeyCharset[int(b)%len(playerKeyCharset)]
	}
	return string(out), nil
}
