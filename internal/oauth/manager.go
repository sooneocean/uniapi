package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sooneocean/uniapi/internal/config"
	"github.com/sooneocean/uniapi/internal/db"
	"github.com/sooneocean/uniapi/internal/repo"
)

// Manager handles OAuth and session token binding flows.
type Manager struct {
	providers   map[string]*BindingProvider
	db          *db.Database
	accountRepo *repo.AccountRepo
	encKey      []byte
	baseURL     string
	oauthCfg    config.OAuthConfigs
	refreshMu   sync.Map
}

// NewManager creates a new OAuth Manager, enabling OAuth for providers
// where client credentials are configured.
func NewManager(database *db.Database, accountRepo *repo.AccountRepo, encKey []byte, baseURL string, oauthCfg config.OAuthConfigs) *Manager {
	providers := make(map[string]*BindingProvider)
	for k, v := range defaultProviders {
		p := *v // copy
		switch k {
		case "openai":
			if oauthCfg.OpenAI != nil && oauthCfg.OpenAI.ClientID != "" {
				p.SupportsOAuth = true
			}
		case "aliyun":
			if oauthCfg.Qwen == nil || oauthCfg.Qwen.ClientID == "" {
				p.SupportsOAuth = false
			}
		case "anthropic":
			if oauthCfg.Claude != nil && oauthCfg.Claude.ClientID != "" {
				p.SupportsOAuth = true
			}
		}
		providers[k] = &p
	}
	return &Manager{
		providers:   providers,
		db:          database,
		accountRepo: accountRepo,
		encKey:      encKey,
		baseURL:     baseURL,
		oauthCfg:    oauthCfg,
	}
}

// BaseURL returns the configured external base URL.
func (m *Manager) BaseURL() string { return m.baseURL }

func (m *Manager) getClientCredentials(providerName string) (clientID, clientSecret string) {
	switch providerName {
	case "openai":
		if m.oauthCfg.OpenAI != nil {
			return m.oauthCfg.OpenAI.ClientID, m.oauthCfg.OpenAI.ClientSecret
		}
	case "aliyun":
		if m.oauthCfg.Qwen != nil {
			return m.oauthCfg.Qwen.ClientID, m.oauthCfg.Qwen.ClientSecret
		}
	case "anthropic":
		if m.oauthCfg.Claude != nil {
			return m.oauthCfg.Claude.ClientID, m.oauthCfg.Claude.ClientSecret
		}
	}
	return "", ""
}

// ListProviders returns all available binding providers.
func (m *Manager) ListProviders() []*BindingProvider {
	result := make([]*BindingProvider, 0, len(m.providers))
	for _, p := range m.providers {
		p := p // copy pointer for safety
		result = append(result, p)
	}
	return result
}

// GetAccount retrieves an account by ID, verifying ownership.
func (m *Manager) GetAccount(accountID, userID string) (*repo.Account, error) {
	acc, err := m.accountRepo.GetByID(accountID)
	if err != nil {
		return nil, err
	}
	if acc.OwnerUserID != "" && acc.OwnerUserID != userID {
		return nil, fmt.Errorf("not authorized")
	}
	return acc, nil
}

// AuthorizeURL generates an OAuth authorization URL and stores a one-time state token.
func (m *Manager) AuthorizeURL(providerName, userID, sessionHash string, shared bool) (string, error) {
	p, ok := m.providers[providerName]
	if !ok {
		return "", fmt.Errorf("unknown provider: %s", providerName)
	}
	if !p.SupportsOAuth {
		return "", fmt.Errorf("provider %s does not support OAuth", providerName)
	}

	state, err := generateState()
	if err != nil {
		return "", fmt.Errorf("authorize url: %w", err)
	}
	_, err = m.db.DB.Exec(
		"INSERT INTO oauth_states (state, provider, user_id, session_hash, shared) VALUES (?, ?, ?, ?, ?)",
		state, providerName, userID, sessionHash, shared,
	)
	if err != nil {
		return "", fmt.Errorf("store oauth state: %w", err)
	}

	// Get client_id for provider
	clientID, _ := m.getClientCredentials(providerName)

	redirectURI := fmt.Sprintf("%s/api/oauth/callback/%s", m.baseURL, providerName)

	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", strings.Join(p.Scopes, " "))
	params.Set("state", state)

	authorizeURL := p.AuthURL + "?" + params.Encode()
	return authorizeURL, nil
}

// HandleCallback validates state+session, exchanges code for token, and creates an account.
func (m *Manager) HandleCallback(providerName, code, state, sessionHash string) (*repo.Account, error) {
	// 1. Validate state exists
	var storedProvider, storedUserID, storedSessionHash string
	var shared bool
	err := m.db.DB.QueryRow(
		"SELECT provider, user_id, session_hash, shared FROM oauth_states WHERE state = ?",
		state,
	).Scan(&storedProvider, &storedUserID, &storedSessionHash, &shared)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired state")
	}

	// 2. Verify provider matches
	if storedProvider != providerName {
		return nil, fmt.Errorf("state provider mismatch")
	}

	// 3. Verify session hash matches
	if storedSessionHash != sessionHash {
		return nil, fmt.Errorf("session mismatch")
	}

	// 4. Delete state (one-time use)
	m.db.DB.Exec("DELETE FROM oauth_states WHERE state = ?", state)

	// 5. Get provider config
	p, ok := m.providers[providerName]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", providerName)
	}

	// 6. Exchange code for token
	accessToken, refreshToken, expiresIn, err := m.exchangeCode(providerName, code)
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}

	ownerUserID := storedUserID
	if shared {
		ownerUserID = ""
	}

	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	return m.accountRepo.CreateBound(
		p.ProviderType,
		fmt.Sprintf("%s (OAuth)", p.DisplayName),
		"oauth",
		providerName,
		accessToken,
		refreshToken,
		expiresAt,
		p.DefaultModels,
		5,
		ownerUserID,
		false,
	)
}

func (m *Manager) exchangeCode(providerName, code string) (accessToken, refreshToken string, expiresIn int, err error) {
	p, ok := m.providers[providerName]
	if !ok {
		return "", "", 0, fmt.Errorf("unknown provider")
	}
	if p.TokenURL == "" {
		return "", "", 0, fmt.Errorf("OAuth token exchange not configured for %s", providerName)
	}

	// Get client credentials
	clientID, clientSecret := m.getClientCredentials(providerName)

	redirectURI := fmt.Sprintf("%s/api/oauth/callback/%s", m.baseURL, providerName)

	// Standard OAuth2 token exchange
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", redirectURI)
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)

	resp, err := http.PostForm(p.TokenURL, data)
	if err != nil {
		return "", "", 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", "", 0, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", "", 0, fmt.Errorf("parse token response: %w", err)
	}

	if tokenResp.ExpiresIn == 0 {
		tokenResp.ExpiresIn = 3600 // default 1 hour
	}

	return tokenResp.AccessToken, tokenResp.RefreshToken, tokenResp.ExpiresIn, nil
}

// BindSessionToken stores a user-provided session token as a new account.
func (m *Manager) BindSessionToken(providerName, userID, token string, shared bool) (*repo.Account, error) {
	p, ok := m.providers[providerName]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", providerName)
	}

	ownerUserID := userID
	if shared {
		ownerUserID = ""
	}

	return m.accountRepo.CreateBound(
		p.ProviderType,
		fmt.Sprintf("%s (session)", p.DisplayName),
		"session_token",
		providerName,
		token,
		"",          // no refresh token for session tokens
		time.Time{}, // no expiry
		p.DefaultModels,
		5,
		ownerUserID,
		false,
	)
}

// ListAccounts returns accounts visible to the given user.
func (m *Manager) ListAccounts(userID string) ([]repo.Account, error) {
	return m.accountRepo.ListForUser(userID)
}

// Unbind removes an OAuth/session account after ownership verification.
func (m *Manager) Unbind(accountID, userID, role string) error {
	acc, err := m.accountRepo.GetByID(accountID)
	if err != nil {
		return err
	}
	if acc.AuthType == "api_key" {
		return fmt.Errorf("cannot unbind API key accounts")
	}
	if acc.OwnerUserID != "" && acc.OwnerUserID != userID && role != "admin" {
		return fmt.Errorf("not authorized")
	}
	return m.accountRepo.Delete(accountID)
}

// generateState creates a cryptographically random state token.
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// HashSession returns the SHA-256 hex hash of a JWT string.
func HashSession(jwt string) string {
	h := sha256.Sum256([]byte(jwt))
	return hex.EncodeToString(h[:])
}
