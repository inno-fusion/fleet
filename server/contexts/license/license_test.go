package license

import (
	"context"
	"testing"

	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/stretchr/testify/require"
)

func TestIsPremium(t *testing.T) {
	cases := []struct {
		desc string
		ctx  context.Context
		want bool
	}{
		{"no license", context.Background(), false},
		{"free license", NewContext(context.Background(), &fleet.LicenseInfo{Tier: fleet.TierFree}), false},
		{"premium license", NewContext(context.Background(), &fleet.LicenseInfo{Tier: fleet.TierPremium}), true},
		{"trial license", NewContext(context.Background(), &fleet.LicenseInfo{Tier: fleet.TierTrial}), true},
		{"research entitlement", NewContext(context.Background(), &fleet.LicenseInfo{Tier: fleet.TierResearch}), true},
		{"unrecognized tier", NewContext(context.Background(), &fleet.LicenseInfo{Tier: "unrecognized"}), false},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			got := IsPremium(c.ctx)
			require.Equal(t, c.want, got)
		})
	}
}
