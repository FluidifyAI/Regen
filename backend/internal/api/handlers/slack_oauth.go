package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/openincident/openincident/internal/repository"
	"github.com/openincident/openincident/internal/services"
)

const slackOAuthStateKey = "slack_oauth_state"

// InitSlackOAuth redirects the browser to Slack's OpenID Connect authorization endpoint.
func InitSlackOAuth(slackRepo repository.SlackConfigRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := slackRepo.Get()
		if err != nil || cfg == nil || cfg.OAuthClientID == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "slack oauth not configured"})
			return
		}

		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate state"})
			return
		}
		state := base64.RawURLEncoding.EncodeToString(b)
		http.SetCookie(c.Writer, &http.Cookie{
			Name:     slackOAuthStateKey,
			Value:    state,
			MaxAge:   300,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		redirectURI := slackCallbackURI(c)
		authURL := "https://slack.com/openid/connect/authorize?" + url.Values{
			"client_id":     {cfg.OAuthClientID},
			"scope":         {"openid,email,profile"},
			"response_type": {"code"},
			"redirect_uri":  {redirectURI},
			"state":         {state},
		}.Encode()

		c.Redirect(http.StatusFound, authURL)
	}
}

// SlackOAuthCallback handles Slack's redirect after the user approves the OAuth prompt.
// It exchanges the code for an ID token, looks up the user by email, and creates a session.
func SlackOAuthCallback(slackRepo repository.SlackConfigRepository, localAuth services.LocalAuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// CSRF state validation
		stateCookie, err := c.Cookie(slackOAuthStateKey)
		if err != nil || stateCookie != c.Query("state") {
			c.Redirect(http.StatusFound, "/login?error=invalid_state")
			return
		}
		// Clear the state cookie
		http.SetCookie(c.Writer, &http.Cookie{Name: slackOAuthStateKey, MaxAge: -1, Path: "/"})

		cfg, err := slackRepo.Get()
		if err != nil || cfg == nil || cfg.OAuthClientID == "" {
			c.Redirect(http.StatusFound, "/login?error=slack_not_configured")
			return
		}

		code := c.Query("code")
		if code == "" {
			c.Redirect(http.StatusFound, "/login?error=no_code")
			return
		}

		email, _, err := exchangeSlackCode(code, cfg.OAuthClientID, cfg.OAuthClientSecret, slackCallbackURI(c))
		if err != nil {
			c.Redirect(http.StatusFound, "/login?error=slack_auth_failed")
			return
		}

		user, err := localAuth.GetUserByEmail(email)
		if err != nil || user == nil {
			c.Redirect(http.StatusFound, "/login?error=no_account")
			return
		}

		session, err := localAuth.LoginByUserID(user.ID)
		if err != nil {
			c.Redirect(http.StatusFound, "/login?error=session_failed")
			return
		}
		setSessionCookie(c, session.Token, 7*24*3600)
		c.Redirect(http.StatusFound, "/")
	}
}

// slackCallbackURI builds the redirect_uri from the incoming request.
func slackCallbackURI(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host + "/api/v1/auth/slack/callback"
}

// exchangeSlackCode exchanges an OAuth2 authorization code for the user's email via Slack OpenID.
func exchangeSlackCode(code, clientID, clientSecret, redirectURI string) (email, name string, err error) {
	resp, err := http.PostForm("https://slack.com/api/openid.connect.token", url.Values{
		"code":          {code},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		return "", "", fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var result struct {
		OK      bool   `json:"ok"`
		Error   string `json:"error"`
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse token response: %w", err)
	}
	if !result.OK {
		return "", "", fmt.Errorf("slack token exchange failed: %s", result.Error)
	}

	return parseSlackIDToken(result.IDToken)
}

// parseSlackIDToken decodes the JWT payload to extract email and name.
// No signature verification needed — the token came from Slack over TLS.
func parseSlackIDToken(idToken string) (email, name string, err error) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid id_token format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("failed to decode id_token payload: %w", err)
	}
	var claims struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", "", fmt.Errorf("failed to parse id_token claims: %w", err)
	}
	if claims.Email == "" {
		return "", "", fmt.Errorf("no email in id_token — ensure email scope is granted")
	}
	return claims.Email, claims.Name, nil
}
