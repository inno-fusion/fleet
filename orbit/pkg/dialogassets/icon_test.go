package dialogassets

import (
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/fleetdm/fleet/v4/orbit/pkg/swiftdialog"
	"github.com/stretchr/testify/require"
)

func TestIcon(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path == "/invalid" {
			_, _ = w.Write([]byte("not an image"))
			return
		}
		width := 80
		if r.URL.Path == "/wide" {
			width = 200
		}
		w.Header().Set("Content-Type", "image/png")
		_ = png.Encode(w, image.NewRGBA(image.Rect(0, 0, width, 80)))
	}))
	defer server.Close()

	icon, size, err := Icon("")
	require.NoError(t, err)
	require.Equal(t, "none", icon)
	require.Equal(t, swiftdialog.DefaultIconSize, size)
	require.Zero(t, requests.Load())

	for _, tc := range []struct {
		path    string
		size    uint
		wantErr bool
	}{
		{"/square", 80, false},
		{"/wide", 200, false},
		{"/invalid", 80, true},
	} {
		t.Run(tc.path, func(t *testing.T) {
			url := server.URL + tc.path
			icon, size, err := Icon(url)
			require.Equal(t, tc.wantErr, err != nil)
			require.Equal(t, url, icon, "preserve an operator-selected logo even if sizing fails")
			require.Equal(t, tc.size, size)
		})
	}
	require.Equal(t, int64(3), requests.Load())
}
