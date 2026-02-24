package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// jwksFetchClient is a dedicated HTTP client for JWKS refreshes.
// Using http.DefaultClient (no timeout) risks blocking indefinitely under the
// write lock, stalling all concurrent Teams webhook requests during slow Microsoft responses.
var jwksFetchClient = &http.Client{Timeout: 10 * time.Second}

// TeamsAuth validates the Bot Framework JWT Bearer token sent by Microsoft.
//
// Bot Framework sends tokens signed by Microsoft's identity platform.
// The token's "aud" claim must match the bot's App ID, and the signature is
// verified against public keys from Microsoft's Bot Framework OIDC discovery doc.
//
// Reference: https://learn.microsoft.com/en-us/azure/bot-service/rest-api/bot-framework-rest-connector-authentication
func TeamsAuth(appID string) gin.HandlerFunc {
	v := newTeamsTokenValidator(appID)
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Bearer token"})
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		if err := v.validate(c.Request.Context(), tokenStr); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("invalid token: %s", err)})
			return
		}
		c.Next()
	}
}

// ─── Token validator ──────────────────────────────────────────────────────────

const botFrameworkOIDCURL = "https://login.botframework.com/v1/.well-known/openidconfiguration"

type teamsTokenValidator struct {
	appID     string
	keysMu    sync.RWMutex
	keys      jwt.VerificationKeySet
	keysExpAt time.Time
}

func newTeamsTokenValidator(appID string) *teamsTokenValidator {
	return &teamsTokenValidator{appID: appID}
}

func (v *teamsTokenValidator) validate(ctx context.Context, tokenStr string) error {
	ks, err := v.getKeySet(ctx)
	if err != nil {
		return fmt.Errorf("fetch signing keys: %w", err)
	}

	// jwt/v5: Parse(tokenString, keyFunc, opts...)
	// The keyfunc returns (interface{}, error); returning a VerificationKeySet
	// tells jwt to try each key until one validates the signature.
	token, err := jwt.Parse(tokenStr,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return ks, nil
		},
		jwt.WithAudience(v.appID),
		jwt.WithIssuer("https://api.botframework.com"), // prevents cross-service token reuse
		jwt.WithIssuedAt(),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return err
	}
	if !token.Valid {
		return fmt.Errorf("token is not valid")
	}
	return nil
}

// getKeySet returns cached JWKS or fetches fresh keys if the cache is expired.
func (v *teamsTokenValidator) getKeySet(ctx context.Context) (jwt.VerificationKeySet, error) {
	v.keysMu.RLock()
	if time.Now().Before(v.keysExpAt) {
		ks := v.keys
		v.keysMu.RUnlock()
		return ks, nil
	}
	v.keysMu.RUnlock()

	// Refresh
	v.keysMu.Lock()
	defer v.keysMu.Unlock()

	// Double-check after acquiring write lock
	if time.Now().Before(v.keysExpAt) {
		return v.keys, nil
	}

	ks, err := fetchBotFrameworkJWKS(ctx)
	if err != nil {
		return jwt.VerificationKeySet{}, err
	}
	v.keys = ks
	v.keysExpAt = time.Now().Add(24 * time.Hour) // JWKS rotates rarely
	return ks, nil
}

// fetchBotFrameworkJWKS follows the OIDC discovery document to find and parse the JWKS.
func fetchBotFrameworkJWKS(ctx context.Context) (jwt.VerificationKeySet, error) {
	// Step 1: Fetch OIDC discovery doc
	oidc, err := fetchJSON[struct {
		JWKSURI string `json:"jwks_uri"`
	}](ctx, botFrameworkOIDCURL)
	if err != nil {
		return jwt.VerificationKeySet{}, fmt.Errorf("fetch OIDC doc: %w", err)
	}

	// Step 2: Fetch JWKS
	jwks, err := fetchJSON[struct {
		Keys []json.RawMessage `json:"keys"`
	}](ctx, oidc.JWKSURI)
	if err != nil {
		return jwt.VerificationKeySet{}, fmt.Errorf("fetch JWKS: %w", err)
	}

	// Step 3: Parse RSA public keys
	var ks jwt.VerificationKeySet
	for _, rawKey := range jwks.Keys {
		var keyData struct {
			Kty string   `json:"kty"`
			N   string   `json:"n"`
			E   string   `json:"e"`
			Kid string   `json:"kid"`
			X5c []string `json:"x5c"`
		}
		if err := json.Unmarshal(rawKey, &keyData); err != nil {
			slog.Warn("teams auth: failed to unmarshal JWKS key entry, skipping", "error", err)
			continue
		}
		if keyData.Kty != "RSA" || len(keyData.X5c) == 0 {
			continue // non-RSA or missing x5c chain — expected, not an error
		}
		// Parse the PEM certificate from x5c
		certPEM := "-----BEGIN CERTIFICATE-----\n" + keyData.X5c[0] + "\n-----END CERTIFICATE-----"
		key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(certPEM))
		if err != nil {
			slog.Warn("teams auth: failed to parse RSA key from x5c, skipping",
				"kid", keyData.Kid, "error", err)
			continue
		}
		ks.Keys = append(ks.Keys, key)
	}

	if len(ks.Keys) == 0 {
		return jwt.VerificationKeySet{}, fmt.Errorf("no RSA keys found in JWKS")
	}
	return ks, nil
}

func fetchJSON[T any](ctx context.Context, url string) (T, error) {
	var zero T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return zero, err
	}
	resp, err := jwksFetchClient.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, err
	}
	if resp.StatusCode >= 400 {
		return zero, fmt.Errorf("HTTP %d fetching %s: %s", resp.StatusCode, url, string(body))
	}
	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return zero, err
	}
	return result, nil
}
