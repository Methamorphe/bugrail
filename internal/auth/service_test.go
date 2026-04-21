package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Methamorphe/bugrail/internal/config"
	"github.com/Methamorphe/bugrail/internal/storage"
)

func TestLoginAndSessionMiddleware(t *testing.T) {
	ctx := context.Background()
	cfg := config.Config{
		DataDir:    t.TempDir(),
		ListenAddr: ":8080",
	}

	store, err := storage.Open(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })

	require.NoError(t, store.Migrate(ctx))

	hash, err := HashPassword("secret123")
	require.NoError(t, err)

	_, err = store.Bootstrap(ctx, storage.BootstrapParams{
		AdminEmail:   "admin@example.com",
		PasswordHash: hash,
		OrgName:      "Acme",
		OrgSlug:      "acme",
		ProjectName:  "Bugrail",
		ProjectSlug:  "bugrail",
	})
	require.NoError(t, err)

	service := New(store)
	req := httptest.NewRequest(http.MethodPost, "https://bugrail.test/login", nil)
	rec := httptest.NewRecorder()

	user, err := service.Login(ctx, rec, req, "admin@example.com", "secret123")
	require.NoError(t, err)
	require.Equal(t, "admin@example.com", user.Email)

	res := rec.Result()
	cookies := res.Cookies()
	require.NotEmpty(t, cookies)

	protectedCalled := false
	protected := service.SessionMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current, ok := CurrentUser(r.Context())
		require.True(t, ok)
		require.Equal(t, "admin@example.com", current.Email)
		protectedCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req2 := httptest.NewRequest(http.MethodGet, "https://bugrail.test/issues", nil)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	rec2 := httptest.NewRecorder()
	protected.ServeHTTP(rec2, req2)

	require.True(t, protectedCalled)
	require.Equal(t, http.StatusNoContent, rec2.Code)
}
