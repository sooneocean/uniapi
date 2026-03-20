package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/user/uniapi/internal/audit"
	"github.com/user/uniapi/internal/oauth"
	"github.com/user/uniapi/internal/repo"
	"github.com/user/uniapi/internal/router"
)

// OAuthHandler handles OAuth and session token binding endpoints.
type OAuthHandler struct {
	manager         *oauth.Manager
	router          *router.Router
	registerAccount func(acc *repo.Account)
	audit           *audit.Logger
}

// NewOAuthHandler creates a new OAuthHandler.
func NewOAuthHandler(mgr *oauth.Manager, rtr *router.Router, registerFn func(acc *repo.Account), auditLogger *audit.Logger) *OAuthHandler {
	return &OAuthHandler{manager: mgr, router: rtr, registerAccount: registerFn, audit: auditLogger}
}

// ListProviders handles GET /api/oauth/providers
func (h *OAuthHandler) ListProviders(c *gin.Context) {
	providers := h.manager.ListProviders()
	c.JSON(200, providers)
}

// Authorize handles GET /api/oauth/bind/:provider/authorize
// Requires JWT auth; requires admin if shared=true.
func (h *OAuthHandler) Authorize(c *gin.Context) {
	providerName := c.Param("provider")
	shared := c.Query("shared") == "true"

	if shared {
		role, _ := c.Get("role")
		if r, ok := role.(string); !ok || r != "admin" {
			c.JSON(403, gin.H{"error": "admin required for shared binding"})
			return
		}
	}

	// Get session hash from JWT cookie or Authorization header
	token, _ := c.Cookie("token")
	if token == "" {
		token = ExtractBearerToken(c)
	}
	sessionHash := oauth.HashSession(token)

	uid, _ := c.Get("user_id")
	userID, _ := uid.(string)

	url, err := h.manager.AuthorizeURL(providerName, userID, sessionHash, shared)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Redirect(http.StatusFound, url)
}

// Callback handles GET /api/oauth/callback/:provider (NO JWT auth — uses state token).
// Returns HTML with postMessage to close the popup window.
func (h *OAuthHandler) Callback(c *gin.Context) {
	providerName := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")

	// Get session hash from cookie (browser still has it during OAuth flow)
	token, _ := c.Cookie("token")
	sessionHash := oauth.HashSession(token)

	_, err := h.manager.HandleCallback(providerName, code, state, sessionHash)

	baseJSON, _ := json.Marshal(h.manager.BaseURL())
	if err != nil {
		errJSON, _ := json.Marshal(err.Error())
		c.Header("Content-Type", "text/html")
		c.String(200, fmt.Sprintf(`<html><body><script>
            window.opener.postMessage('oauth-error:'+%s, %s);
            window.close();
        </script></body></html>`, string(errJSON), string(baseJSON)))
		return
	}
	c.Header("Content-Type", "text/html")
	c.String(200, fmt.Sprintf(`<html><body><script>
        window.opener.postMessage('oauth-done', %s);
        window.close();
    </script></body></html>`, string(baseJSON)))
}

// BindSessionToken handles POST /api/oauth/bind/:provider/session-token
// Requires JWT auth; requires admin if shared=true.
func (h *OAuthHandler) BindSessionToken(c *gin.Context) {
	providerName := c.Param("provider")
	var req struct {
		Token  string `json:"token" binding:"required"`
		Shared bool   `json:"shared"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if req.Shared {
		role, _ := c.Get("role")
		if r, ok := role.(string); !ok || r != "admin" {
			c.JSON(403, gin.H{"error": "admin required for shared binding"})
			return
		}
	}

	uid, _ := c.Get("user_id")
	userID, _ := uid.(string)

	acc, err := h.manager.BindSessionToken(providerName, userID, req.Token, req.Shared)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Dynamically register the new account with the live router
	if h.registerAccount != nil {
		h.registerAccount(acc)
	}

	if h.audit != nil {
		h.audit.Log(userID, "", "bind_account", "account", acc.ID, providerName, c.ClientIP())
	}

	c.JSON(200, gin.H{"ok": true, "account": gin.H{
		"id":       acc.ID,
		"provider": acc.Provider,
		"label":    acc.Label,
	}})
}

// ListAccounts handles GET /api/oauth/accounts
// Returns sanitized account list (no credentials).
func (h *OAuthHandler) ListAccounts(c *gin.Context) {
	uid, _ := c.Get("user_id")
	userID, _ := uid.(string)

	accounts, err := h.manager.ListAccounts(userID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	result := make([]gin.H, len(accounts))
	for i, a := range accounts {
		entry := gin.H{
			"id":            a.ID,
			"provider":      a.Provider,
			"label":         a.Label,
			"auth_type":     a.AuthType,
			"models":        a.Models,
			"owner_user_id": a.OwnerUserID,
			"needs_reauth":  a.NeedsReauth,
			"enabled":       a.Enabled,
		}
		if a.TokenExpiresAt != nil {
			entry["token_expires_at"] = a.TokenExpiresAt
		}
		result[i] = entry
	}
	c.JSON(200, result)
}

// UnbindAccount handles DELETE /api/oauth/accounts/:id
func (h *OAuthHandler) UnbindAccount(c *gin.Context) {
	accountID := c.Param("id")
	uid, _ := c.Get("user_id")
	userID, _ := uid.(string)

	role := ""
	if r, exists := c.Get("role"); exists {
		role, _ = r.(string)
	}

	if err := h.manager.Unbind(accountID, userID, role); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if h.audit != nil {
		h.audit.Log(userID, "", "unbind_account", "account", accountID, "", c.ClientIP())
	}

	c.JSON(200, gin.H{"ok": true})
}

// Reauth handles POST /api/oauth/accounts/:id/reauth
// Returns either an authorize URL (for OAuth accounts) or a prompt to paste a new session token.
func (h *OAuthHandler) Reauth(c *gin.Context) {
	accountID := c.Param("id")
	uid, _ := c.Get("user_id")
	userID, _ := uid.(string)

	acc, err := h.manager.GetAccount(accountID, userID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if acc.AuthType == "oauth" && acc.OAuthProvider != "" {
		token, _ := c.Cookie("token")
		if token == "" {
			token = ExtractBearerToken(c)
		}
		sessionHash := oauth.HashSession(token)
		url, err := h.manager.AuthorizeURL(acc.OAuthProvider, userID, sessionHash, acc.OwnerUserID == "")
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"action": "oauth", "authorize_url": url})
		return
	}

	// Session token account: prompt user to provide a new token
	c.JSON(200, gin.H{"action": "session_token", "provider": acc.Provider})
}
