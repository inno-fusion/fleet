package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/fleetdm/fleet/v4/server/config"
	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/fleetdm/fleet/v4/server/mdm/android"
	"github.com/fleetdm/fleet/v4/server/mdm/android/service/androidmgmt"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfiguredAMAPIClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, tc := range []struct {
		name    string
		cfg     config.MDMConfig
		wantErr string
	}{
		{name: "default proxy"},
		{name: "own proxy", cfg: config.MDMConfig{AndroidProxyURL: "https://mdm.example.test/api/android/"}},
		{name: "insecure proxy", cfg: config.MDMConfig{AndroidProxyURL: "http://mdm.example.test/"}, wantErr: "HTTPS"},
		{name: "proxy credentials in URL", cfg: config.MDMConfig{AndroidProxyURL: "https://user:secret@example.test/"}, wantErr: "HTTPS"},
		{name: "missing direct credentials", cfg: config.MDMConfig{AndroidGoogleServiceCredentials: "/nonexistent/credentials.json"}, wantErr: "read Android"},
		{name: "ambiguous config", cfg: config.MDMConfig{AndroidGoogleServiceCredentials: "credentials.json", AndroidProxyURL: "https://example.test/"}, wantErr: "mutually exclusive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewAMAPIClientWithConfig(t.Context(), logger, "", tc.cfg)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				require.Nil(t, client)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, client)
		})
	}

	for _, data := range []string{"not JSON secret", `{}`, `{"type":"authorized_user","project_id":"project"}`} {
		path := filepath.Join(t.TempDir(), "google.json")
		require.NoError(t, os.WriteFile(path, []byte(data), 0600))
		client, err := NewAMAPIClientWithConfig(t.Context(), logger, "", config.MDMConfig{AndroidGoogleServiceCredentials: path})
		require.Error(t, err)
		require.NotContains(t, err.Error(), data)
		require.Nil(t, client)
	}
}

type androidConfigRoundTripper func(*http.Request) (*http.Response, error)

func (fn androidConfigRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

func TestAndroidReconciliationClientsAreIsolated(t *testing.T) {
	ds := InitCommonDSMocks()
	ds.GetEnterpriseFunc = func(context.Context) (*android.Enterprise, error) { return nil, &notFoundError{} }
	ds.GetAllMDMConfigAssetsByNameFunc = func(context.Context, []fleet.MDMAssetName, sqlx.QueryerContext) (map[fleet.MDMAssetName]fleet.MDMConfigAsset, error) {
		return map[fleet.MDMAssetName]fleet.MDMConfigAsset{fleet.MDMAssetAndroidFleetServerSecret: {Value: []byte("initial-secret")}}, nil
	}
	original := http.DefaultTransport
	http.DefaultTransport = androidConfigRoundTripper(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "proxy.example.test", r.URL.Host)
		if r.URL.Path == "/v1/enterprises" {
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"enterprises":[]}`))}, nil
		}
		require.Equal(t, "Bearer "+strings.TrimPrefix(r.URL.Path, "/v1/enterprises/E/devices/"), r.Header.Get("Authorization"))
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = original })
	cfg := config.MDMConfig{AndroidProxyURL: "https://proxy.example.test/"}
	logger := slog.New(slog.DiscardHandler)
	first, err := newAuthenticatedAMAPIClient(t.Context(), ds, logger, "license", cfg)
	require.NoError(t, err)
	second, err := newAuthenticatedAMAPIClient(t.Context(), ds, logger, "license", cfg)
	require.NoError(t, err)
	require.NotSame(t, first, second)
	require.NoError(t, first.SetAuthenticationSecret("first"))
	require.NoError(t, second.SetAuthenticationSecret("second"))
	var wg sync.WaitGroup
	for _, tc := range []struct {
		client androidmgmt.Client
		secret string
	}{{first, "first"}, {second, "second"}} {
		wg.Go(func() {
			for range 10 {
				assert.NoError(t, tc.client.SetAuthenticationSecret(tc.secret))
				_, err := tc.client.EnterprisesDevicesGet(t.Context(), "enterprises/E/devices/"+tc.secret)
				assert.NoError(t, err)
			}
		})
	}
	wg.Wait()
}

func TestAndroidReconciliationFailsClosedOnConfigurationError(t *testing.T) {
	for _, job := range []string{"profiles", "devices", "commands"} {
		for _, failure := range []string{"configuration", "authentication"} {
			t.Run(job+"/"+failure, func(t *testing.T) {
				ds := InitCommonDSMocks()
				ds.AppConfigFunc = func(context.Context) (*fleet.AppConfig, error) {
					return &fleet.AppConfig{MDM: fleet.MDM{AndroidEnabledAndConfigured: true}}, nil
				}
				cfg := config.MDMConfig{AndroidProxyURL: "http://invalid.example.test"}
				if failure == "authentication" {
					cfg.AndroidProxyURL = "https://proxy.example.test"
					ds.GetAllMDMConfigAssetsByNameFunc = func(context.Context, []fleet.MDMAssetName, sqlx.QueryerContext) (map[fleet.MDMAssetName]fleet.MDMConfigAsset, error) {
						return nil, errors.New("secret unavailable")
					}
				}
				logger := slog.New(slog.DiscardHandler)
				var err error
				switch job {
				case "profiles":
					err = ReconcileProfiles(t.Context(), ds, logger, "license", cfg, config.AndroidAgentConfig{}, 100)
				case "devices":
					err = ReconcileAndroidDevices(t.Context(), ds, logger, "license", cfg, noopNewActivity)
				case "commands":
					err = ReconcileAndroidCommands(t.Context(), ds, logger, "license", cfg, noopNewActivity)
				}
				require.Error(t, err)
				require.False(t, ds.ListMDMAndroidProfilesToSendFuncInvoked)
				require.False(t, ds.ListAndroidEnrolledDevicesForReconcileFuncInvoked)
				require.False(t, ds.FailPendingMDMAndroidCommandsOutsidePrefixFuncInvoked)
			})
		}
	}
}
