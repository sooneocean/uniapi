package repo

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/user/uniapi/internal/db"
)

type User struct {
	ID        string
	Username  string
	Password  string
	Role      string
	CreatedAt time.Time
}

type UserRepo struct {
	db *db.Database
}

func NewUserRepo(database *db.Database) *UserRepo {
	return &UserRepo{db: database}
}

func (r *UserRepo) Create(username, passwordHash, role string) (*User, error) {
	user := &User{
		ID:        uuid.New().String(),
		Username:  username,
		Password:  passwordHash,
		Role:      role,
		CreatedAt: time.Now(),
	}
	_, err := r.db.DB.Exec(
		"INSERT INTO users (id, username, password, role, created_at) VALUES (?, ?, ?, ?, ?)",
		user.ID, user.Username, user.Password, user.Role, user.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (r *UserRepo) GetByUsername(username string) (*User, error) {
	user := &User{}
	err := r.db.DB.QueryRow(
		"SELECT id, username, password, role, created_at FROM users WHERE username = ?",
		username,
	).Scan(&user.ID, &user.Username, &user.Password, &user.Role, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found: %s", username)
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepo) GetByID(id string) (*User, error) {
	user := &User{}
	err := r.db.DB.QueryRow(
		"SELECT id, username, password, role, created_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Username, &user.Password, &user.Role, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found: %s", id)
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepo) List() ([]User, error) {
	rows, err := r.db.DB.Query(
		"SELECT id, username, role, created_at FROM users ORDER BY created_at",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepo) Delete(id string) error {
	result, err := r.db.DB.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user not found: %s", id)
	}
	return nil
}
