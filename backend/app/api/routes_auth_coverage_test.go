package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hay-kot/httpkit/errchain"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sysadminsmedia/homebox/backend/internal/core/services/reporting/eventbus"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/config"
	"github.com/sysadminsmedia/homebox/backend/internal/sys/otel"
	"github.com/sysadminsmedia/homebox/backend/internal/web/mid"
)

// publicAPIRoutes is the explicit allowlist of /api/v1 routes that may be
// served without a session token. Anything mounted under /api/v1 that is not
// listed here must reject an anonymous request with 401, so adding a new route
// without auth middleware fails this test instead of silently shipping open.
// The test also fails if an entry is never seen on any walk, so a renamed or
// removed public route cannot leave a stale exemption behind.
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

// routeParam matches chi URL params (including regexp params such as
// {id:[0-9]+}) and wildcards.
var routeParam = regexp.MustCompile(`\{[^}]*\}|\*`)

// walkAPIRoutes mounts the real router for cfg (and optionally an OpenTelemetry
// provider), sends an anonymous request to every /api/v1 route that is not on
// the allowlist and asserts a 401. It returns the allowlist keys it saw and the
// number of protected routes it checked.
func walkAPIRoutes(t *testing.T, cfg *config.Config, provider *otel.Provider) (seenPublic map[string]bool, checked int) {
	t.Helper()

	a, _ := newAuthTestApp(t)
	a.conf = cfg
	a.otel = provider
	a.bus = eventbus.New()
	a.authLimiter = newAuthRateLimiter(config.AuthRateLimit{Enabled: false})
	a.notifierTestLimiter = newSimpleRateLimiter(1000, time.Minute, false)
	a.registerLimiter = newSimpleRateLimiter(1000, time.Minute, false)
	t.Cleanup(func() {
		a.authLimiter.Stop()
		a.notifierTestLimiter.Stop()
		a.registerLimiter.Stop()
	})

	router := chi.NewRouter()
	chain := errchain.New(mid.Errors(zerolog.Nop()))
	a.mountRoutes(router, chain, a.repos)

	seenPublic = map[string]bool{}
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if !strings.HasPrefix(route, "/api/v1/") || method == http.MethodOptions {
			return nil
		}
		key := method + " " + strings.TrimSuffix(route, "/")
		if publicAPIRoutes[key] {
			seenPublic[key] = true
			return nil
		}

		path := routeParam.ReplaceAllString(route, "00000000-0000-0000-0000-000000000000")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, path, nil))

		checked++
		assert.Equalf(t, http.StatusUnauthorized, rec.Code,
			"%s must reject anonymous requests (got %d); add auth middleware or, if it is intentionally public, list it in publicAPIRoutes", key, rec.Code)
		return nil
	})
	require.NoError(t, err)
	return seenPublic, checked
}

// TestAPIRoutesRequireAuthentication walks the real router twice: once with the
// default configuration and once with the config-gated route groups switched on
// (OIDC login and the OpenTelemetry telemetry proxy), so those are covered too.
func TestAPIRoutesRequireAuthentication(t *testing.T) {
	seen := map[string]bool{}

	base, baseChecked := walkAPIRoutes(t, &config.Config{}, nil)
	for k := range base {
		seen[k] = true
	}
	t.Logf("default config: %d protected routes reject anonymous requests", baseChecked)
	require.GreaterOrEqual(t, baseChecked, 100, "route walk found suspiciously few protected routes")

	otelCfg := config.OTelConfig{Enabled: true, Exporter: "none", ServiceName: "homebox-test"}
	provider, err := otel.NewProvider(context.Background(), &otelCfg, "test")
	require.NoError(t, err)
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	gated := &config.Config{Otel: otelCfg}
	gated.OIDC.Enabled = true
	gated.Otel.ProxyEnabled = true
	full, fullChecked := walkAPIRoutes(t, gated, provider)
	for k := range full {
		seen[k] = true
	}
	t.Logf("OIDC + telemetry enabled: %d protected routes reject anonymous requests", fullChecked)
	require.Greater(t, fullChecked, baseChecked, "telemetry proxy route was not mounted; config-gated walk is not covering it")

	for key := range publicAPIRoutes {
		assert.Truef(t, seen[key], "allowlisted public route %q was never mounted; remove the stale entry", key)
	}
}
