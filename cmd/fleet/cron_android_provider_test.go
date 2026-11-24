package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fleetdm/fleet/v4/server/config"
	"github.com/fleetdm/fleet/v4/server/datastore/redis/redistest"
	"github.com/fleetdm/fleet/v4/server/dev_mode"
	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/fleetdm/fleet/v4/server/mdm/android"
	android_svc "github.com/fleetdm/fleet/v4/server/mdm/android/service"
	ds_mock "github.com/fleetdm/fleet/v4/server/mock"
	"github.com/fleetdm/fleet/v4/server/ptr"
	"github.com/fleetdm/fleet/v4/server/service/redis_install_attempts"
	"github.com/fleetdm/fleet/v4/server/service/schedule"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Trap all HTTPS requests, including Google's token exchange, on a local TLS
// server while retaining the original host/path so routing is actually tested.
func interceptAndroidHTTPS(t *testing.T, handler http.Handler) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialTLSContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		dialer := tls.Dialer{Config: server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()}
		return dialer.DialContext(ctx, network, server.Listener.Addr().String())
	}
	original := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() {
		transport.CloseIdleConnections()
		http.DefaultTransport = original
	})
}

func androidTestCredentials(t *testing.T, project string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	data, err := json.Marshal(map[string]string{
		"type": "service_account", "project_id": project,
		"client_email": "test@" + project + ".iam.gserviceaccount.com",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})),
		"token_uri":    "https://oauth.example.test/token",
	})
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "google.json")
	require.NoError(t, os.WriteFile(path, data, 0600))
	return path
}

func androidCronDatastore(t *testing.T, completed chan<- fleet.CronScheduleErrors) *ds_mock.Store {
	t.Helper()
	ds := &ds_mock.Store{}
	ds.GetLatestCronStatsFunc = func(context.Context, string) ([]fleet.CronStats, error) { return nil, nil }
	ds.InsertCronStatsFunc = func(context.Context, fleet.CronStatsType, string, string, fleet.CronStatsStatus) (int, error) {
		return 1, nil
	}
	ds.UpdateCronStatsFunc = func(_ context.Context, _ int, _ fleet.CronStatsStatus, errs *fleet.CronScheduleErrors) error {
		completed <- *errs
		return nil
	}
	ds.LockFunc = func(context.Context, string, string, time.Duration) (bool, error) { return true, nil }
	ds.UnlockFunc = func(context.Context, string, string) error { return nil }
	ds.AppConfigFunc = func(context.Context) (*fleet.AppConfig, error) {
		return &fleet.AppConfig{MDM: fleet.MDM{AndroidEnabledAndConfigured: true}}, nil
	}
	ds.GetEnterpriseFunc = func(context.Context) (*android.Enterprise, error) { return &android.Enterprise{EnterpriseID: "E"}, nil }
	ds.GetAllMDMConfigAssetsByNameFunc = func(context.Context, []fleet.MDMAssetName, sqlx.QueryerContext) (map[fleet.MDMAssetName]fleet.MDMConfigAsset, error) {
		return map[fleet.MDMAssetName]fleet.MDMConfigAsset{fleet.MDMAssetAndroidFleetServerSecret: {Value: []byte("cron-secret")}}, nil
	}
	ds.ListAndroidEnrolledDevicesForReconcileFunc = func(context.Context) ([]*android.Device, error) {
		return []*android.Device{{HostID: 1, DeviceID: "D"}}, nil
	}
	ds.FailPendingMDMAndroidCommandsOutsidePrefixFunc = func(context.Context, string, time.Time, string, int) (int64, error) { return 0, nil }
	ds.ListPendingMDMAndroidCommandsFunc = func(context.Context, string, time.Time, int) ([]*android.MDMAndroidCommand, error) {
		return []*android.MDMAndroidCommand{{OperationName: "enterprises/E/devices/D/operations/O"}}, nil
	}
	ds.GetMDMAndroidReconcileCursorFunc = func(context.Context) (string, error) { return "", nil }
	ds.ListAndroidHostUUIDsWithPendingCertificateTemplatesFunc = func(context.Context, string, int) ([]string, error) { return nil, nil }
	ds.ListAndroidHostsOnHostPolicyFunc = func(context.Context) ([]*fleet.Host, error) { return nil, nil }
	ds.ListAndroidAppsInScopeForHostsFunc = func(context.Context, []uint) (map[uint][]fleet.VPPAppTeam, error) { return nil, nil }
	ds.ListHostMDMAndroidVPPAppsFromSetupExperienceFunc = func(context.Context, []uint) (map[uint][]*fleet.HostAndroidVPPSoftwareInstall, error) {
		return nil, nil
	}
	ds.ListHostAndroidSoftwareInstallsFunc = func(context.Context, []uint) (map[uint]map[string]*fleet.HostAndroidSoftwareInstall, error) {
		return nil, nil
	}
	ds.GetAndroidAppStoreAppVersionsByIDFunc = func(context.Context) (map[uint]*fleet.VPPAppStoreApp, error) { return nil, nil }
	ds.BulkUpsertHostAndroidSoftwareInstallsFunc = func(context.Context, []fleet.HostAndroidSoftwareInstall, []fleet.HostAndroidSoftwareInstall) error {
		return nil
	}
	ds.SetHostAndroidSoftwareInstallsNeedsResendFunc = func(context.Context, []uint, *time.Time, *string) error { return nil }
	ds.ListAndroidFleetDefaultPolicyProfilesFunc = func(context.Context, *uint) ([]*fleet.MDMAndroidConfigProfile, error) { return nil, nil }
	ds.GetAndroidFleetDefaultPolicyChecksumsFunc = func(context.Context) (map[uint][]byte, error) { return nil, nil }
	ds.ListAndroidFleetDefaultPolicyDeletedFleetsFunc = func(context.Context) ([]uint, error) { return nil, nil }
	ds.GetMDMAndroidProfilesWriteTimeFunc = func(context.Context) (time.Time, error) { return time.Now(), nil }
	ds.ListMDMAndroidProfilesToSendFunc = func(context.Context, string, int) ([]*fleet.MDMAndroidProfilePayload, []*fleet.MDMAndroidProfilePayload, error) {
		return []*fleet.MDMAndroidProfilePayload{{HostUUID: "host", ProfileUUID: "profile", ProfileName: "policy"}}, nil, nil
	}
	ds.GetMDMAndroidProfilesContentsFunc = func(context.Context, []string) (map[string]json.RawMessage, error) {
		return map[string]json.RawMessage{"profile": json.RawMessage(`{"cameraDisabled":true}`)}, nil
	}
	ds.NewAndroidPolicyRequestFunc = func(_ context.Context, req *android.MDMAndroidPolicyRequest) error {
		assert.Equal(t, http.StatusOK, req.StatusCode)
		return nil
	}
	ds.AndroidHostLiteByHostUUIDFunc = func(context.Context, string) (*fleet.AndroidHost, error) {
		return &fleet.AndroidHost{Device: &android.Device{AppliedPolicyID: ptr.String("host")}}, nil
	}
	ds.BulkUpsertMDMAndroidHostProfilesUnlessResetSinceFunc = func(_ context.Context, profiles []*fleet.MDMAndroidProfilePayload, _ time.Time) error {
		assert.Len(t, profiles, 1)
		return nil
	}
	return ds
}

func TestAndroidCronsUseConfiguredProvider(t *testing.T) {
	dev_mode.SetOverride("FLEET_DEV_ANDROID_GOOGLE_CLIENT", "", t)
	// A production setting must override dev routing as well as the vendor default.
	dev_mode.SetOverride("FLEET_DEV_ANDROID_PROXY_ENDPOINT", "https://dev.example.test/", t)
	for _, provider := range []string{"custom proxy", "direct Google", "legacy dev proxy", "default proxy"} {
		for _, job := range []string{"profiles", "devices", "commands"} {
			t.Run(provider+"/"+job, func(t *testing.T) {
				cfg := config.MDMConfig{}
				host, prefix := "dev.example.test", ""
				switch provider {
				case "custom proxy":
					cfg.AndroidProxyURL = "https://proxy.example.test/android/"
					host, prefix = "proxy.example.test", "/android"
				case "default proxy":
					dev_mode.SetOverride("FLEET_DEV_ANDROID_PROXY_ENDPOINT", "", t)
					host, prefix = "fleetdm.com", "/api/android"
				case "direct Google":
					cfg.AndroidGoogleServiceCredentials = androidTestCredentials(t, "project-b")
					host = "androidmanagement.googleapis.com"
				}
				var calls atomic.Int64
				interceptAndroidHTTPS(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if r.Host == "oauth.example.test" {
						assert.Equal(t, "/token", r.URL.Path)
						_, _ = io.WriteString(w, `{"access_token":"google-test-token","token_type":"Bearer","expires_in":3600}`)
						return
					}
					calls.Add(1)
					assert.Equal(t, host, r.Host, "background traffic must use the configured provider")
					if r.URL.Path == prefix+"/v1/enterprises" {
						_, _ = io.WriteString(w, `{"enterprises":[{"name":"enterprises/E"}]}`)
						return
					}
					if provider == "direct Google" {
						assert.True(t, strings.HasPrefix(r.Header.Get("Authorization"), "Bearer "))
						assert.NotEqual(t, "Bearer cron-secret", r.Header.Get("Authorization"))
					} else {
						assert.Equal(t, "Bearer cron-secret", r.Header.Get("Authorization"))
					}
					switch job {
					case "profiles":
						assert.Equal(t, "PATCH", r.Method)
						assert.Equal(t, prefix+"/v1/enterprises/E/policies/host", r.URL.Path)
						_, _ = io.WriteString(w, `{"name":"enterprises/E/policies/host","version":"1"}`)
					case "devices":
						assert.Equal(t, "GET", r.Method)
						assert.Equal(t, prefix+"/v1/enterprises/E/devices", r.URL.Path)
						_, _ = io.WriteString(w, `{"devices":[{"name":"enterprises/E/devices/D"}]}`)
					case "commands":
						assert.Equal(t, "GET", r.Method)
						assert.Equal(t, prefix+"/v1/enterprises/E/devices/D/operations/O", r.URL.Path)
						_, _ = io.WriteString(w, `{"name":"enterprises/E/devices/D/operations/O","done":false}`)
					}
				}))
				completed := make(chan fleet.CronScheduleErrors, 1)
				ds := androidCronDatastore(t, completed)
				ctx, cancel := context.WithCancel(t.Context())
				logger := slog.New(slog.DiscardHandler)
				var sch *schedule.Schedule
				var err error
				noopActivity := func(context.Context, *fleet.User, fleet.ActivityDetails) error { return nil }
				switch job {
				case "profiles":
					sch, err = newAndroidMDMProfileManagerSchedule(ctx, "test", ds, logger, "license", cfg, config.AndroidAgentConfig{}, 100, redistest.NopRedis())
				case "devices":
					sch, err = newAndroidMDMDeviceReconcilerSchedule(ctx, "test", ds, logger, "license", cfg, noopActivity)
				case "commands":
					sch, err = newAndroidMDMCommandReconcilerSchedule(ctx, "test", ds, logger, "license", cfg, noopActivity)
				}
				require.NoError(t, err)
				sch.Start()
				t.Cleanup(func() { cancel(); <-sch.Done() })
				require.Eventually(t, func() bool {
					_, triggered, err := sch.Trigger(ctx)
					require.NoError(t, err)
					return triggered
				}, 5*time.Second, time.Millisecond)
				select {
				case errs := <-completed:
					require.Empty(t, errs)
				case <-time.After(5 * time.Second):
					t.Fatal("Android cron did not finish")
				}
				assert.EqualValues(t, 2, calls.Load())
				assert.False(t, ds.SetAndroidHostUnenrolledFuncInvoked)
				assert.False(t, ds.UpdateMDMAndroidCommandStatusFuncInvoked)
			})
		}
	}
}

func TestAndroidProviderSwitchPreservesEnterprise(t *testing.T) {
	for _, provider := range []string{"direct Google", "custom proxy"} {
		for _, response := range []string{`{}`, `{"enterprises":[{"name":"enterprises/different"}]}`} {
			t.Run(provider+"/"+response, func(t *testing.T) {
				cfg := config.MDMConfig{AndroidProxyURL: "https://proxy.example.test/android"}
				if provider == "direct Google" {
					cfg = config.MDMConfig{AndroidGoogleServiceCredentials: androidTestCredentials(t, "new-project")}
				}
				var calls atomic.Int64
				interceptAndroidHTTPS(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if r.Host == "oauth.example.test" {
						_, _ = io.WriteString(w, `{"access_token":"google-test-token","token_type":"Bearer","expires_in":3600}`)
						return
					}
					calls.Add(1)
					assert.Equal(t, "GET", r.Method, "verification must not issue provider writes")
					if provider == "direct Google" {
						assert.Equal(t, "androidmanagement.googleapis.com", r.Host)
						assert.Equal(t, "new-project", r.URL.Query().Get("projectId"))
					} else {
						assert.Equal(t, "proxy.example.test", r.Host)
						assert.Equal(t, "Bearer cron-secret", r.Header.Get("Authorization"))
					}
					_, _ = io.WriteString(w, response)
				}))
				ds := androidCronDatastore(t, make(chan fleet.CronScheduleErrors, 1))
				logger := slog.New(slog.DiscardHandler)
				client, err := android_svc.NewAMAPIClientWithConfig(t.Context(), logger, "license", cfg)
				require.NoError(t, err)
				svc, err := android_svc.NewServiceWithClient(logger, ds, client, "test-key", ds, nil, config.AndroidAgentConfig{})
				require.NoError(t, err)
				err = svc.VerifyExistingEnterpriseIfAny(t.Context())
				require.ErrorContains(t, err, "not visible to the configured provider or Google project")
				require.False(t, fleet.IsNotFound(err))
				for _, job := range []string{"profiles", "devices", "commands"} {
					switch job {
					case "profiles":
						err = android_svc.ReconcileProfiles(t.Context(), ds, logger, "license", cfg, config.AndroidAgentConfig{}, 100, redis_install_attempts.New(redistest.NopRedis(), redis_install_attempts.AppStoreAppPrefix))
					case "devices":
						err = android_svc.ReconcileAndroidDevices(t.Context(), ds, logger, "license", cfg, nil)
					case "commands":
						err = android_svc.ReconcileAndroidCommands(t.Context(), ds, logger, "license", cfg, nil)
					}
					require.ErrorContains(t, err, "not visible to the configured provider or Google project", job)
				}
				assert.EqualValues(t, 4, calls.Load())
				assert.False(t, ds.ListAndroidEnrolledDevicesForReconcileFuncInvoked)
				assert.False(t, ds.FailPendingMDMAndroidCommandsOutsidePrefixFuncInvoked)
				assert.False(t, ds.ListMDMAndroidProfilesToSendFuncInvoked)
				assert.False(t, ds.DeleteZeroTouchEnrollmentTokensFuncInvoked)
				assert.False(t, ds.DeleteAllEnterprisesFuncInvoked)
				assert.False(t, ds.SetAndroidEnabledAndConfiguredFuncInvoked)
				assert.False(t, ds.BulkSetAndroidHostsUnenrolledFuncInvoked)
				assert.False(t, ds.MarkAllPendingAndroidVPPInstallsAsFailedFuncInvoked)
			})
		}
	}
}
