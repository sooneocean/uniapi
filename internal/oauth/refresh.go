package oauth

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// RefreshToken refreshes a single account's credential (mutex-protected per account).
// For now marks the account as needs_reauth since real OAuth token exchange is TBD.
func (m *Manager) RefreshToken(accountID string) error {
	mu, _ := m.refreshMu.LoadOrStore(accountID, &sync.Mutex{})
	mutex := mu.(*sync.Mutex)
	if !mutex.TryLock() {
		return nil // refresh already in progress
	}
	defer mutex.Unlock()

	acc, err := m.accountRepo.GetByID(accountID)
	if err != nil {
		return err
	}
	if acc.RefreshToken == "" {
		return fmt.Errorf("no refresh token for account %s", accountID)
	}

	// TODO: Exchange refresh token with provider's token endpoint.
	// For now, mark as needs_reauth since OAuth endpoints are not yet available.
	slog.Warn("token refresh not implemented, marking needs_reauth",
		"account_id", accountID,
		"provider", acc.Provider,
	)
	return m.accountRepo.SetNeedsReauth(accountID, true)
}

// RefreshExpiring proactively refreshes tokens expiring within 5 minutes.
func (m *Manager) RefreshExpiring() error {
	accounts, err := m.accountRepo.ListAll()
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(5 * time.Minute)
	for _, acc := range accounts {
		if acc.AuthType == "api_key" {
			continue
		}
		if acc.NeedsReauth {
			continue
		}
		if acc.TokenExpiresAt != nil && acc.TokenExpiresAt.Before(cutoff) {
			if err := m.RefreshToken(acc.ID); err != nil {
				slog.Error("token refresh failed", "account_id", acc.ID, "error", err)
			}
		}
	}
	return nil
}

// CleanupStates removes expired OAuth state tokens older than 10 minutes.
func (m *Manager) CleanupStates() error {
	cutoff := time.Now().Add(-10 * time.Minute).Format("2006-01-02T15:04:05")
	_, err := m.db.DB.Exec("DELETE FROM oauth_states WHERE created_at < ?", cutoff)
	return err
}
