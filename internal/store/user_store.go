package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/pizixi/gpipe/internal/model"
)

type UserStore struct {
	db *sql.DB
}

func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) List(pageNumber, pageSize int) ([]model.User, int, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}
	if pageNumber < 0 {
		pageNumber = 0
	}

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM user`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(`
		SELECT id, username, password, create_time, last_online_time, last_ip
		FROM user
		ORDER BY create_time DESC, id DESC
		LIMIT ? OFFSET ?`, pageSize, pageNumber*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, user)
	}
	return users, total, rows.Err()
}

func (s *UserStore) FindByKey(key string) (*model.User, error) {
	rows, err := s.db.Query(`
		SELECT id, username, password, create_time, last_online_time, last_ip
		FROM user
		WHERE password = ?
		ORDER BY id
		LIMIT 2`,
		key,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		user  *model.User
		count int
	)
	for rows.Next() {
		count++
		item, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		user = &item
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	switch count {
	case 0:
		return nil, nil
	case 1:
		return user, nil
	default:
		return nil, fmt.Errorf("duplicate player key exists")
	}
}

func (s *UserStore) FindByRemark(remark string) (*model.User, error) {
	row := s.db.QueryRow(`
		SELECT id, username, password, create_time, last_online_time, last_ip
		FROM user
		WHERE username = ?`,
		remark,
	)
	user, err := scanUser(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) FindByID(id uint32) (*model.User, error) {
	row := s.db.QueryRow(`
		SELECT id, username, password, create_time, last_online_time, last_ip
		FROM user
		WHERE id = ?`,
		id,
	)
	user, err := scanUser(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) FindAll() ([]model.User, error) {
	rows, err := s.db.Query(`
		SELECT id, username, password, create_time, last_online_time, last_ip
		FROM user
		ORDER BY create_time DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *UserStore) Insert(user model.User) error {
	_, err := s.db.Exec(`
		INSERT INTO user(id, username, password, create_time, last_online_time, last_ip)
		VALUES(?, ?, ?, ?, ?, ?)`,
		user.ID,
		user.Remark,
		user.Key,
		user.CreateTime.UTC().Format(time.RFC3339Nano),
		formatOptionalTime(user.LastOnlineTime),
		strings.TrimSpace(user.LastIP),
	)
	return err
}

func (s *UserStore) Update(update model.PlayerUpdate) error {
	result, err := s.db.Exec(`
		UPDATE user
		SET username = ?, password = ?
		WHERE id = ?`,
		update.Remark, update.Key, update.ID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("update player: rows_affected = %d", rows)
	}
	return nil
}

func (s *UserStore) UpdateLoginInfo(id uint32, at time.Time, ip string) error {
	result, err := s.db.Exec(`
		UPDATE user
		SET last_online_time = ?, last_ip = ?
		WHERE id = ?`,
		at.UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(ip),
		id,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("update player login info: rows_affected = %d", rows)
	}
	return nil
}

func (s *UserStore) Delete(id uint32) error {
	result, err := s.db.Exec(`DELETE FROM user WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("delete player: rows_affected = %d", rows)
	}
	return nil
}

type userScanner interface {
	Scan(dest ...any) error
}

func scanUser(scanner userScanner) (model.User, error) {
	var (
		user           model.User
		createTime     string
		lastOnlineTime sql.NullString
		lastIP         sql.NullString
	)
	if err := scanner.Scan(
		&user.ID,
		&user.Remark,
		&user.Key,
		&createTime,
		&lastOnlineTime,
		&lastIP,
	); err != nil {
		return model.User{}, err
	}
	user.CreateTime, _ = time.Parse(time.RFC3339Nano, createTime)
	if lastOnlineTime.Valid && strings.TrimSpace(lastOnlineTime.String) != "" {
		if ts, err := time.Parse(time.RFC3339Nano, lastOnlineTime.String); err == nil {
			user.LastOnlineTime = &ts
		}
	}
	if lastIP.Valid {
		user.LastIP = strings.TrimSpace(lastIP.String)
	}
	return user, nil
}

func formatOptionalTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
