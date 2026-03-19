package repo

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/user/uniapi/internal/crypto"
	"github.com/user/uniapi/internal/db"
)

type Account struct {
	ID            string
	Provider      string
	Label         string
	Credential    string // decrypted API key
	Models        []string
	MaxConcurrent int
	Enabled       bool
	ConfigManaged bool
	CreatedAt     time.Time
}

type AccountRepo struct {
	db     *db.Database
	encKey []byte
}

func NewAccountRepo(database *db.Database, encKey []byte) *AccountRepo {
	return &AccountRepo{db: database, encKey: encKey}
}

func modelsToString(models []string) string {
	return strings.Join(models, ",")
}

func stringToModels(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}

func (r *AccountRepo) Create(provider, label, apiKey string, models []string, maxConcurrent int, configManaged bool) (*Account, error) {
	encrypted, err := crypto.Encrypt(r.encKey, apiKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt credential: %w", err)
	}
	a := &Account{
		ID:            uuid.New().String(),
		Provider:      provider,
		Label:         label,
		Credential:    apiKey,
		Models:        models,
		MaxConcurrent: maxConcurrent,
		Enabled:       true,
		ConfigManaged: configManaged,
		CreatedAt:     time.Now(),
	}
	_, err = r.db.DB.Exec(
		`INSERT INTO accounts (id, provider, label, credential, models, max_concurrent, enabled, config_managed, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Provider, a.Label, encrypted,
		modelsToString(models), a.MaxConcurrent,
		a.Enabled, a.ConfigManaged, a.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}
	return a, nil
}

func (r *AccountRepo) scanAccount(row *sql.Row) (*Account, error) {
	var a Account
	var encCredential string
	var modelsStr string
	err := row.Scan(
		&a.ID, &a.Provider, &a.Label, &encCredential,
		&modelsStr, &a.MaxConcurrent, &a.Enabled, &a.ConfigManaged, &a.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("account not found")
	}
	if err != nil {
		return nil, err
	}
	decrypted, err := crypto.Decrypt(r.encKey, encCredential)
	if err != nil {
		return nil, fmt.Errorf("decrypt credential: %w", err)
	}
	a.Credential = decrypted
	a.Models = stringToModels(modelsStr)
	return &a, nil
}

func (r *AccountRepo) GetByID(id string) (*Account, error) {
	row := r.db.DB.QueryRow(
		`SELECT id, provider, label, credential, models, max_concurrent, enabled, config_managed, created_at
		 FROM accounts WHERE id = ?`,
		id,
	)
	a, err := r.scanAccount(row)
	if err != nil {
		if strings.Contains(err.Error(), "account not found") {
			return nil, fmt.Errorf("account not found: %s", id)
		}
		return nil, err
	}
	return a, nil
}

func (r *AccountRepo) ListAll() ([]Account, error) {
	rows, err := r.db.DB.Query(
		`SELECT id, provider, label, credential, models, max_concurrent, enabled, config_managed, created_at
		 FROM accounts ORDER BY created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []Account
	for rows.Next() {
		var a Account
		var encCredential string
		var modelsStr string
		if err := rows.Scan(
			&a.ID, &a.Provider, &a.Label, &encCredential,
			&modelsStr, &a.MaxConcurrent, &a.Enabled, &a.ConfigManaged, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		decrypted, err := crypto.Decrypt(r.encKey, encCredential)
		if err != nil {
			return nil, fmt.Errorf("decrypt credential for account %s: %w", a.ID, err)
		}
		a.Credential = decrypted
		a.Models = stringToModels(modelsStr)
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func (r *AccountRepo) Update(id string, label string, enabled bool) error {
	result, err := r.db.DB.Exec(
		"UPDATE accounts SET label = ?, enabled = ? WHERE id = ?",
		label, enabled, id,
	)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("account not found: %s", id)
	}
	return nil
}

func (r *AccountRepo) Delete(id string) error {
	result, err := r.db.DB.Exec("DELETE FROM accounts WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("account not found: %s", id)
	}
	return nil
}

func (r *AccountRepo) SetEnabled(id string, enabled bool) error {
	result, err := r.db.DB.Exec(
		"UPDATE accounts SET enabled = ? WHERE id = ?",
		enabled, id,
	)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("account not found: %s", id)
	}
	return nil
}
