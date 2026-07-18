# Runbook: SAML misconfigured

**Symptom:** Clicking "Sign in with SSO" redirects to an error page, loops back to the login screen, or shows a `403 Forbidden`. Users cannot log in via SSO.

---

## Diagnose

**1. Check app logs during a login attempt:**

```bash
# Docker Compose
docker logs fluidify-regen --since 5m 2>&1 | grep -i "saml\|sso\|auth\|assertion"

# Kubernetes
kubectl logs -n fluidify deploy/fluidify-regen --since=5m | grep -i "saml\|sso\|auth\|assertion"
```

Common patterns:
- `saml: failed to validate response` — clock skew or wrong IdP certificate
- `saml: invalid ACS URL` — the ACS URL in the IdP doesn't match Regen's actual URL
- `saml: no email attribute in assertion` — IdP is not sending the email claim
- `saml: certificate expired` — the IdP signing certificate has expired

**2. Check the SAML configuration in Regen:**

Go to **Settings → Authentication → SAML SSO**. Verify:
- **Metadata URL** or **Metadata XML** is populated
- **ACS URL** shown matches what's configured in your IdP
- **Entity ID** matches what the IdP expects

**3. Test with clock skew:**

SAML assertions have a validity window (typically ±5 minutes). If the server clock drifts, assertions fail. Check server time:

```bash
date -u
```

Compare against `https://time.is/UTC`. More than 1 minute of drift causes failures.

---

## Common causes and fixes

**ACS URL mismatch**

The IdP must post the SAML assertion to Regen's ACS URL exactly. It is:

```
https://your-regen-host/api/v1/auth/saml/acs
```

Ensure `APP_URL` is set to your public-facing base URL — Regen constructs the ACS URL from it:

```env
APP_URL=https://incidents.yourcompany.com
```

Do not include a trailing slash.

**Wrong Entity ID**

Regen's entity ID (SP entity ID) is:

```
https://your-regen-host/api/v1/auth/saml/metadata
```

Configure this exactly in your IdP as the "Audience URI" or "SP Entity ID".

**IdP certificate expired**

Download the latest IdP metadata XML from your IdP and re-paste it into **Settings → Authentication → SAML SSO → Metadata XML**, or update the Metadata URL so Regen re-fetches it automatically.

For Okta: go to **Applications → your app → Sign On → Identity Provider metadata** and download fresh metadata.
For Azure AD: download from `https://login.microsoftonline.com/<tenant-id>/federationmetadata/2007-06/federationmetadata.xml`.

**Email attribute not mapped**

Regen requires the email address to be present in the SAML assertion. Ensure your IdP is configured to include the email attribute:

| IdP | Attribute name |
|-----|----------------|
| Okta | `user.email` → `email` |
| Azure AD | `user.mail` → `http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress` |
| Google Workspace | `Basic Information > Primary email` |

**Clock skew**

```bash
# Sync server clock (Linux)
sudo timedatectl set-ntp true
sudo systemctl restart systemd-timesyncd
timedatectl status
```

For Docker, the container inherits the host clock — fix the host.

---

## Emergency bypass

If SAML is broken and users are locked out, disable SAML temporarily to allow local login:

```env
SAML_ENABLED=false
```

Restart the app. Local accounts (username/password) will work again. Re-enable SAML once the IdP configuration is fixed.

---

## Resolve

1. Re-test the SSO login flow end-to-end
2. Check logs confirm no SAML errors during the assertion
3. Re-enable SAML if you temporarily disabled it
4. Notify users once SSO is restored

---

## Prevention

- Set a calendar reminder to check IdP certificate expiry — most certs are valid for 1–3 years
- Test SSO login after any IdP configuration change before communicating it to users
- Keep at least one local admin account active as a break-glass, even when SAML is enabled
