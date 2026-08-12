# BrandRelay all-social setup — v4.5

Your working Secure Connect base URL is:

    https://brandrelay-oauth-broker.onrender.com

Facebook is already working. Add the remaining provider registrations to Render one at a time.

## Render environment variables

Instagram: INSTAGRAM_CLIENT_ID, INSTAGRAM_CLIENT_SECRET
Threads: THREADS_CLIENT_ID, THREADS_CLIENT_SECRET
TikTok: TIKTOK_CLIENT_KEY, TIKTOK_CLIENT_SECRET
X: X_CLIENT_ID, X_CLIENT_SECRET
LinkedIn: LINKEDIN_CLIENT_ID, LINKEDIN_CLIENT_SECRET
Google/YouTube: GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET
Pinterest: PINTEREST_CLIENT_ID, PINTEREST_CLIENT_SECRET
Reddit: REDDIT_CLIENT_ID, REDDIT_CLIENT_SECRET

Do not put user access tokens into Render. OAuth generates them after the customer signs in.

## Exact callbacks

Instagram  https://brandrelay-oauth-broker.onrender.com/callback/instagram
Threads    https://brandrelay-oauth-broker.onrender.com/callback/threads
TikTok     https://brandrelay-oauth-broker.onrender.com/callback/tiktok
X          https://brandrelay-oauth-broker.onrender.com/callback/x
LinkedIn   https://brandrelay-oauth-broker.onrender.com/callback/linkedin
Google     https://brandrelay-oauth-broker.onrender.com/callback/google
Pinterest  https://brandrelay-oauth-broker.onrender.com/callback/pinterest
Reddit     https://brandrelay-oauth-broker.onrender.com/callback/reddit

## Minimum scopes used initially

Instagram: instagram_business_basic
Threads: threads_basic
TikTok: user.info.basic,video.list
X: tweet.read users.read offline.access
LinkedIn: openid profile
Google/YouTube: openid profile + youtube.readonly
Pinterest: user_accounts:read,boards:read,pins:read
Reddit: identity read

Start with these minimum read scopes so provider review/setup errors are easy to isolate. Add publishing/messaging scopes later after the provider product is enabled and approved.

## Verify the broker after updating Render

Open:

    https://brandrelay-oauth-broker.onrender.com/healthz

The response should show version `1.2.0-all-socials`, `configured`, `missing`, `scopes`, and `callbacks`.
