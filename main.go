package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Provider struct {
	Slug, Name, ClientID, ClientSecret, AuthURL, TokenURL, Scope string
	PKCE                                                         bool
}

type Session struct {
	ID, Secret, Provider, State, Verifier, Redirect string
	Created                                         time.Time
	Status, Error                                   string
	AccessToken, RefreshToken, Scope                string
	ExpiresIn                                       int64
	AccountID, AccountLabel                         string
}

var (
	publicBase string
	sessionsMu sync.Mutex
	sessions   = map[string]*Session{}
	providers  map[string]Provider
)

func env(k string) string { return strings.TrimSpace(os.Getenv(k)) }
func random(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	s := base64.RawURLEncoding.EncodeToString(b)
	if len(s) > n {
		s = s[:n]
	}
	return s
}

func main() {
	publicBase = strings.TrimRight(env("BRANDRELAY_PUBLIC_URL"), "/")
	providers = loadProviders()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health)
	mux.HandleFunc("/v1/connect/start", start)
	mux.HandleFunc("/v1/connect/status", status)
	mux.HandleFunc("/callback/", callback)
	port := env("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{Addr: ":" + port, Handler: securityHeaders(mux), ReadHeaderTimeout: 8 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second}
	go cleanupLoop()
	log.Printf("BrandRelay OAuth Broker listening on :%s public=%s", port, publicBase)
	log.Fatal(srv.ListenAndServe())
}

func loadProviders() map[string]Provider {
	metaID, metaSecret := env("META_APP_ID"), env("META_APP_SECRET")
	return map[string]Provider{
		"facebook":  {Slug: "facebook", Name: "Facebook", ClientID: metaID, ClientSecret: metaSecret, AuthURL: "https://www.facebook.com/dialog/oauth", TokenURL: "https://graph.facebook.com/v26.0/oauth/access_token", Scope: "public_profile,user_posts,pages_show_list,pages_read_engagement,pages_read_user_content,pages_manage_posts"},
		"instagram": {Slug: "instagram", Name: "Instagram", ClientID: envDefault("INSTAGRAM_CLIENT_ID", metaID), ClientSecret: envDefault("INSTAGRAM_CLIENT_SECRET", metaSecret), AuthURL: "https://www.instagram.com/oauth/authorize", TokenURL: "https://api.instagram.com/oauth/access_token", Scope: "instagram_business_basic,instagram_business_content_publish,instagram_business_manage_comments,instagram_business_manage_messages"},
		"threads":   {Slug: "threads", Name: "Threads", ClientID: envDefault("THREADS_CLIENT_ID", metaID), ClientSecret: envDefault("THREADS_CLIENT_SECRET", metaSecret), AuthURL: "https://threads.net/oauth/authorize", TokenURL: "https://graph.threads.net/oauth/access_token", Scope: "threads_basic,threads_content_publish"},
		"tiktok":    {Slug: "tiktok", Name: "TikTok", ClientID: env("TIKTOK_CLIENT_KEY"), ClientSecret: env("TIKTOK_CLIENT_SECRET"), AuthURL: "https://www.tiktok.com/v2/auth/authorize/", TokenURL: "https://open.tiktokapis.com/v2/oauth/token/", Scope: "user.info.basic,video.list", PKCE: true},
		"x":         {Slug: "x", Name: "X", ClientID: env("X_CLIENT_ID"), ClientSecret: env("X_CLIENT_SECRET"), AuthURL: "https://x.com/i/oauth2/authorize", TokenURL: "https://api.x.com/2/oauth2/token", Scope: "tweet.read users.read offline.access", PKCE: true},
		"linkedin":  {Slug: "linkedin", Name: "LinkedIn", ClientID: env("LINKEDIN_CLIENT_ID"), ClientSecret: env("LINKEDIN_CLIENT_SECRET"), AuthURL: "https://www.linkedin.com/oauth/v2/authorization", TokenURL: "https://www.linkedin.com/oauth/v2/accessToken", Scope: "openid profile w_member_social", PKCE: false},
		"google":    {Slug: "google", Name: "Google / YouTube", ClientID: env("GOOGLE_CLIENT_ID"), ClientSecret: env("GOOGLE_CLIENT_SECRET"), AuthURL: "https://accounts.google.com/o/oauth2/v2/auth", TokenURL: "https://oauth2.googleapis.com/token", Scope: "https://www.googleapis.com/auth/youtube", PKCE: true},
		"pinterest": {Slug: "pinterest", Name: "Pinterest", ClientID: env("PINTEREST_CLIENT_ID"), ClientSecret: env("PINTEREST_CLIENT_SECRET"), AuthURL: "https://www.pinterest.com/oauth/", TokenURL: "https://api.pinterest.com/v5/oauth/token", Scope: "user_accounts:read,boards:read,pins:read,pins:write"},
		"reddit":    {Slug: "reddit", Name: "Reddit", ClientID: env("REDDIT_CLIENT_ID"), ClientSecret: env("REDDIT_CLIENT_SECRET"), AuthURL: "https://www.reddit.com/api/v1/authorize", TokenURL: "https://www.reddit.com/api/v1/access_token", Scope: "identity read submit"},
	}
}
func envDefault(k, d string) string {
	if v := env(k); v != "" {
		return v
	}
	return d
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func health(w http.ResponseWriter, r *http.Request) {
	configured := []string{}
	for slug, p := range providers {
		if p.ClientID != "" {
			configured = append(configured, slug)
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true, "public_url": publicBase, "configured": configured})
}

func start(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, 405, map[string]any{"error": "POST required"})
		return
	}
	if publicBase == "" || !strings.HasPrefix(publicBase, "https://") {
		writeJSON(w, 503, map[string]any{"error": "BRANDRELAY_PUBLIC_URL must be a public HTTPS URL"})
		return
	}
	var in struct {
		Provider string `json:"provider"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&in) != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid JSON"})
		return
	}
	p, ok := providers[strings.ToLower(strings.TrimSpace(in.Provider))]
	if !ok {
		writeJSON(w, 404, map[string]any{"error": "unknown provider"})
		return
	}
	if p.ClientID == "" {
		writeJSON(w, 503, map[string]any{"error": p.Name + " app registration is not configured on the broker"})
		return
	}
	s := &Session{ID: random(32), Secret: random(48), Provider: p.Slug, State: random(42), Created: time.Now(), Status: "pending", Redirect: publicBase + "/callback/" + p.Slug}
	if p.PKCE {
		s.Verifier = random(86)
	}
	auth := authorizationURL(p, s)
	sessionsMu.Lock()
	sessions[s.ID] = s
	sessionsMu.Unlock()
	writeJSON(w, 200, map[string]any{"session_id": s.ID, "session_secret": s.Secret, "authorization_url": auth})
}

func authorizationURL(p Provider, s *Session) string {
	q := url.Values{}
	key := "client_id"
	if p.Slug == "tiktok" {
		key = "client_key"
	}
	q.Set(key, p.ClientID)
	q.Set("redirect_uri", s.Redirect)
	q.Set("response_type", "code")
	q.Set("state", s.State)
	q.Set("scope", p.Scope)
	if p.PKCE {
		sum := sha256.Sum256([]byte(s.Verifier))
		challenge := base64.RawURLEncoding.EncodeToString(sum[:])
		if p.Slug == "tiktok" {
			challenge = fmt.Sprintf("%x", sum[:])
		}
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", "S256")
	}
	switch p.Slug {
	case "google":
		q.Set("access_type", "offline")
		q.Set("include_granted_scopes", "true")
		q.Set("prompt", "consent")
	case "reddit":
		q.Set("duration", "permanent")
	}
	sep := "?"
	if strings.Contains(p.AuthURL, "?") {
		sep = "&"
	}
	return p.AuthURL + sep + q.Encode()
}

func callback(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/callback/")
	st := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	oauthErr := r.URL.Query().Get("error")
	sessionsMu.Lock()
	var s *Session
	for _, x := range sessions {
		if x.Provider == slug && x.State == st && x.Status == "pending" {
			s = x
			break
		}
	}
	sessionsMu.Unlock()
	if s == nil {
		writeHTML(w, false, "Secure Connect", "Invalid or expired authorization state.")
		return
	}
	if oauthErr != "" {
		setError(s, "Provider denied authorization: "+oauthErr)
		writeHTML(w, false, providers[slug].Name, s.Error)
		return
	}
	if code == "" {
		setError(s, "Provider returned no authorization code")
		writeHTML(w, false, providers[slug].Name, s.Error)
		return
	}
	p := providers[slug]
	tok, err := exchange(p, s, code)
	if err != nil {
		setError(s, err.Error())
		writeHTML(w, false, p.Name, s.Error)
		return
	}
	s.AccessToken = tok.AccessToken
	s.RefreshToken = tok.RefreshToken
	s.ExpiresIn = tok.ExpiresIn
	s.Scope = tok.Scope
	s.Status = "complete"
	if tok.AccountID != "" {
		s.AccountID = tok.AccountID
	}
	discoverIdentity(p, s)
	writeHTML(w, true, p.Name, "Authorization completed. Return to Hall Monitor; it will finish linking and sync the available feed automatically.")
}

type TokenResult struct {
	AccessToken, RefreshToken, Scope, AccountID string
	ExpiresIn                                   int64
}

func exchange(p Provider, s *Session, code string) (TokenResult, error) {
	vals := url.Values{"code": {code}, "redirect_uri": {s.Redirect}, "grant_type": {"authorization_code"}}
	switch p.Slug {
	case "tiktok":
		vals.Set("client_key", p.ClientID)
		vals.Set("client_secret", p.ClientSecret)
		vals.Set("code_verifier", s.Verifier)
	case "x":
		vals.Set("client_id", p.ClientID)
		if s.Verifier != "" {
			vals.Set("code_verifier", s.Verifier)
		}
	case "google":
		vals.Set("client_id", p.ClientID)
		if p.ClientSecret != "" {
			vals.Set("client_secret", p.ClientSecret)
		}
		vals.Set("code_verifier", s.Verifier)
	case "linkedin":
		vals.Set("client_id", p.ClientID)
		vals.Set("client_secret", p.ClientSecret)
	case "instagram", "threads":
		vals.Set("client_id", p.ClientID)
		vals.Set("client_secret", p.ClientSecret)
	case "facebook":
		q := url.Values{"client_id": {p.ClientID}, "client_secret": {p.ClientSecret}, "redirect_uri": {s.Redirect}, "code": {code}}
		req, _ := http.NewRequest("GET", p.TokenURL+"?"+q.Encode(), nil)
		return doToken(req, p)
	case "pinterest", "reddit": // Basic auth below
	default:
		vals.Set("client_id", p.ClientID)
		vals.Set("client_secret", p.ClientSecret)
	}
	req, _ := http.NewRequest("POST", p.TokenURL, strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if p.Slug == "pinterest" || p.Slug == "reddit" || p.Slug == "x" {
		if p.ClientSecret != "" {
			req.SetBasicAuth(p.ClientID, p.ClientSecret)
		}
	}
	if p.Slug == "reddit" {
		req.Header.Set("User-Agent", "BrandRelay-OAuth-Broker/1.0")
	}
	return doToken(req, p)
}

func doToken(req *http.Request, p Provider) (TokenResult, error) {
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return TokenResult{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TokenResult{}, fmt.Errorf("%s token exchange HTTP %d: %s", p.Name, resp.StatusCode, clip(string(b), 300))
	}
	var raw map[string]any
	if json.Unmarshal(b, &raw) != nil {
		return TokenResult{}, fmt.Errorf("invalid token response")
	}
	tr := TokenResult{AccessToken: str(raw["access_token"]), RefreshToken: str(raw["refresh_token"]), Scope: str(raw["scope"]), ExpiresIn: num(raw["expires_in"])}
	if tr.AccessToken == "" {
		if data, ok := raw["data"].(map[string]any); ok {
			tr.AccessToken = str(data["access_token"])
			tr.RefreshToken = str(data["refresh_token"])
			tr.Scope = str(data["scope"])
			tr.ExpiresIn = num(data["expires_in"])
			tr.AccountID = str(data["open_id"])
		}
	}
	if tr.AccessToken == "" {
		return TokenResult{}, fmt.Errorf("%s returned no access token: %s", p.Name, clip(string(b), 260))
	}
	return tr, nil
}

func discoverIdentity(p Provider, s *Session) {
	endpoint := ""
	switch p.Slug {
	case "facebook":
		endpoint = "https://graph.facebook.com/v26.0/me?fields=id,name"
	case "instagram":
		endpoint = "https://graph.instagram.com/me?fields=id,username"
	case "threads":
		endpoint = "https://graph.threads.net/v1.0/me?fields=id,username"
	case "tiktok":
		endpoint = "https://open.tiktokapis.com/v2/user/info/?fields=open_id,display_name"
	case "x":
		endpoint = "https://api.x.com/2/users/me?user.fields=id,name,username"
	case "linkedin":
		endpoint = "https://api.linkedin.com/v2/userinfo"
	case "google":
		endpoint = "https://www.googleapis.com/youtube/v3/channels?part=id,snippet&mine=true"
	case "pinterest":
		endpoint = "https://api.pinterest.com/v5/user_account"
	case "reddit":
		endpoint = "https://oauth.reddit.com/api/v1/me"
	}
	if endpoint == "" {
		return
	}
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Authorization", "Bearer "+s.AccessToken)
	if p.Slug == "reddit" {
		req.Header.Set("User-Agent", "BrandRelay-OAuth-Broker/1.0")
	}
	resp, err := (&http.Client{Timeout: 12 * time.Second}).Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}
	var v any
	if json.Unmarshal(b, &v) != nil {
		return
	}
	s.AccountID = find(v, []string{"open_id", "sub", "id", "user_id"})
	s.AccountLabel = find(v, []string{"display_name", "username", "name", "title"})
}
func find(v any, keys []string) string {
	switch x := v.(type) {
	case map[string]any:
		for _, k := range keys {
			if z := str(x[k]); z != "" {
				return z
			}
		}
		for _, vv := range x {
			if z := find(vv, keys); z != "" {
				return z
			}
		}
	case []any:
		for _, vv := range x {
			if z := find(vv, keys); z != "" {
				return z
			}
		}
	}
	return ""
}

func status(w http.ResponseWriter, r *http.Request) {
	id, secret := r.URL.Query().Get("session_id"), r.URL.Query().Get("session_secret")
	sessionsMu.Lock()
	s := sessions[id]
	sessionsMu.Unlock()
	if s == nil || secret == "" || s.Secret != secret {
		writeJSON(w, 404, map[string]any{"error": "unknown session"})
		return
	}
	out := map[string]any{"status": s.Status}
	if s.Status == "error" {
		out["error"] = s.Error
	}
	if s.Status == "complete" {
		out["access_token"] = s.AccessToken
		out["refresh_token"] = s.RefreshToken
		out["scope"] = s.Scope
		out["expires_in"] = s.ExpiresIn
		out["account_id"] = s.AccountID
		out["account_label"] = s.AccountLabel
	}
	writeJSON(w, 200, out)
}
func setError(s *Session, e string) {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	s.Status = "error"
	s.Error = e
}
func cleanupLoop() {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for range t.C {
		cut := time.Now().Add(-15 * time.Minute)
		sessionsMu.Lock()
		for id, s := range sessions {
			if s.Created.Before(cut) {
				delete(sessions, id)
			}
		}
		sessionsMu.Unlock()
	}
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeHTML(w http.ResponseWriter, ok bool, provider, text string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	mark := "✿"
	title := "Connected"
	if !ok {
		mark = "!"
		title = "Connection failed"
	}
	fmt.Fprintf(w, `<!doctype html><meta charset=utf-8><title>BrandRelay Secure Connect</title><style>body{font-family:Segoe UI,sans-serif;background:#fff7fb;color:#352d38;display:grid;place-items:center;min-height:100vh;margin:0}.c{max-width:560px;background:white;border:1px solid #eadfea;border-radius:24px;padding:34px;text-align:center}.m{font-size:52px}.p{color:#806f80}</style><div class=c><div class=m>%s</div><h1>%s</h1><h2>%s</h2><p>%s</p><p class=p>You can close this browser tab. Hall Monitor is finishing the connection.</p>`, mark, title, provider, text)
}
func str(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	}
	return ""
}
func num(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	}
	return 0
}
func clip(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}
