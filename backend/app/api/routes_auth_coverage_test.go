package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/hay-kot/httpkit/errchain"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/reporting/eventbus"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
	"github.com/sysadminsmedia/homebox/backend/internal/web/mid"
)

// publicAPIRoutes is the explicit allowlist of /api/v1 routes that may be
// served without a session token. Anything mounted under /api/v1 that is not
// listed here must reject an anonymous request with 401, so adding a new route
// without auth middleware fails this test instead of silently shipping open.
var publicAPIRoutes = map[string]bool{
	"GET /api/v1/status":                    true,
	"GET /api/v1/currencies":                true,
	"POST /api/v1/users/register":           true,
	"POST /api/v1/users/login":              true,
	"POST /api/v1/users/forgot-password":    true,
	"POST /api/v1/users/reset-password":     true,
	"GET /api/v1/users/login/oidc":          true,
	"GET /api/v1/users/login/oidc/callback": true,
}

// TestAPIRoutesRequireAuthentication walks the real router and sends an
// anonymous request to every /api/v1 route.
func TestAPIRoutesRequireAuthentication(t *testing.T) {
	a, _ := newAuthTestApp(t)
	a.conf = &config.Config{}
	a.bus = eventbus.New()
	a.authLimiter = newAuthRateLimiter(config.AuthRateLimit{Enabled: false})
	a.notifierTestLimiter = newSimpleRateLimiter(1000, 60, false)
	a.registerLimiter = newSimpleRateLimiter(1000, 60, false)

	router := chi.NewRouter()
	chain := errchain.New(mid.Errors(zerolog.Nop()))
	a.mountRoutes(router, chain, a.repos)

	var checked int
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if !strings.HasPrefix(route, "/api/v1/") || method == http.MethodOptions {
			return nil
		}
		key := method + " " + strings.TrimSuffix(route, "/")
		if publicAPIRoutes[key] {
			return nil
		}

		path := strings.NewReplacer("{id}", "00000000-0000-0000-0000-000000000000", "*", "x").Replace(route)
		// Remaining named params (e.g. {attachmentId}) get a placeholder too.
		for strings.Contains(path, "{") {
			start := strings.Index(path, "{")
			end := strings.Index(path, "}")
			path = path[:start] + "x" + path[end+1:]
		}

		req := httptest.NewRequest(method, path, nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		checked++
		assert.Equalf(t, http.StatusUnauthorized, rec.Code,
			"%s must reject anonymous requests (got %d); add auth middleware or, if it is intentionally public, list it in publicAPIRoutes", key, rec.Code)
		return nil
	})
	require.NoError(t, err)
	t.Logf("verified %d protected routes reject anonymous requests", checked)
	require.Greater(t, checked, 20, "route walk found suspiciously few protected routes")
}
