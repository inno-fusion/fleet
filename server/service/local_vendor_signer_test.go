package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/fleetdm/fleet/v4/server/config"
	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/fleetdm/fleet/v4/server/mock"
	"github.com/fleetdm/fleet/v4/server/test"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func TestLocalVendorSigningNeverFallsBack(t *testing.T) {
	var calls atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(remote.Close)
	t.Setenv("TEST_FLEETDM_API_URL", remote.URL)
	cfg := config.TestConfig()
	cfg.MDM.AppleMDMVendorCert = "/missing/vendor.pem"
	cfg.MDM.AppleMDMVendorKey = "/missing/vendor.key"
	cfg.MDM.AppleMDMVendorChain = "/missing/chain.pem"
	ds := new(mock.Store)
	svc, ctx := newTestServiceWithConfig(t, ds, cfg, nil, nil, &TestServerOpts{SkipCreateTestUsers: true})
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	ds.GetAllMDMConfigAssetsByNameFunc = func(context.Context, []fleet.MDMAssetName, sqlx.QueryerContext) (map[fleet.MDMAssetName]fleet.MDMConfigAsset, error) {
		return map[fleet.MDMAssetName]fleet.MDMConfigAsset{fleet.MDMAssetAPNSKey: {Value: keyPEM}}, nil
	}
	ds.AppConfigFunc = func(context.Context) (*fleet.AppConfig, error) {
		return &fleet.AppConfig{OrgInfo: fleet.OrgInfo{OrgName: "Research"}}, nil
	}
	ctx = test.UserContext(ctx, test.UserAdmin)
	_, err = svc.GetMDMAppleCSR(ctx)
	require.Error(t, err)
	require.Zero(t, calls.Load(), "local signing failure must not transmit CSR to Fleet")
}
