package licensing

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	licensectx "github.com/fleetdm/fleet/v4/server/contexts/license"
	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/fleetdm/fleet/v4/server/platform/endpointer"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func TestLoadLocalResearchLicense(t *testing.T) {
	for _, enabled := range []string{"1", "true", "TRUE"} {
		t.Run(enabled, func(t *testing.T) {
			t.Setenv("FLEET_LICENSE_LOCAL_RESEARCH", enabled)
			lic, err := LoadLicense("")
			require.NoError(t, err)
			require.Equal(t, fleet.TierResearch, lic.Tier)
			require.Equal(t, "Local development and testing", lic.Organization)
			require.Contains(t, lic.Note, "not a Fleet subscription")
			require.Contains(t, lic.Note, "remote services")
			require.True(t, lic.IsPremium())
			require.False(t, lic.IsExpired())
			require.True(t, lic.Expiration.IsZero(), "research has no rolling expiry")
			require.Zero(t, lic.DeviceCount, "zero leaves the licensed-host quota unset")
			require.True(t, lic.IsAllowDisableTelemetry())

			ctx := licensectx.NewContext(context.Background(), lic)
			require.True(t, licensectx.IsPremium(ctx))
			require.True(t, licensectx.IsAllowDisableTelemetry(ctx))
			checker, ok := licensectx.FromContext(ctx)
			require.True(t, ok)
			require.Equal(t, fleet.TierResearch, checker.GetTier())
			require.Equal(t, lic.Organization, checker.GetOrganization())
			require.Zero(t, checker.GetDeviceCount())

			data, err := json.Marshal(lic)
			require.NoError(t, err)
			require.Contains(t, string(data), `"tier":"research"`)
			require.NotContains(t, string(data), `"tier":"premium"`)
			var restored fleet.LicenseInfo
			require.NoError(t, json.Unmarshal(data, &restored))
			require.True(t, restored.IsPremium())
			require.False(t, restored.IsExpired())
			restored.ForceUpgrade()
			require.Equal(t, fleet.TierResearch, restored.Tier)

			again, err := LoadLicense("")
			require.NoError(t, err)
			require.Equal(t, lic, again, "research metadata must be durable across reloads")
		})
	}
}

func TestLocalResearchLicenseOptIn(t *testing.T) {
	for _, disabled := range []string{"", "0", "false", "FALSE"} {
		t.Run(disabled, func(t *testing.T) {
			t.Setenv("FLEET_LICENSE_LOCAL_RESEARCH", disabled)
			lic, err := LoadLicense("")
			require.NoError(t, err)
			require.Equal(t, &fleet.LicenseInfo{Tier: fleet.TierFree}, lic)
		})
	}
	for _, invalid := range []string{"yes", "enabled", " true ", "research"} {
		t.Run(invalid, func(t *testing.T) {
			t.Setenv("FLEET_LICENSE_LOCAL_RESEARCH", invalid)
			lic, err := LoadLicense("")
			require.ErrorContains(t, err, "FLEET_LICENSE_LOCAL_RESEARCH")
			require.Nil(t, lic)
		})
	}

	t.Run("configured key conflicts with research", func(t *testing.T) {
		t.Setenv("FLEET_LICENSE_LOCAL_RESEARCH", "true")
		const key = "do-not-echo-configured-license-key"
		lic, err := LoadLicense(key)
		require.ErrorContains(t, err, "cannot be combined with a license key")
		require.NotContains(t, err.Error(), key)
		require.Nil(t, lic)
	})
	t.Run("disabling research restores free", func(t *testing.T) {
		t.Setenv("FLEET_LICENSE_LOCAL_RESEARCH", "true")
		lic, err := LoadLicense("")
		require.NoError(t, err)
		require.True(t, lic.IsPremium())
		t.Setenv("FLEET_LICENSE_LOCAL_RESEARCH", "false")
		lic, err = LoadLicense("")
		require.NoError(t, err)
		require.Equal(t, fleet.TierFree, lic.Tier)
		require.False(t, lic.IsPremium())
	})
}

func TestLocalResearchDoesNotAcceptUnsignedLicense(t *testing.T) {
	t.Setenv("FLEET_LICENSE_LOCAL_RESEARCH", "false")
	for _, key := range []string{"research", "premium", "not-a-jwt"} {
		lic, err := LoadLicense(key)
		require.Error(t, err)
		require.Nil(t, lic)
	}
	claims := &licenseClaims{
		StandardClaims: jwt.StandardClaims{Issuer: expectedIssuer, ExpiresAt: time.Now().Add(time.Hour).Unix()},
		Tier:           fleet.TierResearch,
		Devices:        1,
	}
	key, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	lic, err := LoadLicense(key)
	require.Error(t, err)
	require.Nil(t, lic)

	// The research tier is local-only even if a signed token claims that tier.
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Valid = true
	lic, err = validate(token)
	require.ErrorContains(t, err, "reserved for local development and testing")
	require.Nil(t, lic)
	require.False(t, strings.Contains(err.Error(), key))
}

type researchGatedRequest struct {
	Critical bool   `json:"critical" premium:"true"`
	Platform string `json:"platform"`
}

func (r *researchGatedRequest) ValidateRequest() error {
	if r.Platform != "darwin" {
		return errors.New("unsupported platform")
	}
	return nil
}

func TestLocalResearchPremiumRequestValidation(t *testing.T) {
	t.Setenv(LocalResearchEnv, "true")
	lic, err := LoadLicense("")
	require.NoError(t, err)
	decode := endpointer.MakeDecoder(researchGatedRequest{}, func(body io.Reader, req any) error {
		return json.NewDecoder(body).Decode(req)
	}, nil, nil, nil, nil, -1)
	ctx := licensectx.NewContext(context.Background(), lic)

	request := func(body string) *http.Request {
		return httptest.NewRequest("POST", "/", strings.NewReader(body))
	}
	_, err = decode(ctx, request(`{"critical":true,"platform":"darwin"}`))
	require.NoError(t, err, "premium-tagged options must accept local research")
	_, err = decode(context.Background(), request(`{"critical":true,"platform":"darwin"}`))
	require.ErrorContains(t, err, "requires a premium license")
	_, err = decode(ctx, request(`{"critical":true,"platform":"unsupported"}`))
	require.ErrorContains(t, err, "unsupported platform", "entitlements must not bypass request validation")
	_, err = decode(ctx, request(`{"critical":`))
	require.Error(t, err, "malformed requests must still fail")
}
