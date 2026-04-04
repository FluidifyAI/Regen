# SAML SSO

SAML 2.0 Single Sign-On is included in the Community edition — free, no enterprise license required.

Regen acts as a SAML Service Provider (SP). It works with any standards-compliant Identity Provider (IdP): Okta, Azure AD (Entra ID), Google Workspace, OneLogin, and others.

When SSO is configured, users sign in via your IdP. Accounts are provisioned automatically on first login (JIT provisioning). Local username/password login continues to work alongside SSO.

## Prerequisites

- Regen must be accessible over HTTPS (required by SAML)
- You need admin access to your IdP

## Step 1: Configure Regen

Add to your `.env`:

```env
SAML_IDP_METADATA_URL=https://your-idp.com/metadata
SAML_BASE_URL=https://incidents.yourcompany.com
```

`SAML_BASE_URL` must be the externally reachable URL of your Regen instance. This is used to build the SP metadata, ACS URL, and redirect URLs.

Restart: `make stop && make start`

## Step 2: Get the SP metadata

After restarting, fetch your SP metadata:

```
https://incidents.yourcompany.com/saml/metadata
```

You'll need this URL (or the XML content) when configuring your IdP.

## Okta

1. In Okta admin console, go to **Applications → Create App Integration**
2. Select **SAML 2.0**
3. Set **App name**: `Fluidify Regen`
4. Set **Single sign on URL**: `https://incidents.yourcompany.com/saml/acs`
5. Set **Audience URI (SP Entity ID)**: `https://incidents.yourcompany.com/saml/metadata`
6. Set **Name ID format**: `EmailAddress`
7. Add attribute statement: `email` → `user.email`
8. Finish and copy the **Identity Provider metadata URL**
9. Set `SAML_IDP_METADATA_URL` to that URL

**For Okta tile (IdP-initiated) login:**
```env
SAML_ALLOW_IDP_INITIATED=true
```

## Azure AD / Entra ID

1. In Azure portal, go to **Enterprise Applications → New application → Create your own**
2. Select **Integrate any other application you don't find in the gallery**
3. Go to **Single sign-on → SAML**
4. Set **Identifier (Entity ID)**: `https://incidents.yourcompany.com/saml/metadata`
5. Set **Reply URL (ACS URL)**: `https://incidents.yourcompany.com/saml/acs`
6. Set **Sign on URL**: `https://incidents.yourcompany.com`
7. Under **Attributes & Claims**, ensure `emailaddress` maps to `user.mail`
8. Copy the **App Federation Metadata URL** (under SAML Signing Certificate)
9. Set `SAML_IDP_METADATA_URL` to that URL

## Google Workspace

1. In Google Admin console, go to **Apps → Web and mobile apps → Add app → Add custom SAML app**
2. Name it `Fluidify Regen`
3. Download the IdP metadata XML and host it somewhere accessible (Google doesn't provide a metadata URL in all configurations), or use the provided metadata URL
4. Set **ACS URL**: `https://incidents.yourcompany.com/saml/acs`
5. Set **Entity ID**: `https://incidents.yourcompany.com/saml/metadata`
6. Add attribute mapping: `Primary email` → `email`
7. Set `SAML_IDP_METADATA_URL` to the metadata URL

## Custom SP certificate

By default, Regen generates a self-signed certificate for the SP. For production, provide your own:

```env
SAML_CERT_FILE=/app/saml/sp.crt
SAML_KEY_FILE=/app/saml/sp.key
```

Mount the files into the container in `docker-compose.yml`:

```yaml
volumes:
  - ./saml/sp.crt:/app/saml/sp.crt:ro
  - ./saml/sp.key:/app/saml/sp.key:ro
```

## Testing the configuration

1. Open a private/incognito browser window
2. Go to `https://incidents.yourcompany.com/login`
3. Click **Sign in with SSO**
4. You should be redirected to your IdP
5. After authenticating, you should land back at Regen as a logged-in user

First-time users are automatically created with the `user` role. Promote to admin under **Settings → Users**.

## Disabling SSO

Remove `SAML_IDP_METADATA_URL` from your `.env` and restart. Local login continues to work.
