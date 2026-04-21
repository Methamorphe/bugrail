package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/Methamorphe/bugrail/internal/storage"
)

const (
	sessionCookieName = "bugrail_session"
	csrfCookieName    = "bugrail_csrf"
	csrfFieldName     = "csrf_token"
	sessionTTLSeconds = int64(60 * 60 * 24 * 30)
)

type contextKey string

const (
	userContextKey contextKey = "auth_user"
	csrfContextKey contextKey = "csrf_token"
)

// User is the authenticated user attached to the request context.
type User struct {
	ID    string
	Email string
}

// Service handles password verification, session persistence, and web auth middleware.
type Service struct {
	store storage.Store
}

// New creates a new authentication service.
func New(store storage.Store) Service {
	return Service{store: store}
}

// HashPassword hashes a plaintext password for storage.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// Login validates credentials, creates a persisted session, and writes the cookie.
func (s Service) Login(ctx context.Context, w http.ResponseWriter, r *http.Request, email, password string) (User, error) {
	record, err := s.store.GetUserByEmail(ctx, strings.TrimSpace(strings.ToLower(email)))
	if err != nil {
		return User{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(record.PasswordHash), []byte(password)); err != nil {
		return User{}, storage.ErrNotFound
	}

	rawToken, err := randomToken()
	if err != nil {
		return User{}, err
	}
	now := unixNow()
	session, err := s.store.CreateSession(ctx, storage.CreateSessionParams{
		UserID:    record.ID,
		TokenHash: hashToken(rawToken),
		CreatedAt: now,
		ExpiresAt: now + sessionTTLSeconds,
	})
	if err != nil {
		return User{}, fmt.Errorf("create session: %w", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    rawToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(r),
		MaxAge:   int(sessionTTLSeconds),
	})

	return User{
		ID:    session.UserID,
		Email: session.Email,
	}, nil
}

// Logout deletes the persisted session and expires the client cookie.
func (s Service) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil && cookie.Value != "" {
		if deleteErr := s.store.DeleteSessionByTokenHash(ctx, hashToken(cookie.Value)); deleteErr != nil && !errors.Is(deleteErr, storage.ErrNotFound) {
			return deleteErr
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(r),
		MaxAge:   -1,
	})
	return nil
}

// SessionMiddleware loads the current session, if any, and attaches it to the request context.
func (s Service) SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			next.ServeHTTP(w, r)
			return
		}

		now := unixNow()
		session, err := s.store.GetSessionByTokenHash(r.Context(), hashToken(cookie.Value))
		if err != nil {
			s.expireSessionCookie(w, r)
			next.ServeHTTP(w, r)
			return
		}
		if session.ExpiresAt <= now {
			_ = s.store.DeleteSessionByTokenHash(r.Context(), session.TokenHash)
			s.expireSessionCookie(w, r)
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, User{
			ID:    session.UserID,
			Email: session.Email,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CSRFMiddleware issues and validates the double-submit CSRF token used by HTML forms.
func (s Service) CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := r.Cookie(csrfCookieName)
		if err != nil || token.Value == "" {
			value, tokenErr := randomToken()
			if tokenErr != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			token = &http.Cookie{
				Name:     csrfCookieName,
				Value:    value,
				Path:     "/",
				HttpOnly: false,
				SameSite: http.SameSiteLaxMode,
				Secure:   cookieSecure(r),
			}
			http.SetCookie(w, token)
		}

		ctx := context.WithValue(r.Context(), csrfContextKey, token.Value)
		if isUnsafeMethod(r.Method) {
			provided := strings.TrimSpace(r.FormValue(csrfFieldName))
			if provided == "" {
				provided = strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
			}
			if provided == "" || provided != token.Value {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireUser redirects anonymous users to the login page.
func (s Service) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := CurrentUser(r.Context()); !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CurrentUser returns the current authenticated user, if present.
func CurrentUser(ctx context.Context) (User, bool) {
	value, ok := ctx.Value(userContextKey).(User)
	return value, ok
}

// CSRFToken returns the request CSRF token for form rendering.
func CSRFToken(ctx context.Context) string {
	value, _ := ctx.Value(csrfContextKey).(string)
	return value
}

// CSRFFieldName returns the hidden form field name expected by the middleware.
func CSRFFieldName() string {
	return csrfFieldName
}

func (s Service) expireSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   cookieSecure(r),
		MaxAge:   -1,
	})
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random token bytes: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func cookieSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func unixNow() int64 {
	return timeNow().Unix()
}
