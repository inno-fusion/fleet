package fleet

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fleetdm/fleet/v4/server/ptr"
	"github.com/stretchr/testify/require"
)

func TestLicenseEntitlements(t *testing.T) {
	t.Parallel()
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)
	for _, tc := range []struct {
		name         string
		license      LicenseInfo
		premium      bool
		expired      bool
		canDisable   bool
		upgradedTier string
	}{
		{"free", LicenseInfo{Tier: TierFree}, false, true, true, TierFree},
		{"premium", LicenseInfo{Tier: TierPremium, DeviceCount: 100, Expiration: future}, true, false, false, TierPremium},
		{"expired premium", LicenseInfo{Tier: TierPremium, DeviceCount: 100, Expiration: past}, true, true, false, TierPremium},
		{"trial", LicenseInfo{Tier: TierTrial, DeviceCount: 10, Expiration: future}, true, false, false, TierTrial},
		{"expired trial", LicenseInfo{Tier: TierTrial, DeviceCount: 10, Expiration: past}, true, true, false, TierTrial},
		{"basic", LicenseInfo{Tier: tierBasicDeprecated, Expiration: future}, true, false, false, TierPremium},
		{"unknown", LicenseInfo{Tier: "unrecognized"}, false, true, true, "unrecognized"},
		{"premium zero expiry", LicenseInfo{Tier: TierPremium}, true, true, false, TierPremium},
		{"trial zero expiry", LicenseInfo{Tier: TierTrial}, true, true, false, TierTrial},
		{"research", LicenseInfo{Tier: TierResearch}, true, false, true, TierResearch},
		{"research is not a trial", LicenseInfo{Tier: TierResearch, Expiration: past}, true, false, true, TierResearch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.premium, tc.license.IsPremium())
			require.Equal(t, tc.expired, tc.license.IsExpired())
			require.Equal(t, tc.canDisable, tc.license.IsAllowDisableTelemetry())
			originalCount, originalExpiry := tc.license.DeviceCount, tc.license.Expiration
			tc.license.ForceUpgrade()
			require.Equal(t, tc.upgradedTier, tc.license.GetTier())
			require.Equal(t, originalCount, tc.license.GetDeviceCount())
			require.Equal(t, originalExpiry, tc.license.Expiration)

			data, err := json.Marshal(tc.license)
			require.NoError(t, err)
			var restored LicenseInfo
			require.NoError(t, json.Unmarshal(data, &restored))
			require.Equal(t, tc.premium, restored.IsPremium())
			require.Equal(t, tc.expired, restored.IsExpired())
		})
	}
}

func TestResearchLicensePreservesRoleRequirements(t *testing.T) {
	t.Parallel()
	lic := LicenseInfo{Tier: TierResearch}
	payload := UserPayload{GlobalRole: ptr.String(RoleGitOps), APIOnly: ptr.Bool(true)}
	require.NoError(t, ValidateUserRoles(true, payload, lic))
	require.ErrorIs(t, ValidateUserRoles(true, payload, LicenseInfo{Tier: TierFree}), ErrMissingLicense)

	payload.APIOnly = ptr.Bool(false)
	err := ValidateUserRoles(true, payload, lic)
	var roleErr *Error
	require.ErrorAs(t, err, &roleErr)
	require.Equal(t, ErrAPIOnlyRole, roleErr.Code)

	payload.GlobalRole = ptr.String("invalid-role")
	require.Error(t, ValidateUserRoles(true, payload, lic))
}
