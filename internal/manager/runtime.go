package manager

import (
	"database/sql"

	"github.com/pizixi/gpipe/internal/store"
)

type Runtime struct {
	Users   *store.UserStore
	Tunnels *store.TunnelStore
	Players *PlayerManager
	Tunnel  *TunnelManager
}

func NewRuntime(db *sql.DB, notifier TunnelNotifier) (*Runtime, error) {
	userStore := store.NewUserStore(db)
	tunnelStore := store.NewTunnelStore(db)
	players := NewPlayerManager(userStore)
	if err := players.LoadAll(); err != nil {
		return nil, err
	}
	tunnelManager := NewTunnelManager(tunnelStore, players, notifier)
	if err := tunnelManager.LoadAll(); err != nil {
		return nil, err
	}
	return &Runtime{
		Users:   userStore,
		Tunnels: tunnelStore,
		Players: players,
		Tunnel:  tunnelManager,
	}, nil
}
