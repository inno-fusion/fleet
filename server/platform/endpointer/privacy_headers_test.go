package endpointer

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrivacyBrowserSecurityHeaders(t *testing.T) {
	r := httptest.NewRecorder()
	_, err := WriteBrowserSecurityHeaders(r, true, false)
	require.NoError(t, err)
	csp := r.Header().Get("Content-Security-Policy")
	require.NotContains(t, csp, "gravatar")
	require.Contains(t, csp, "connect-src 'self' ws: wss:;")
	require.Contains(t, csp, "img-src 'self' data: https:;")
	require.True(t, strings.Contains(csp, "script-src 'self'"), "scripts must remain same-origin")
	require.Equal(t, "nosniff", r.Header().Get("X-Content-Type-Options"))
}
