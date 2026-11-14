package setupexperience

import (
	"encoding/json"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fleetdm/fleet/v4/orbit/pkg/swiftdialog"
	"github.com/stretchr/testify/require"
)

func TestSetupDialogPresentationAssets(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_ = png.Encode(w, image.NewRGBA(image.Rect(0, 0, 200, 80)))
	}))
	defer server.Close()

	for _, tc := range []struct {
		name      string
		logo      string
		legacy    bool
		webview   bool
		wantIcon  string
		wantSize  uint
		wantFetch bool
	}{
		{name: "initial without logo", wantIcon: "none", wantSize: 80},
		{name: "legacy without logo", legacy: true, wantIcon: "none", wantSize: 80},
		{name: "initial custom logo", logo: server.URL + "/org.png", wantIcon: server.URL + "/org.png", wantSize: 200, wantFetch: true},
		{name: "legacy custom logo", legacy: true, logo: server.URL + "/org.png", wantIcon: server.URL + "/org.png", wantSize: 200, wantFetch: true},
		{name: "webview without logo", webview: true, wantIcon: "none"},
		{name: "webview does not fetch hidden logo", logo: server.URL + "/org.png", webview: true, wantIcon: "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			output := filepath.Join(dir, "options.json")
			t.Setenv("FLEET_TEST_DIALOG_OPTIONS", output)
			binary := filepath.Join(dir, "dialog")
			// Capture the real swiftDialog command's JSON without opening a GUI.
			require.NoError(t, os.WriteFile(binary, []byte("#!/bin/sh\nprintf '%s' \"$2\" > \"$FLEET_TEST_DIALOG_OPTIONS\"\nprintf '{}'\n"), 0o700))
			s := &SetupExperiencer{closeChan: make(chan struct{}, 1)}
			before := requests.Load()
			if tc.legacy {
				require.NoError(t, s.startSwiftDialogLegacy(binary, tc.logo))
			} else {
				require.NoError(t, s.startSwiftDialog(binary, tc.logo, tc.webview))
			}
			select {
			case <-s.closeChan:
			case <-time.After(5 * time.Second):
				t.Fatal("dialog did not finish")
			}
			data, err := os.ReadFile(output)
			require.NoError(t, err)
			var opts swiftdialog.SwiftDialogOptions
			require.NoError(t, json.Unmarshal(data, &opts))
			require.Equal(t, tc.wantIcon, opts.Icon)
			require.Equal(t, tc.wantSize, opts.IconSize)
			require.NotContains(t, string(data), "fleetdm.com/images")
			require.Equal(t, tc.wantFetch, requests.Load() > before)
			if tc.webview {
				require.Equal(t, "none", opts.Message)
			} else {
				require.Contains(t, opts.Message, "Setting up your Mac")
			}
			require.True(t, opts.Button1Disabled)
		})
	}
}
