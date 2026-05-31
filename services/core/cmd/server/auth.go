package main

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	authCookieName    = "dcp_session"
	authStateCookie   = "dcp_auth_state"
	defaultAuthPath   = "/api/v1/auth/callback"
	defaultLogoutPath = "/"
)

var (
	errSessionNotFound = errors.New("session not found")
	errAuthUnavailable = errors.New("keycloak configuration not available")
	errStateMismatch   = errors.New("authentication state mismatch")
	errAuthExpired     = errors.New("authentication session expired")
)

type authUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	Name     string `json:"name,omitempty"`
}

type keycloakAuth struct {
	baseURL       string
	realm         string
	clientID      string
	clientSecret  string
	redirectPath  string
	logoutPath    string
	sessionSecret string

	client *http.Client

	mu         sync.Mutex
	discovery  *oidcDiscovery
	jwks       *jwksDocument
	jwksLoaded time.Time
}

type oidcDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type authState struct {
	State        string `json:"state"`
	Nonce        string `json:"nonce"`
	CodeVerifier string `json:"codeVerifier"`
	ReturnTo     string `json:"returnTo"`
	Mode         string `json:"mode"`
	ExpiresAt    int64  `json:"expiresAt"`
}

type sessionEnvelope struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email,omitempty"`
	Name      string `json:"name,omitempty"`
	ExpiresAt int64  `json:"expiresAt"`
}

type idTokenClaims struct {
	Sub               string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	Nonce             string `json:"nonce"`
	Iss               string `json:"iss"`
	Exp               int64  `json:"exp"`
	AuthTime          int64  `json:"auth_time"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type idTokenHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	Typ string `json:"typ"`
}

func newKeycloakAuthFromEnv() (*keycloakAuth, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("DCP_KEYCLOAK_BASE_URL")), "/")
	realm := strings.TrimSpace(os.Getenv("DCP_KEYCLOAK_REALM"))
	clientID := strings.TrimSpace(os.Getenv("DCP_KEYCLOAK_CLIENT_ID"))
	if baseURL == "" || realm == "" || clientID == "" {
		return nil, errAuthUnavailable
	}

	clientSecret := strings.TrimSpace(os.Getenv("DCP_KEYCLOAK_CLIENT_SECRET"))
	redirectPath := strings.TrimSpace(os.Getenv("DCP_KEYCLOAK_REDIRECT_PATH"))
	if redirectPath == "" {
		redirectPath = defaultAuthPath
	}
	logoutPath := strings.TrimSpace(os.Getenv("DCP_KEYCLOAK_LOGOUT_PATH"))
	if logoutPath == "" {
		logoutPath = defaultLogoutPath
	}
	sessionSecret := strings.TrimSpace(os.Getenv("DCP_SESSION_SECRET"))
	if sessionSecret == "" {
		sessionSecret = "dcp-session-secret-change-me"
	}

	return &keycloakAuth{
		baseURL:       baseURL,
		realm:         realm,
		clientID:      clientID,
		clientSecret:  clientSecret,
		redirectPath:  redirectPath,
		logoutPath:    logoutPath,
		sessionSecret: sessionSecret,
		client:        &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (a *keycloakAuth) Login(w http.ResponseWriter, r *http.Request) error {
	return a.beginAuth(w, r, "login")
}

func (a *keycloakAuth) Register(w http.ResponseWriter, r *http.Request) error {
	return a.beginAuth(w, r, "register")
}

func (a *keycloakAuth) Callback(w http.ResponseWriter, r *http.Request) error {
	if a == nil {
		return errAuthUnavailable
	}
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("認可結果の読み込みに失敗しました")
	}

	stateValue := strings.TrimSpace(r.Form.Get("state"))
	code := strings.TrimSpace(r.Form.Get("code"))
	if stateValue == "" || code == "" {
		return fmt.Errorf("認可結果が不正です")
	}

	stateCookie, err := r.Cookie(authStateCookie)
	if err != nil || stateCookie.Value == "" {
		return errStateMismatch
	}
	state, err := a.decodeAuthState(stateCookie.Value)
	if err != nil {
		return errStateMismatch
	}
	if state.State != stateValue {
		return errStateMismatch
	}

	discovery, err := a.discoveryDocument(r.Context())
	if err != nil {
		return err
	}

	token, err := a.exchangeCode(r, discovery.TokenEndpoint, code, state.CodeVerifier)
	if err != nil {
		return err
	}
	claims, err := a.verifyIDToken(discovery, token.IDToken, state.Nonce)
	if err != nil {
		return err
	}

	user := authUser{
		ID:       claims.Sub,
		Username: firstNonEmpty(claims.PreferredUsername, claims.Email, claims.Name, claims.Sub),
		Email:    claims.Email,
		Name:     claims.Name,
	}
	if err := a.setSessionCookie(w, user, claims.Exp, isSecureRequest(r)); err != nil {
		return err
	}
	http.SetCookie(w, clearAuthStateCookie(isSecureRequest(r)))
	http.Redirect(w, r, a.returnToURL(r, state.ReturnTo), http.StatusSeeOther)
	return nil
}

func (a *keycloakAuth) Logout(w http.ResponseWriter, r *http.Request) error {
	if a == nil {
		return errAuthUnavailable
	}
	http.SetCookie(w, clearAuthCookie(isSecureRequest(r)))
	http.SetCookie(w, clearAuthStateCookie(isSecureRequest(r)))

	discovery, err := a.discoveryDocument(r.Context())
	if err != nil {
		http.Redirect(w, r, a.returnToURL(r, defaultLogoutPath), http.StatusSeeOther)
		return nil
	}

	logoutURL, err := url.Parse(discovery.EndSessionEndpoint)
	if err != nil {
		http.Redirect(w, r, a.returnToURL(r, defaultLogoutPath), http.StatusSeeOther)
		return nil
	}
	q := logoutURL.Query()
	q.Set("client_id", a.clientID)
	q.Set("post_logout_redirect_uri", a.returnToURL(r, a.logoutPath))
	logoutURL.RawQuery = q.Encode()
	http.Redirect(w, r, logoutURL.String(), http.StatusSeeOther)
	return nil
}

func (a *keycloakAuth) CurrentUser(r *http.Request) (authUser, error) {
	cookie, err := r.Cookie(authCookieName)
	if err != nil || cookie.Value == "" {
		return authUser{}, errSessionNotFound
	}
	session, err := a.decodeSession(cookie.Value)
	if err != nil {
		return authUser{}, err
	}
	return authUser{
		ID:       session.ID,
		Username: session.Username,
		Email:    session.Email,
		Name:     session.Name,
	}, nil
}

func (a *keycloakAuth) beginAuth(w http.ResponseWriter, r *http.Request, mode string) error {
	if a == nil {
		return errAuthUnavailable
	}
	discovery, err := a.discoveryDocument(r.Context())
	if err != nil {
		return err
	}
	stateValue, err := randomToken(16)
	if err != nil {
		return err
	}
	nonce, err := randomToken(16)
	if err != nil {
		return err
	}
	codeVerifier, err := randomCodeVerifier()
	if err != nil {
		return err
	}
	state := authState{
		State:        stateValue,
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
		ReturnTo:     a.returnToURL(r, defaultLogoutPath),
		Mode:         mode,
		ExpiresAt:    time.Now().UTC().Add(10 * time.Minute).Unix(),
	}
	if err := a.setAuthStateCookie(w, state, isSecureRequest(r)); err != nil {
		return err
	}

	loginURL, err := url.Parse(discovery.AuthorizationEndpoint)
	if err != nil {
		return err
	}
	q := loginURL.Query()
	q.Set("client_id", a.clientID)
	q.Set("redirect_uri", a.returnToURL(r, a.redirectPath))
	q.Set("response_type", "code")
	q.Set("scope", "openid profile email")
	q.Set("state", stateValue)
	q.Set("nonce", nonce)
	q.Set("code_challenge", pkceChallenge(codeVerifier))
	q.Set("code_challenge_method", "S256")
	if mode == "register" {
		q.Set("prompt", "create")
	}
	loginURL.RawQuery = q.Encode()
	http.Redirect(w, r, loginURL.String(), http.StatusSeeOther)
	return nil
}

func (a *keycloakAuth) exchangeCode(r *http.Request, tokenEndpoint, code, codeVerifier string) (tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", a.clientID)
	form.Set("code", code)
	form.Set("redirect_uri", a.returnToURL(r, a.redirectPath))
	form.Set("code_verifier", codeVerifier)
	if a.clientSecret != "" {
		form.Set("client_secret", a.clientSecret)
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := a.client.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer res.Body.Close()

	var payload tokenResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return tokenResponse{}, fmt.Errorf("トークン応答を読み取れませんでした")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return tokenResponse{}, fmt.Errorf("Keycloak での認証に失敗しました")
	}
	if payload.IDToken == "" {
		return tokenResponse{}, fmt.Errorf("IDトークンが返されませんでした")
	}
	return payload, nil
}

func (a *keycloakAuth) verifyIDToken(discovery *oidcDiscovery, token string, expectedNonce string) (idTokenClaims, error) {
	header, claimsBytes, signature, signedPart, err := splitJWT(token)
	if err != nil {
		return idTokenClaims{}, err
	}
	if !strings.EqualFold(header.Alg, "RS256") {
		return idTokenClaims{}, fmt.Errorf("未対応の署名アルゴリズムです")
	}
	key, err := a.jwkForKid(discovery.JWKSURI, header.Kid)
	if err != nil {
		return idTokenClaims{}, err
	}
	publicKey, err := rsaPublicKeyFromJWK(key)
	if err != nil {
		return idTokenClaims{}, err
	}
	sum := sha256.Sum256([]byte(signedPart))
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, sum[:], signature); err != nil {
		return idTokenClaims{}, fmt.Errorf("IDトークンの署名を検証できませんでした")
	}

	var claims idTokenClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return idTokenClaims{}, fmt.Errorf("IDトークンを読み取れませんでした")
	}
	if claims.Iss != discovery.Issuer {
		return idTokenClaims{}, fmt.Errorf("issuer が一致しません")
	}
	if claims.Exp <= time.Now().UTC().Unix() {
		return idTokenClaims{}, errAuthExpired
	}
	if expectedNonce != "" && claims.Nonce != expectedNonce {
		return idTokenClaims{}, fmt.Errorf("nonce が一致しません")
	}
	if claims.Sub == "" {
		return idTokenClaims{}, fmt.Errorf("subject がありません")
	}
	return claims, nil
}

func (a *keycloakAuth) jwkForKid(jwksURI, kid string) (jwk, error) {
	a.mu.Lock()
	if a.jwks != nil && time.Since(a.jwksLoaded) < 5*time.Minute {
		for _, key := range a.jwks.Keys {
			if key.Kid == kid {
				a.mu.Unlock()
				return key, nil
			}
		}
	}
	a.mu.Unlock()

	req, err := http.NewRequest(http.MethodGet, jwksURI, nil)
	if err != nil {
		return jwk{}, err
	}
	res, err := a.client.Do(req)
	if err != nil {
		return jwk{}, err
	}
	defer res.Body.Close()
	var doc jwksDocument
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		return jwk{}, fmt.Errorf("JWK を読み取れませんでした")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return jwk{}, fmt.Errorf("JWK の取得に失敗しました")
	}
	a.mu.Lock()
	a.jwks = &doc
	a.jwksLoaded = time.Now().UTC()
	a.mu.Unlock()
	for _, key := range doc.Keys {
		if key.Kid == kid {
			return key, nil
		}
	}
	return jwk{}, fmt.Errorf("該当する公開鍵がありません")
}

func (a *keycloakAuth) discoveryDocument(ctx context.Context) (*oidcDiscovery, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.discovery != nil {
		return a.discovery, nil
	}
	discoveryURL := fmt.Sprintf("%s/realms/%s/.well-known/openid-configuration", a.baseURL, a.realm)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, err
	}
	res, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var discovery oidcDiscovery
	if err := json.NewDecoder(res.Body).Decode(&discovery); err != nil {
		return nil, fmt.Errorf("Keycloak の設定を読み取れませんでした")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("Keycloak の設定を取得できませんでした")
	}
	a.discovery = &discovery
	return &discovery, nil
}

func (a *keycloakAuth) setSessionCookie(w http.ResponseWriter, user authUser, exp int64, secure bool) error {
	session := sessionEnvelope{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Name:      user.Name,
		ExpiresAt: exp,
	}
	value, err := a.signPayload(session)
	if err != nil {
		return err
	}
	cookie := &http.Cookie{
		Name:     authCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   max(1, int(time.Until(time.Unix(exp, 0)).Seconds())),
	}
	http.SetCookie(w, cookie)
	return nil
}

func (a *keycloakAuth) setAuthStateCookie(w http.ResponseWriter, state authState, secure bool) error {
	value, err := a.signPayload(state)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authStateCookie,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   60 * 10,
	})
	return nil
}

func clearAuthCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	}
}

func clearAuthStateCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     authStateCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	}
}

func (a *keycloakAuth) decodeSession(value string) (sessionEnvelope, error) {
	var payload sessionEnvelope
	if err := a.verifyPayload(value, &payload); err != nil {
		return sessionEnvelope{}, err
	}
	if payload.ExpiresAt > 0 && time.Now().UTC().Unix() > payload.ExpiresAt {
		return sessionEnvelope{}, errAuthExpired
	}
	return payload, nil
}

func (a *keycloakAuth) decodeAuthState(value string) (authState, error) {
	var payload authState
	if err := a.verifyPayload(value, &payload); err != nil {
		return authState{}, err
	}
	if payload.ExpiresAt > 0 && time.Now().UTC().Unix() > payload.ExpiresAt {
		return authState{}, errAuthExpired
	}
	return payload, nil
}

func (a *keycloakAuth) signPayload(v any) (string, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	data := base64.RawURLEncoding.EncodeToString(body)
	mac := hmac.New(sha256.New, []byte(a.sessionSecret))
	_, _ = mac.Write([]byte(data))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return data + "." + sig, nil
}

func (a *keycloakAuth) verifyPayload(value string, out any) error {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return errSessionNotFound
	}
	mac := hmac.New(sha256.New, []byte(a.sessionSecret))
	_, _ = mac.Write([]byte(parts[0]))
	expected := mac.Sum(nil)
	actual, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return errSessionNotFound
	}
	if !hmac.Equal(expected, actual) {
		return errSessionNotFound
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return errSessionNotFound
	}
	if err := json.Unmarshal(body, out); err != nil {
		return errSessionNotFound
	}
	return nil
}

func (a *keycloakAuth) returnToURL(r *http.Request, path string) string {
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return publicBaseURL(r) + path
}

func splitJWT(token string) (idTokenHeader, []byte, []byte, string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return idTokenHeader{}, nil, nil, "", fmt.Errorf("IDトークンの形式が不正です")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return idTokenHeader{}, nil, nil, "", fmt.Errorf("IDトークンのヘッダを読めませんでした")
	}
	bodyBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return idTokenHeader{}, nil, nil, "", fmt.Errorf("IDトークンの本文を読めませんでした")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return idTokenHeader{}, nil, nil, "", fmt.Errorf("IDトークンの署名を読めませんでした")
	}
	var header idTokenHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return idTokenHeader{}, nil, nil, "", fmt.Errorf("IDトークンのヘッダを読めませんでした")
	}
	return header, bodyBytes, sig, parts[0] + "." + parts[1], nil
}

func rsaPublicKeyFromJWK(key jwk) (*rsa.PublicKey, error) {
	if key.Kty != "RSA" {
		return nil, fmt.Errorf("RSA 公開鍵ではありません")
	}
	nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("公開鍵の modulus を読めませんでした")
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("公開鍵の exponent を読めませんでした")
	}
	exp := big.NewInt(0)
	exp.SetBytes(eBytes)
	if !exp.IsInt64() {
		return nil, fmt.Errorf("公開鍵の exponent が大きすぎます")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(exp.Int64()),
	}, nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomCodeVerifier() (string, error) {
	return randomToken(32)
}

func randomToken(length int) (string, error) {
	if length <= 0 {
		length = 32
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
