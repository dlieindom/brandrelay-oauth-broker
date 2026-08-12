# BrandRelay Secure Connect — easiest permanent test setup

This package is designed for Render so you get a stable public HTTPS URL without
running a browser engine or HTTPS server on your Windows PC.

## 1. Put these files in a GitHub repository

Create a repository such as:

    brandrelay-oauth-broker

Upload the contents of this folder to the repository root.

Do NOT upload any real App Secrets or access tokens.

## 2. Deploy the Render Blueprint

In Render:

1. New -> Blueprint
2. Connect the GitHub repository containing this folder.
3. Render reads render.yaml.
4. During first deploy, Render will ask for environment variables.

For the first Facebook-only test you only need to enter:

    META_APP_ID
    META_APP_SECRET

You can leave the other provider values blank.

For BRANDRELAY_PUBLIC_URL, you cannot know the final Render URL until the service
name is established. If Render asks for it during Blueprint creation, enter the
expected service URL:

    https://brandrelay-oauth-broker.onrender.com

If Render changes the service name or adds a suffix, update
BRANDRELAY_PUBLIC_URL after deploy under Render -> Service -> Environment so it
exactly matches the final HTTPS URL.

Render supplies PORT automatically. The broker reads it automatically.

## 3. Confirm the broker is healthy

Open:

    https://YOUR-RENDER-SERVICE.onrender.com/healthz

You should receive JSON containing:

    "ok": true

and Facebook should appear under "configured" when META_APP_ID is present.

## 4. Update Meta / Facebook

In Meta Developers -> BrandRelay -> Facebook Login -> Settings, add this exact
Valid OAuth Redirect URI:

    https://YOUR-RENDER-SERVICE.onrender.com/callback/facebook

Keep Enforce HTTPS enabled.

The redirect URI must be identical to what the broker uses.

## 5. Tell Hall Monitor where the broker lives

Copy brandrelay_cloud.json.template into the same folder as:

    Hall Monitor Native v4.4 SECURE LIVE FEED.exe

Rename it:

    brandrelay_cloud.json

Edit it so it contains your real Render HTTPS base URL, for example:

    {
      "broker_url": "https://brandrelay-oauth-broker.onrender.com"
    }

Restart Hall Monitor.

## 6. Connect Facebook

Hall Monitor -> Accounts -> Facebook -> Sign in with Facebook (Secure)

Expected flow:

    Hall Monitor
      -> BrandRelay Secure Connect
      -> Facebook authorization
      -> HTTPS callback to Render
      -> code/token exchange on the broker
      -> account identity returned to Hall Monitor
      -> Facebook marked Connected
      -> Universal Feed sync begins

## Important

- Do NOT put your Facebook user access token into Render.
- Do NOT put META_APP_SECRET beside Hall Monitor.exe.
- META_APP_SECRET belongs only in Render's encrypted environment settings.
- The App ID may be public; the App Secret must remain private.
- Once Facebook works, add the other provider Client IDs/Secrets in Render and
  register their matching /callback/... URLs.

## Free Render note

Free web services can sleep after inactivity. That is fine for testing, but a
paid always-on service is preferable for a production login broker.
