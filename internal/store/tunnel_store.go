package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/pizixi/gpipe/internal/model"
)

type TunnelStore struct {
	db *sql.DB
}

func NewTunnelStore(db *sql.DB) *TunnelStore {
	return &TunnelStore{db: db}
}

func (s *TunnelStore) FindAll() ([]model.Tunnel, error) {
	rows, err := s.db.Query(`
		SELECT id, source, endpoint, enabled, sender, receiver, description, tunnel_type,
		       password, username, is_compressed, custom_mapping, encryption_method
		FROM tunnel
		ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tunnels []model.Tunnel
	for rows.Next() {
		tunnel, err := scanTunnel(rows)
		if err != nil {
			return nil, err
		}
		tunnels = append(tunnels, tunnel)
	}
	return tunnels, rows.Err()
}

func (s *TunnelStore) FindByID(id uint32) (*model.Tunnel, error) {
	row := s.db.QueryRow(`
		SELECT id, source, endpoint, enabled, sender, receiver, description, tunnel_type,
		       password, username, is_compressed, custom_mapping, encryption_method
		FROM tunnel
		WHERE id = ?`, id)
	tunnel, err := scanTunnel(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tunnel, nil
}

func (s *TunnelStore) Insert(tunnel model.Tunnel) (uint32, error) {
	customMapping, err := json.Marshal(tunnel.CustomMapping)
	if err != nil {
		return 0, err
	}
	result, err := s.db.Exec(`
		INSERT INTO tunnel(source, endpoint, enabled, sender, receiver, description, tunnel_type,
		                   password, username, is_compressed, custom_mapping, encryption_method)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tunnel.Source, tunnel.Endpoint, boolToInt(tunnel.Enabled), tunnel.Sender, tunnel.Receiver,
		tunnel.Description, tunnel.TunnelType, tunnel.Password, tunnel.Username,
		boolToInt(tunnel.IsCompressed), string(customMapping), tunnel.EncryptionMethod,
	)
	if err != nil {
		return 0, err
	}
	lastID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint32(lastID), nil
}

func (s *TunnelStore) Update(tunnel model.Tunnel) error {
	customMapping, err := json.Marshal(tunnel.CustomMapping)
	if err != nil {
		return err
	}
	result, err := s.db.Exec(`
		UPDATE tunnel
		SET source = ?, endpoint = ?, enabled = ?, sender = ?, receiver = ?, description = ?,
		    tunnel_type = ?, password = ?, username = ?, is_compressed = ?, custom_mapping = ?,
		    encryption_method = ?
		WHERE id = ?`,
		tunnel.Source, tunnel.Endpoint, boolToInt(tunnel.Enabled), tunnel.Sender, tunnel.Receiver,
		tunnel.Description, tunnel.TunnelType, tunnel.Password, tunnel.Username,
		boolToInt(tunnel.IsCompressed), string(customMapping), tunnel.EncryptionMethod, tunnel.ID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("update tunnel: rows_affected = %d", rows)
	}
	return nil
}

func (s *TunnelStore) Delete(id uint32) error {
	result, err := s.db.Exec(`DELETE FROM tunnel WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("delete tunnel: rows_affected = %d", rows)
	}
	return nil
}

func scanTunnel(scanner interface {
	Scan(dest ...any) error
}) (model.Tunnel, error) {
	var tunnel model.Tunnel
	var enabled int
	var isCompressed int
	var customMapping string
	err := scanner.Scan(
		&tunnel.ID,
		&tunnel.Source,
		&tunnel.Endpoint,
		&enabled,
		&tunnel.Sender,
		&tunnel.Receiver,
		&tunnel.Description,
		&tunnel.TunnelType,
		&tunnel.Password,
		&tunnel.Username,
		&isCompressed,
		&customMapping,
		&tunnel.EncryptionMethod,
	)
	if err != nil {
		return model.Tunnel{}, err
	}
	tunnel.Enabled = enabled == 1
	tunnel.IsCompressed = isCompressed == 1
	if customMapping != "" {
		_ = json.Unmarshal([]byte(customMapping), &tunnel.CustomMapping)
	}
	if tunnel.CustomMapping == nil {
		tunnel.CustomMapping = map[string]string{}
	}
	return tunnel, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
