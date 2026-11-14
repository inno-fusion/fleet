package mysql

import (
	"context"
	"testing"
	"time"

	"github.com/fleetdm/fleet/v4/server/config"
	"github.com/fleetdm/fleet/v4/server/contexts/ctxerr"
	"github.com/fleetdm/fleet/v4/server/contexts/license"
	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/stretchr/testify/require"
)

func TestProductAnalyticsNeverCollected(t *testing.T) {
	for _, tier := range []string{fleet.TierFree, fleet.TierPremium, fleet.TierResearch} {
		t.Run(tier, func(t *testing.T) {
			// A nil datastore proves this path cannot read inventories, create an
			// anonymous identifier, or require even an available database.
			var ds *Datastore
			handler := &ctxerr.MockHandler{RetrieveImpl: func(bool) ([]*ctxerr.StoredError, error) {
				t.Fatal("must not retrieve errors for vendor analytics")
				return nil, nil
			}}
			ctx := license.NewContext(ctxerr.NewContext(context.Background(), handler), &fleet.LicenseInfo{Tier: tier})
			stats, send, err := ds.ShouldSendStatistics(ctx, time.Nanosecond, config.FleetConfig{})
			require.NoError(t, err)
			require.False(t, send)
			require.Equal(t, fleet.StatisticsPayload{}, stats)
		})
	}
}
