package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/osmendes/db-auditor/internal/repository"
)

type AuthStore interface {
	FindAuditorUser(context.Context, string) (*repository.AuditorUser, error)
	CreateAuditorSession(context.Context, string, []byte, time.Time) error
	GetAuditorSession(context.Context, []byte) (*repository.AuditorUser, error)
	DeleteAuditorSession(context.Context, []byte) error
	CreateAuditorUser(context.Context, string, string, string, []string) (*repository.AuditorUser, error)
	LogAuditorOperation(context.Context, *repository.AuditorUser, string, string, string, string) error
	ResolveAuditorResourceEnvironment(context.Context, string, string) (string, error)
	RecordLoginAttempt(context.Context, []byte, time.Duration) (int, error)
	ClearLoginAttempt(context.Context, []byte) error
}

type identityKey struct{}
type sessionKey struct{}

const sessionCookie = "auditor_session"

type requestSession struct {
	token  string
	cookie bool
}

func requestIdentity(r *http.Request) *repository.AuditorUser {
	user, _ := r.Context().Value(identityKey{}).(*repository.AuditorUser)
	return user
}

type attemptWindow struct {
	count int
	start time.Time
}

type loginLimiter struct {
	mu   sync.Mutex
	byIP map[string]attemptWindow
	max  int
}

func (l *loginLimiter) allow(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if len(l.byIP) > 10000 {
		for k, v := range l.byIP {
			if now.Sub(v.start) > time.Minute {
				delete(l.byIP, k)
			}
		}
	}
	v := l.byIP[host]
	if now.Sub(v.start) >= time.Minute {
		v = attemptWindow{start: now}
	}
	v.count++
	l.byIP[host] = v
	limit := l.max
	if limit <= 0 {
		limit = 10
	}
	return v.count <= limit
}

func registerAuthRoutes(mux *http.ServeMux, store AuthStore) {
	mux.HandleFunc("POST /api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		var body struct{ Username, Password string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&body) != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Credenciais inválidas.")
			return
		}
		username := strings.TrimSpace(body.Username)
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		ipKey := sha256.Sum256([]byte("login-ip:" + host))
		accountKey := sha256.Sum256([]byte("login-user:" + strings.ToLower(username)))
		ipAttempts, err := store.RecordLoginAttempt(r.Context(), ipKey[:], time.Minute)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Autenticação indisponível.")
			return
		}
		accountAttempts, err := store.RecordLoginAttempt(r.Context(), accountKey[:], 5*time.Minute)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Autenticação indisponível.")
			return
		}
		if ipAttempts > 120 || accountAttempts > 10 {
			retryAfter := "60"
			if accountAttempts > 10 {
				retryAfter = "300"
			}
			w.Header().Set("Retry-After", retryAfter)
			writeError(w, http.StatusTooManyRequests, CodeUnavailable, "Muitas tentativas. Aguarde e tente novamente.")
			return
		}
		user, err := store.FindAuditorUser(r.Context(), username)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Autenticação indisponível.")
			return
		}
		if user == nil || !user.Active || !repository.CheckAuditorPassword(body.Password, user.PasswordHash) {
			writeError(w, http.StatusUnauthorized, CodeUnavailable, "Credenciais inválidas.")
			return
		}
		token := make([]byte, 32)
		if _, err := rand.Read(token); err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao criar sessão.")
			return
		}
		encoded := base64.RawURLEncoding.EncodeToString(token)
		digest := sha256.Sum256([]byte(encoded))
		if err := store.CreateAuditorSession(r.Context(), user.ID, digest[:], time.Now().Add(8*time.Hour)); err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao criar sessão.")
			return
		}
		if err := store.ClearLoginAttempt(r.Context(), accountKey[:]); err != nil {
			slog.Warn("could not clear login attempt counter", "error", err)
		}
		if r.URL.Query().Get("mode") == "cookie" {
			http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: encoded, Path: "/", MaxAge: 28800,
				HttpOnly: true, Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https", SameSite: http.SameSiteLaxMode})
			writeJSON(w, http.StatusOK, map[string]any{"user": user, "csrf_token": csrfToken(encoded), "expires_in": 28800})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"token": encoded, "user": user, "expires_in": 28800})
	})
	mux.HandleFunc("GET /api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		session, _ := r.Context().Value(sessionKey{}).(requestSession)
		if session.cookie {
			writeJSON(w, http.StatusOK, map[string]any{"user": requestIdentity(r), "csrf_token": csrfToken(session.token)})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": requestIdentity(r)})
	})
	mux.HandleFunc("POST /api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		session, _ := r.Context().Value(sessionKey{}).(requestSession)
		digest := sha256.Sum256([]byte(session.token))
		if err := store.DeleteAuditorSession(r.Context(), digest[:]); err != nil {
			writeError(w, http.StatusInternalServerError, CodeInternal, "Falha ao encerrar sessão.")
			return
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1,
			HttpOnly: true, Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https", SameSite: http.SameSiteLaxMode})
		writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
	})
	mux.HandleFunc("POST /api/v1/auth/users", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username     string   `json:"username"`
			Password     string   `json:"password"`
			Role         string   `json:"role"`
			Environments []string `json:"environments"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Usuário inválido.")
			return
		}
		hash, err := repository.HashAuditorPassword(body.Password)
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "A senha deve ter pelo menos 16 caracteres.")
			return
		}
		user, err := store.CreateAuditorUser(r.Context(), body.Username, hash, body.Role, body.Environments)
		if err != nil {
			writeError(w, http.StatusBadRequest, CodeValidation, "Não foi possível criar o usuário.")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"user": user})
	})
}

func authMiddleware(next http.Handler, store AuthStore) http.Handler {
	limiter := &loginLimiter{byIP: make(map[string]attemptWindow), max: 600}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") || r.URL.Path == "/api/v1/auth/login" {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		provided := r.Header.Get("Authorization")
		session := requestSession{}
		if provided != "" {
			if !strings.HasPrefix(provided, "Bearer ") || len(provided) > 128 {
				writeError(w, http.StatusUnauthorized, CodeUnavailable, "Sessão ausente ou expirada.")
				return
			}
			session.token = strings.TrimPrefix(provided, "Bearer ")
		} else if cookie, err := r.Cookie(sessionCookie); err == nil {
			session.token, session.cookie = cookie.Value, true
		}
		if session.token == "" || len(session.token) > 128 {
			writeError(w, http.StatusUnauthorized, CodeUnavailable, "Sessão ausente ou expirada.")
			return
		}
		digest := sha256.Sum256([]byte(session.token))
		user, err := store.GetAuditorSession(r.Context(), digest[:])
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, CodeUnavailable, "Autenticação indisponível.")
			return
		}
		if user == nil {
			writeError(w, http.StatusUnauthorized, CodeUnavailable, "Sessão ausente ou expirada.")
			return
		}
		if !limiter.allow(user.ID) {
			writeError(w, http.StatusTooManyRequests, CodeUnavailable, "Muitas requisições. Aguarde um minuto.")
			return
		}
		if session.cookie && r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions &&
			subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(csrfToken(session.token))) != 1 {
			writeError(w, http.StatusForbidden, CodeUnavailable, "Token de proteção da sessão ausente ou inválido.")
			return
		}
		env, allowed := authorizeRequest(r, user, store)
		if !allowed {
			if err := store.LogAuditorOperation(r.Context(), user, r.Method+" "+r.URL.Path, env, "", "denied"); err != nil {
				slog.Warn("could not write authorization log", "error", err)
			}
			writeError(w, http.StatusForbidden, CodeUnavailable, "Acesso não autorizado.")
			return
		}
		ctx := context.WithValue(context.WithValue(r.Context(), identityKey{}, user), sessionKey{}, session)
		if r.Method == http.MethodGet && !strings.HasSuffix(r.URL.Path, "/download") {
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		recorder := &operationRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r.WithContext(ctx))
		result := "success"
		if recorder.status >= 400 {
			result = "failed"
		}
		if err := store.LogAuditorOperation(r.Context(), user, r.Method+" "+r.URL.Path, env, "", result); err != nil {
			slog.Warn("could not write operation log", "error", err)
		}
	})
}

func csrfToken(token string) string {
	digest := sha256.Sum256([]byte("auditor-csrf:" + token))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

type operationRecorder struct {
	http.ResponseWriter
	status int
}

func (r *operationRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func authorizeRequest(r *http.Request, user *repository.AuditorUser, store AuthStore) (string, bool) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	parts := strings.Split(path, "/")
	if path == "auth/logout" {
		return "", true
	}
	if strings.HasSuffix(path, "/download") && !roleAtLeast(user.Role, "auditor") {
		return environmentFromRequest(r, parts), false
	}
	if r.Method != http.MethodGet {
		minimum := "operator"
		if (parts[0] == "findings" && (r.Method == http.MethodPatch || len(parts) > 1 && parts[1] == "analyze")) ||
			parts[0] == "mappings" || strings.Contains(path, "/reports") {
			minimum = "auditor"
		}
		if !roleAtLeast(user.Role, minimum) {
			return "", false
		}
	}
	if path == "auth/users" && user.Role != "operator" {
		return "", false
	}
	if user.Role == "operator" {
		return environmentFromRequest(r, parts), true
	}
	if path == "environments" || strings.HasPrefix(path, "auth/") {
		return "", true
	}
	if len(parts) >= 2 && (parts[0] == "audit-runs" || parts[0] == "findings") && uuidPattern.MatchString(parts[1]) {
		resolved, err := store.ResolveAuditorResourceEnvironment(r.Context(), parts[0], parts[1])
		if err != nil || resolved == "" {
			return "", false
		}
		return resolved, hasEnvironment(user, resolved)
	}
	if parts[0] == "environments" {
		if len(parts) < 2 || !uuidPattern.MatchString(parts[1]) {
			return "", false
		}
	} else {
		allowedGlobal := path == "audit-runs" || path == "findings" ||
			path == "finding-categories/security" || path == "finding-categories/performance" ||
			path == "analytics/kpis" || path == "analytics/storage" || path == "analytics/findings-trends" || path == "analytics/job-health" ||
			path == "reports/inventory" || path == "reports/findings"
		if !allowedGlobal {
			return "", false
		}
	}
	env := environmentFromRequest(r, parts)
	if env == "" || !hasEnvironment(user, env) {
		return env, false
	}
	for _, key := range []string{"source_environment_id", "target_environment_id"} {
		if other := r.URL.Query().Get(key); other != "" && !hasEnvironment(user, other) {
			return env, false
		}
	}
	return env, true
}

func environmentFromRequest(r *http.Request, parts []string) string {
	if len(parts) >= 2 && parts[0] == "environments" && uuidPattern.MatchString(parts[1]) {
		return parts[1]
	}
	return r.URL.Query().Get("environment_id")
}

func hasEnvironment(user *repository.AuditorUser, id string) bool {
	for _, allowed := range user.Environments {
		if allowed == id {
			return true
		}
	}
	return false
}

func roleAtLeast(actual, required string) bool {
	rank := map[string]int{"viewer": 1, "auditor": 2, "operator": 3}
	return rank[actual] >= rank[required]
}
