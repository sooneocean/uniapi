package repo

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sooneocean/uniapi/internal/crypto"
	"github.com/sooneocean/uniapi/internal/db"
)

type Account struct {
	ID             string
	Provider       string
	Label          string
	Credential     string // decrypted API key
	Models         []string
	MaxConcurrent  int
	Enabled        bool
	ConfigManaged  bool
	CreatedAt      time.Time
	AuthType       string     // "api_key", "oauth", "session_token"
	OAuthProvider  string
	RefreshToken   string     // decrypted
	TokenExpiresAt *time.Time
	OwnerUserID    string     // "" = shared
	NeedsReauth    bool
}

type AccountRepo struct {
	db     *db.Database
	encKey []byte
}

func NewAccountRepo(database *db.Database, encKey []byte) *AccountRepo {
	return &AccountRepo{db: database, encKey: encKey}
}

func nullStr(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

func nullTime(nt sql.NullTime) *time.Time {
	if nt.Valid {
		return &nt.Time
	}
	return nil
}

func toNullable(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func checkAffected(result sql.Result, entity string) error {
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("%s not found", entity)
	}
	return nil
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
		AuthType:      "api_key",
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

// CreateBound creates an OAuth/session_token account bound to a specific user.
func (r *AccountRepo) CreateBound(provider, label, authType, oauthProvider, accessToken, refreshToken string, expiresAt time.Time, models []string, maxConcurrent int, ownerUserID string, configManaged bool) (*Account, error) {
	encAccess, err := crypto.Encrypt(r.encKey, accessToken)
	if err != nil {
		return nil, fmt.Errorf("encrypt credential: %w", err)
	}
	var encRefresh string
	if refreshToken != "" {
		encRefresh, err = crypto.Encrypt(r.encKey, refreshToken)
		if err != nil {
			return nil, fmt.Errorf("encrypt refresh_token: %w", err)
		}
	}

	a := &Account{
		ID:            uuid.New().String(),
		Provider:      provider,
		Label:         label,
		Credential:    accessToken,
		Models:        models,
		MaxConcurrent: maxConcurrent,
		Enabled:       true,
		ConfigManaged: configManaged,
		CreatedAt:     time.Now(),
		AuthType:      authType,
		OAuthProvider: oauthProvider,
		RefreshToken:  refreshToken,
		OwnerUserID:   ownerUserID,
	}

	var tokenExpiresAt interface{}
	if !expiresAt.IsZero() {
		tokenExpiresAt = expiresAt
		t := expiresAt
		a.TokenExpiresAt = &t
	}

	_, err = r.db.DB.Exec(
		`INSERT INTO accounts (id, provider, label, credential, models, max_concurrent, enabled, config_managed, created_at, auth_type, oauth_provider, refresh_token, token_expires_at, owner_user_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Provider, a.Label, encAccess,
		modelsToString(models), a.MaxConcurrent,
		a.Enabled, a.ConfigManaged, a.CreatedAt,
		authType, toNullable(oauthProvider), encRefresh, tokenExpiresAt, toNullable(ownerUserID),
	)
	if err != nil {
		return nil, fmt.Errorf("create bound account: %w", err)
	}
	return a, nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func (r *AccountRepo) scanInto(s scanner) (*Account, error) {
	var a Account
	var encCredential string
	var modelsStr string
	var authType sql.NullString
	var oauthProvider sql.NullString
	var encRefreshToken sql.NullString
	var tokenExpiresAt sql.NullTime
	var ownerUserID sql.NullString
	var needsReauth bool

	err := s.Scan(
		&a.ID, &a.Provider, &a.Label, &encCredential,
		&modelsStr, &a.MaxConcurrent, &a.Enabled, &a.ConfigManaged, &a.CreatedAt,
		&authType, &oauthProvider, &encRefreshToken, &tokenExpiresAt, &ownerUserID, &needsReauth,
	)
	if err != nil {
		return nil, err
	}

	decrypted, err := crypto.Decrypt(r.encKey, encCredential)
	if err != nil {
		return nil, fmt.Errorf("decrypt credential: %w", err)
	}
	a.Credential = decrypted
	a.Models = stringToModels(modelsStr)

	if authType.Valid {
		a.AuthType = authType.String
	} else {
		a.AuthType = "api_key"
	}
	a.OAuthProvider = nullStr(oauthProvider)
	if encRefreshToken.Valid && encRefreshToken.String != "" {
		decRefresh, err := crypto.Decrypt(r.encKey, encRefreshToken.String)
		if err != nil {
			return nil, fmt.Errorf("decrypt refresh_token: %w", err)
		}
		a.RefreshToken = decRefresh
	}
	a.TokenExpiresAt = nullTime(tokenExpiresAt)
	a.OwnerUserID = nullStr(ownerUserID)
	a.NeedsReauth = needsReauth

	return &a, nil
}

const accountSelectColumns = `id, provider, label, credential, models, max_concurrent, enabled, config_managed, created_at,
		auth_type, oauth_provider, refresh_token, token_expires_at, owner_user_id, needs_reauth`

func (r *AccountRepo) GetByID(id string) (*Account, error) {
	row := r.db.DB.QueryRow(
		`SELECT `+accountSelectColumns+`
		 FROM accounts WHERE id = ?`,
		id,
	)
	a, err := r.scanInto(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("account not found: %s", id)
		}
		return nil, err
	}
	return a, nil
}

func (r *AccountRepo) ListAll() ([]Account, error) {
	rows, err := r.db.DB.Query(
		`SELECT ` + accountSelectColumns + `
		 FROM accounts ORDER BY created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []Account
	for rows.Next() {
		a, err := r.scanInto(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, *a)
	}
	return accounts, rows.Err()
}

// ListForUser returns accounts that are shared (owner_user_id IS NULL) or owned by the given user,
// excluding accounts that need reauth.
func (r *AccountRepo) ListForUser(userID string) ([]Account, error) {
	rows, err := r.db.DB.Query(
		`SELECT `+accountSelectColumns+`
		 FROM accounts
		 WHERE (owner_user_id IS NULL OR owner_user_id = ?) AND needs_reauth = 0
		 ORDER BY created_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []Account
	for rows.Next() {
		a, err := r.scanInto(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, *a)
	}
	return accounts, rows.Err()
}

// UpdateCredential encrypts and updates credential + refresh_token, clears needs_reauth.
func (r *AccountRepo) UpdateCredential(id, accessToken, refreshToken string, expiresAt time.Time) error {
	encAccess, err := crypto.Encrypt(r.encKey, accessToken)
	if err != nil {
		return fmt.Errorf("encrypt credential: %w", err)
	}
	var encRefresh string
	if refreshToken != "" {
		encRefresh, err = crypto.Encrypt(r.encKey, refreshToken)
		if err != nil {
			return fmt.Errorf("encrypt refresh_token: %w", err)
		}
	}

	var tokenExpiresAt interface{}
	if !expiresAt.IsZero() {
		tokenExpiresAt = expiresAt
	}

	result, err := r.db.DB.Exec(
		`UPDATE accounts SET credential = ?, refresh_token = ?, token_expires_at = ?, needs_reauth = 0 WHERE id = ?`,
		encAccess, encRefresh, tokenExpiresAt, id,
	)
	if err != nil {
		return err
	}
	return checkAffected(result, "account")
}

// SetNeedsReauth marks or clears the needs_reauth flag for an account.
func (r *AccountRepo) SetNeedsReauth(id string, needsReauth bool) error {
	result, err := r.db.DB.Exec(
		"UPDATE accounts SET needs_reauth = ? WHERE id = ?",
		needsReauth, id,
	)
	if err != nil {
		return err
	}
	return checkAffected(result, "account")
}

func (r *AccountRepo) Update(id string, label string, enabled bool) error {
	result, err := r.db.DB.Exec(
		"UPDATE accounts SET label = ?, enabled = ? WHERE id = ?",
		label, enabled, id,
	)
	if err != nil {
		return err
	}
	return checkAffected(result, "account")
}

func (r *AccountRepo) Delete(id string) error {
	result, err := r.db.DB.Exec("DELETE FROM accounts WHERE id = ?", id)
	if err != nil {
		return err
	}
	return checkAffected(result, "account")
}

func (r *AccountRepo) SetEnabled(id string, enabled bool) error {
	result, err := r.db.DB.Exec(
		"UPDATE accounts SET enabled = ? WHERE id = ?",
		enabled, id,
	)
	if err != nil {
		return err
	}
	return checkAffected(result, "account")
}
