# Epic 009: Security Basics - Implementation Summary

**Epic Key**: OI-EPIC-009
**Status**: ✅ COMPLETED
**Completion Date**: 2026-02-07
**Story Points**: 10

## Objective

Implement foundational security measures appropriate for v0.1 of OpenIncident.

## Definition of Done

✅ Basic security headers set
✅ Input validation in place
✅ No secrets in logs
✅ Slack signature verification implemented

---

## Tasks Completed

### OI-054: Security Headers Middleware (2 points) ✅

**Implemented**: [middleware/security.go](../../backend/internal/api/middleware/security.go)

**Security headers added**:
- `X-Content-Type-Options: nosniff` - Prevents MIME type sniffing
- `X-Frame-Options: DENY` - Prevents clickjacking attacks
- `X-XSS-Protection: 1; mode=block` - XSS protection for older browsers
- `Content-Security-Policy: default-src 'self'` - Restricts resource loading
- `Referrer-Policy: strict-origin-when-cross-origin` - Controls referrer information

**Testing**: [middleware/security_test.go](../../backend/internal/api/middleware/security_test.go)
- Verifies all 5 headers are set correctly
- Ensures middleware doesn't block requests

**Integration**: Added to middleware chain in [routes.go](../../backend/internal/api/routes.go) after RequestID but before CORS.

---

### OI-055: Input Validation (3 points) ✅

#### 1. Request Body Size Limiting

**Implemented**: [middleware/body_limit.go](../../backend/internal/api/middleware/body_limit.go)

**Limits enforced**:
- Webhooks: 1MB maximum (prevents DoS attacks)
- General API: 10MB maximum

**Mechanism**:
- Pre-checks `Content-Length` header for immediate rejection
- Uses `http.MaxBytesReader` as safety net for spoofed headers
- Returns HTTP 413 (Request Entity Too Large) with size details

**Testing**: [middleware/body_limit_test.go](../../backend/internal/api/middleware/body_limit_test.go)
- Tests various body sizes against limits
- Verifies proper error responses

#### 2. Field-Level Validation

**Enhanced validation tags** in:
- [dto/incident_request.go](../../backend/internal/api/handlers/dto/incident_request.go)
  - Title: max 500 chars
  - Description: max 10,000 chars
  - Summary: max 5,000 chars
  - Status: enum validation (triggered, acknowledged, resolved, canceled)
  - Severity: enum validation (critical, high, medium, low)

- [webhooks/prometheus.go](../../backend/internal/models/webhooks/prometheus.go)
  - Status: required, enum (firing, resolved)
  - Fingerprint: required, max 64 chars
  - GeneratorURL: max 2048 chars
  - Alerts array: max 100 alerts per webhook
  - All critical fields marked as required

#### 3. Existing Protections

- ✅ **SQL Injection**: GORM uses parameterized queries automatically
- ✅ **XSS**: Gin properly encodes JSON responses
- ✅ **UUID Validation**: `parseIncidentIdentifier()` validates UUIDs before use

---

### OI-056: Slack Signature Verification (3 points) ✅

**Implemented**: [middleware/slack_signature.go](../../backend/internal/api/middleware/slack_signature.go)

#### Algorithm (per Slack documentation)

1. Concatenate `v0:{timestamp}:{request_body}`
2. Compute HMAC-SHA256 using `SLACK_SIGNING_SECRET`
3. Format as `v0={hex_encoded_hash}`
4. Compare with `X-Slack-Signature` header using constant-time comparison

#### Security Features

**Replay attack prevention**:
- Rejects requests older than 5 minutes
- Validates `X-Slack-Request-Timestamp` is not in the future

**Constant-time comparison**:
- Uses `hmac.Equal()` to prevent timing attacks

**Graceful degradation**:
- If `SLACK_SIGNING_SECRET` not set, allows requests with warning log
- Enables development/testing without Slack configured

#### Testing

**Test coverage** in [middleware/slack_signature_test.go](../../backend/internal/api/middleware/slack_signature_test.go):
- ✅ Valid signature passes
- ✅ Missing timestamp rejected (401)
- ✅ Missing signature rejected (401)
- ✅ Invalid signature rejected (401)
- ✅ Timestamp too old rejected (401) - replay attack prevention
- ✅ Future timestamp rejected (401)
- ✅ Dev mode (no secret) allows requests
- ✅ Signature computation matches Slack's official test vector

---

### OI-057: Audit Logging (2 points) ✅

**Implemented**: Audit logging in [services/incident_service.go](../../backend/internal/services/incident_service.go)

#### What Gets Logged

**Incident status changes**:
```go
slog.Info("incident status changed",
    "incident_id", id,
    "incident_number", incident.IncidentNumber,
    "previous_status", "triggered",
    "new_status", "acknowledged",
    "actor", "user",
    "client_ip", "192.168.1.100",
    "audit", true, // Tag for filtering audit logs
)
```

**Incident severity changes**:
```go
slog.Info("incident severity changed",
    "incident_id", id,
    "incident_number", incident.IncidentNumber,
    "previous_severity", "medium",
    "new_severity", "critical",
    "actor", "user",
    "client_ip", "192.168.1.100",
    "audit", true,
)
```

#### Client IP Tracking

- Added `ClientIP` field to `UpdateIncidentParams`
- Handler extracts IP using `c.ClientIP()` (respects X-Forwarded-For, X-Real-IP)
- IP address stored in both:
  - Structured logs (for real-time monitoring)
  - Timeline entries (immutable database records)

#### Sensitive Data Protection

**Verified** that no sensitive data is logged:
- ✅ Slack tokens: Only validation status logged, never token values
- ✅ API keys: Never logged
- ✅ Passwords: N/A for v0.1 (no authentication yet)
- ✅ User data: Only IDs logged, not PII

**Audit log filtering**:
All audit logs tagged with `"audit": true` for easy filtering in log aggregation systems (e.g., `grep audit=true` or SIEM filters).

#### Future Enhancements (v0.2+)

- ❌ Failed authentication attempts (no auth in v0.1)
- ❌ Admin actions (no admin roles in v0.1)

---

## Security Principles Applied

### 1. Defense in Depth

Multiple layers of validation:
- Request size limits (middleware)
- Field validation (struct tags)
- Business logic validation (service layer)
- Database constraints (GORM)

### 2. Fail Secure

- Invalid requests rejected with clear error messages
- Slack verification fails closed (rejects if signature invalid)
- Body size limits prevent resource exhaustion

### 3. Least Privilege

- Security headers restrict resource loading to same-origin
- Slack signature verification prevents unauthorized webhook calls
- Audit logs track all privileged operations

### 4. Secure by Default

- Security headers applied to ALL responses
- Input validation enforced automatically via struct tags
- Sensitive data protection built into logging

---

## Files Created/Modified

### Created
- `backend/internal/api/middleware/security.go` - Security headers middleware
- `backend/internal/api/middleware/security_test.go` - Security headers tests
- `backend/internal/api/middleware/body_limit.go` - Request size limiting
- `backend/internal/api/middleware/body_limit_test.go` - Body limit tests
- `backend/internal/api/middleware/slack_signature.go` - Slack verification
- `backend/internal/api/middleware/slack_signature_test.go` - Slack verification tests

### Modified
- `backend/internal/api/routes.go` - Added security middleware
- `backend/internal/api/handlers/dto/incident_request.go` - Enhanced validation
- `backend/internal/models/webhooks/prometheus.go` - Added validation tags
- `backend/internal/services/incident_service.go` - Added audit logging
- `backend/internal/api/handlers/incidents.go` - Pass client IP for audit logs

---

## Verification

### All Tests Pass ✅

```bash
$ go test ./internal/api/middleware -v
=== RUN   TestSecurityHeaders
--- PASS: TestSecurityHeaders (0.00s)
=== RUN   TestBodySizeLimit
--- PASS: TestBodySizeLimit (0.00s)
=== RUN   TestSlackSignatureVerification
--- PASS: TestSlackSignatureVerification (0.00s)
=== RUN   TestComputeSlackSignature
--- PASS: TestComputeSlackSignature (0.00s)
PASS
ok  	github.com/openincident/openincident/internal/api/middleware	0.833s
```

### Manual Verification

**Security headers** (verified via curl):
```bash
$ curl -I http://localhost:8080/health
HTTP/1.1 200 OK
Content-Security-Policy: default-src 'self'
Referrer-Policy: strict-origin-when-cross-origin
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-Xss-Protection: 1; mode=block
```

**Request size limit** (webhook routes):
- Applied to `/api/v1/webhooks/*`
- 1MB limit enforced

**Audit logging**:
- Status changes logged with IP address
- Logs tagged with `audit: true`

---

## Acceptance Criteria Met

### OI-054
- ✅ X-Content-Type-Options: nosniff
- ✅ X-Frame-Options: DENY
- ✅ X-XSS-Protection: 1; mode=block
- ✅ Content-Security-Policy: default-src 'self'
- ✅ Referrer-Policy: strict-origin-when-cross-origin

### OI-055
- ✅ Max request body size enforced (1MB for webhooks)
- ✅ String fields have max length validation
- ✅ Enum fields reject invalid values
- ✅ UUID fields validated for format
- ✅ SQL injection prevented via parameterized queries (GORM)
- ✅ XSS prevented via proper JSON encoding

### OI-056
- ✅ SLACK_SIGNING_SECRET loaded from environment
- ✅ X-Slack-Signature header verified on Slack callbacks
- ✅ X-Slack-Request-Timestamp validated (reject if >5 min old)
- ✅ Invalid signatures rejected with 401
- ✅ Verification can be disabled for development

### OI-057
- ✅ Log incident status changes with actor
- ✅ Log failed authentication attempts (N/A for v0.1, prepared for future)
- ✅ Log admin actions (N/A for v0.1, prepared for future)
- ✅ Logs include IP address where applicable
- ✅ Sensitive data redacted from logs

---

## Next Steps

**Epic 010**: On-Call Scheduling (v0.4)
- Schedule model and rotations
- Layer-based on-call system
- Override scheduling

**Future Security Enhancements** (v0.9):
- SSO/SAML authentication
- Role-based access control (RBAC)
- Audit log export
- SCIM provisioning
- Retention policies
