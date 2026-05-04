package licence

import (
	"crypto/ed25519"
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/golang-jwt/jwt/v5"
)

//go:embed keys/public.pem
var publicKeyPEM []byte

// testPublicKeyPEM overrides publicKeyPEM during tests. Set via SetPublicKeyForTest.
var testPublicKeyPEM []byte

var (
	ErrNoLicence     = errors.New("no licence key provided")
	ErrExpired       = errors.New("licence key has expired")
	ErrInvalidIssuer = errors.New("licence key issuer is invalid")
	ErrMalformed     = errors.New("licence key is malformed")
)

// Licence holds the decoded, verified claims from a Pro licence key.
type Licence struct {
	CustomerID string
	OrgName    string
	Seats      int
	Features   []string
}

// SeatLimitExceeded returns true when activeUsers is within the seat limit.
// Returns false when the limit is exceeded (caller should warn, not crash).
func (l *Licence) SeatLimitExceeded(activeUsers int) bool {
	return activeUsers <= l.Seats
}

func activePublicKeyPEM() []byte {
	if len(testPublicKeyPEM) > 0 {
		return testPublicKeyPEM
	}
	return publicKeyPEM
}

// Load parses and cryptographically verifies a licence key string.
// Returns ErrNoLicence for empty input, ErrExpired for expired keys,
// ErrInvalidIssuer for wrong issuer, ErrMalformed for missing required claims.
// Any other error indicates a forged or corrupted token.
func Load(token string) (*Licence, error) {
	if token == "" {
		return nil, ErrNoLicence
	}

	pubKey, err := parsePublicKey(activePublicKeyPEM())
	if err != nil {
		return nil, fmt.Errorf("load public key: %w", err)
	}

	parsed, err := jwt.Parse(token,
		func(t *jwt.Token) (interface{}, error) {
			// Pin the algorithm — reject anything that isn't EdDSA.
			// This prevents algorithm confusion attacks (e.g. HS256 with public key as secret).
			if _, ok := t.Method.(*jwt.SigningMethodEd25519); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return pubKey, nil
		},
		jwt.WithIssuedAt(),
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpired
		}
		return nil, err
	}

	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, ErrMalformed
	}

	iss, _ := claims.GetIssuer()
	if iss != "fluidify" {
		return nil, ErrInvalidIssuer
	}

	return extractLicence(claims)
}

func extractLicence(claims jwt.MapClaims) (*Licence, error) {
	customerID, _ := claims["customer_id"].(string)
	orgName, _ := claims["org_name"].(string)
	seatsF, _ := claims["seats"].(float64)
	seats := int(seatsF)

	if customerID == "" || orgName == "" || seats <= 0 {
		return nil, ErrMalformed
	}

	var features []string
	if raw, ok := claims["features"].([]interface{}); ok {
		for _, f := range raw {
			if s, ok := f.(string); ok {
				features = append(features, s)
			}
		}
	}

	return &Licence{
		CustomerID: customerID,
		OrgName:    orgName,
		Seats:      seats,
		Features:   features,
	}, nil
}

func parsePublicKey(pemBytes []byte) (ed25519.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("failed to decode public key PEM")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ed, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("public key is not Ed25519")
	}
	return ed, nil
}

// --- Global state (initialised once at startup via Init) ---

var (
	mu      sync.RWMutex
	current *Licence
)

// Init loads the licence key from the environment, validates it, logs the result,
// and stores it in global state. activeUsers is the current DB user count for seat check.
// Call once at startup after the DB is ready.
func Init(token string, activeUsers int) {
	if token == "" {
		slog.Info("licence: no REGEN_LICENCE_KEY set — running in OSS mode")
		return
	}

	lic, err := Load(token)
	if err != nil {
		slog.Warn("licence: invalid key, falling back to OSS mode", "error", err)
		return
	}

	slog.Info("licence: Pro activated",
		"org", lic.OrgName,
		"customer_id", lic.CustomerID,
		"seats", lic.Seats,
		"features", lic.Features,
	)

	if !lic.SeatLimitExceeded(activeUsers) {
		slog.Warn("licence: active user count exceeds seat limit",
			"active_users", activeUsers,
			"seats", lic.Seats,
			"org", lic.OrgName,
		)
	}

	mu.Lock()
	current = lic
	mu.Unlock()
}

// IsProEnabled returns true if a valid, non-expired Pro licence is active.
func IsProEnabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return current != nil
}

// HasFeature returns true if the active licence includes the named feature.
func HasFeature(feature string) bool {
	mu.RLock()
	defer mu.RUnlock()
	if current == nil {
		return false
	}
	for _, f := range current.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// SetPublicKeyForTest overrides the embedded production public key for unit tests.
// Pass nil to restore the production key.
func SetPublicKeyForTest(pemBytes []byte) {
	mu.Lock()
	defer mu.Unlock()
	testPublicKeyPEM = pemBytes
}

// SetForTest replaces the global licence state for unit tests.
// Pass an empty string to reset to OSS mode.
func SetForTest(token string) {
	mu.Lock()
	defer mu.Unlock()
	if token == "" {
		current = nil
		return
	}
	lic, err := Load(token)
	if err != nil {
		current = nil
		return
	}
	current = lic
}
