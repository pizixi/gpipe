package store

import (
	"database/sql"
	"fmt"
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
		SELECT id, username, password, create_time
		FROM user
		ORDER BY id
		LIMIT ? OFFSET ?`, pageSize, pageNumber*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var user model.User
		var createTime string
		if err := rows.Scan(&user.ID, &user.Remark, &user.Key, &createTime); err != nil {
			return nil, 0, err
		}
		if ts, err := time.Parse(time.RFC3339Nano, createTime); err == nil {
			user.CreateTime = ts
		}
		users = append(users, user)
	}
	return users, total, rows.Err()
}

func (s *UserStore) FindByKey(key string) (*model.User, error) {
	rows, err := s.db.Query(`
		SELECT id, username, password, create_time
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
		item := &model.User{}
		var createTime string
		if err := rows.Scan(&item.ID, &item.Remark, &item.Key, &createTime); err != nil {
			return nil, err
		}
		item.CreateTime, _ = time.Parse(time.RFC3339Nano, createTime)
		user = item
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
		SELECT id, username, password, create_time
		FROM user
		WHERE username = ?`,
		remark,
	)
	var user model.User
	var createTime string
	if err := row.Scan(&user.ID, &user.Remark, &user.Key, &createTime); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	user.CreateTime, _ = time.Parse(time.RFC3339Nano, createTime)
	return &user, nil
}

func (s *UserStore) FindByID(id uint32) (*model.User, error) {
	row := s.db.QueryRow(`
		SELECT id, username, password, create_time
		FROM user
		WHERE id = ?`,
		id,
	)
	var user model.User
	var createTime string
	if err := row.Scan(&user.ID, &user.Remark, &user.Key, &createTime); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	user.CreateTime, _ = time.Parse(time.RFC3339Nano, createTime)
	return &user, nil
}

func (s *UserStore) FindAll() ([]model.User, error) {
	rows, err := s.db.Query(`
		SELECT id, username, password, create_time
		FROM user
		ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var user model.User
		var createTime string
		if err := rows.Scan(&user.ID, &user.Remark, &user.Key, &createTime); err != nil {
			return nil, err
		}
		user.CreateTime, _ = time.Parse(time.RFC3339Nano, createTime)
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *UserStore) Insert(user model.User) error {
	_, err := s.db.Exec(`
		INSERT INTO user(id, username, password, create_time)
		VALUES(?, ?, ?, ?)`,
		user.ID, user.Remark, user.Key, user.CreateTime.UTC().Format(time.RFC3339Nano),
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
