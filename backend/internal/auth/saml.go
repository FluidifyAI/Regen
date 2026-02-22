package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlsp"
	"github.com/openincident/openincident/internal/config"
	"github.com/openincident/openincident/internal/services"
)

// NewSAMLMiddleware constructs a crewjam/saml Middleware from application config.
// Returns nil, nil when SAML is not configured (SAML_IDP_METADATA_URL is empty).
// Returns nil, err when config is present but invalid (e.g. bad IdP URL).
func NewSAMLMiddleware(cfg *config.Config) (*samlsp.Middleware, error) {
	if cfg.SAMLIDPMetadataURL == "" {
		return nil, nil
	}

	// Production safety checks — fail fast rather than deploy a broken configuration.
	if cfg.IsProduction() {
		if !strings.HasPrefix(cfg.SAMLBaseURL, "https://") {
			return nil, fmt.Errorf("saml: SAML_BASE_URL must use https:// in production (got %q) — session cookies require Secure flag", cfg.SAMLBaseURL)
		}
		if cfg.SAMLCertFile == "" || cfg.SAMLKeyFile == "" {
			return nil, fmt.Errorf("saml: SAML_CERT_FILE and SAML_KEY_FILE are required in production — ephemeral self-signed keypairs regenerate on every restart, invalidating cached IdP metadata")
		}
	}

	// 1. Load or generate SP signing keypair
	keyPair, err := loadOrGenerateKeyPair(cfg.SAMLCertFile, cfg.SAMLKeyFile)
	if err != nil {
		return nil, fmt.Errorf("saml: load keypair: %w", err)
	}

	// 2. Parse base URL (externally reachable URL of this OpenIncident instance)
	baseURL, err := url.Parse(cfg.SAMLBaseURL)
	if err != nil {
		return nil, fmt.Errorf("saml: parse base url %q: %w", cfg.SAMLBaseURL, err)
	}

	// 3. Fetch IdP metadata
	idpMetadataURL, err := url.Parse(cfg.SAMLIDPMetadataURL)
	if err != nil {
		return nil, fmt.Errorf("saml: parse idp metadata url %q: %w", cfg.SAMLIDPMetadataURL, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	idpMetadata, err := samlsp.FetchMetadata(ctx, http.DefaultClient, *idpMetadataURL)
	if err != nil {
		return nil, fmt.Errorf("saml: fetch idp metadata from %s: %w", cfg.SAMLIDPMetadataURL, err)
	}

	// 4. Build the SP middleware
	opts := samlsp.Options{
		URL:         *baseURL,
		Key:         keyPair.PrivateKey.(*rsa.PrivateKey),
		Certificate: keyPair.Leaf,
		IDPMetadata: idpMetadata,
		// SameSite=None is required for SAML POST binding: the IdP issues a
		// cross-site POST back to our ACS endpoint. SameSite=Lax blocks that
		// POST in all modern browsers (Chrome 80+, Firefox, Safari), breaking
		// the tracking cookie that correlates the AuthnRequest to the response.
		// SameSite=None requires HTTPS (Secure flag), enforced above in production.
		CookieSameSite:    http.SameSiteNoneMode,
		CookieName:        "openincident_session",
		AllowIDPInitiated: cfg.SAMLAllowIDPInitiated,
	}
	if cfg.SAMLEntityID != "" {
		opts.EntityID = cfg.SAMLEntityID
	}
	if cfg.SAMLAllowIDPInitiated {
		// IdP-initiated flows skip InResponseTo validation. A stolen or replayed
		// assertion from a different SP can be accepted. Only enable over HTTPS.
		slog.Warn("SAML: IdP-initiated SSO enabled — AuthnRequest replay protection is reduced; HTTPS is required")
	}

	middleware, err := samlsp.New(opts)
	if err != nil {
		return nil, fmt.Errorf("saml: create middleware: %w", err)
	}

	return middleware, nil
}

// NewProvisioningSessionProvider wraps the default CookieSessionProvider so that
// every successful SAML assertion also JIT-provisions the local user record.
func NewProvisioningSessionProvider(
	inner samlsp.SessionProvider,
	authSvc services.AuthService,
) samlsp.SessionProvider {
	return &provisioningSessionProvider{inner: inner, authSvc: authSvc}
}

type provisioningSessionProvider struct {
	inner   samlsp.SessionProvider
	authSvc services.AuthService
}

func (p *provisioningSessionProvider) CreateSession(
	w http.ResponseWriter, r *http.Request, assertion *saml.Assertion,
) error {
	// Guard nil fields that are permitted-but-uncommon by the SAML 2.0 spec.
	if assertion.Subject == nil || assertion.Subject.NameID == nil {
		return fmt.Errorf("saml: assertion missing Subject/NameID — cannot provision user")
	}
	email := samlAttr(assertion,
		"email",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/email",
	)
	name := samlAttr(assertion,
		"displayName",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname",
	)
	subject := assertion.Subject.NameID.Value
	issuer := assertion.Issuer.Value

	// Validate required identity fields before touching the database.
	if subject == "" {
		return fmt.Errorf("saml: assertion NameID is empty — cannot provision user")
	}
	if email == "" {
		return fmt.Errorf("saml: assertion contains no email attribute — ensure IdP sends email/emailaddress claim")
	}

	if err := p.authSvc.UpsertFromSAML(r.Context(), subject, issuer, email, name); err != nil {
		// Provisioning failure blocks session creation. A user with no local
		// record cannot have their role or identity resolved downstream.
		slog.Error("saml: user provisioning failed, aborting session",
			"subject_hash", subjectHash(subject),
			"error", err)
		return fmt.Errorf("saml: user provisioning failed: %w", err)
	}

	// Log a non-reversible hash for log correlation; keep PII at Debug level.
	slog.Info("saml: user provisioned", "subject_hash", subjectHash(subject))
	slog.Debug("saml: user provisioned", "email", email, "subject", subject)

	return p.inner.CreateSession(w, r, assertion)
}

func (p *provisioningSessionProvider) GetSession(r *http.Request) (samlsp.Session, error) {
	return p.inner.GetSession(r)
}

func (p *provisioningSessionProvider) DeleteSession(w http.ResponseWriter, r *http.Request) error {
	return p.inner.DeleteSession(w, r)
}

// subjectHash returns an 8-char hex prefix of SHA-256(subject) for log correlation.
// Short enough to not leak the full subject, long enough to be unique per user.
func subjectHash(subject string) string {
	h := sha256.Sum256([]byte(subject))
	return fmt.Sprintf("%x", h[:4])
}

// samlAttr extracts the first non-empty attribute value by trying each name in order.
func samlAttr(assertion *saml.Assertion, names ...string) string {
	for _, stmt := range assertion.AttributeStatements {
		for _, attr := range stmt.Attributes {
			for _, name := range names {
				if attr.Name == name || attr.FriendlyName == name {
					if len(attr.Values) > 0 {
						return attr.Values[0].Value
					}
				}
			}
		}
	}
	return ""
}

// loadOrGenerateKeyPair loads a TLS keypair from PEM files, or generates a
// self-signed RSA keypair for local development when no files are configured.
func loadOrGenerateKeyPair(certFile, keyFile string) (tls.Certificate, error) {
	if certFile != "" && keyFile != "" {
		kp, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("load keypair from files: %w", err)
		}
		kp.Leaf, err = x509.ParseCertificate(kp.Certificate[0])
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("parse leaf cert: %w", err)
		}
		return kp, nil
	}

	slog.Warn("SAML: no cert/key files configured — generating self-signed keypair (not suitable for production)",
		"hint", "set SAML_CERT_FILE and SAML_KEY_FILE in production")

	return generateSelfSignedKeyPair()
}

func generateSelfSignedKeyPair() (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate rsa key: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"OpenIncident"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour), // 10 years
		// KeyUsageDigitalSignature: sign AuthnRequests sent to the IdP.
		// KeyUsageKeyEncipherment: decrypt encrypted assertions from the IdP.
		// KeyUsageContentCommitment: non-repudiation for XML document signing.
		// No ExtKeyUsage: ExtKeyUsageServerAuth (TLS server) is incorrect for a
		// SAML SP signing cert and rejected by strict IdP validators (e.g. ADFS).
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageContentCommitment,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse generated certificate: %w", err)
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	kp, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create tls keypair: %w", err)
	}
	kp.Leaf = cert
	return kp, nil
}
