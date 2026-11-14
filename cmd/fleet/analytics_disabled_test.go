package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fleetdm/fleet/v4/server/config"
	"github.com/fleetdm/fleet/v4/server/contexts/license"
	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/fleetdm/fleet/v4/server/mock"
	"github.com/stretchr/testify/require"
)

func TestUsageStatisticsDisabled(t *testing.T) {
	var requests atomic.Int64
	sink := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer sink.Close()

	for _, tier := range []string{fleet.TierFree, fleet.TierPremium, fleet.TierResearch} {
		for _, enabled := range []bool{false, true} {
			for _, allowDisable := range []bool{false, true} {
				t.Run(tier+"/enabled="+strconv.FormatBool(enabled)+"/allow_disable="+strconv.FormatBool(allowDisable), func(t *testing.T) {
					ds := new(mock.Store)
					ds.AppConfigFunc = func(context.Context) (*fleet.AppConfig, error) {
						return &fleet.AppConfig{ServerSettings: fleet.ServerSettings{EnableAnalytics: enabled}}, nil
					}
					ds.ShouldSendStatisticsFunc = func(context.Context, time.Duration, config.FleetConfig) (fleet.StatisticsPayload, bool, error) {
						return fleet.StatisticsPayload{Organization: "must not be collected"}, true, nil
					}
					ctx := license.NewContext(t.Context(), &fleet.LicenseInfo{Tier: tier, AllowDisableTelemetry: allowDisable})
					require.NoError(t, trySendStatistics(ctx, ds, fleet.StatisticsFrequency, sink.URL, config.FleetConfig{}, slog.Default()))
					require.False(t, ds.AppConfigFuncInvoked, "must return before reading even stale enabled settings")
					require.False(t, ds.ShouldSendStatisticsFuncInvoked, "must not collect product analytics")
					require.False(t, ds.CleanupStatisticsFuncInvoked)
					require.False(t, ds.RecordStatisticsSentFuncInvoked)
					require.Zero(t, requests.Load(), "must not upload analytics")
				})
			}
		}
	}

	// No database, config or license is needed to suppress the legacy upload path.
	require.NoError(t, trySendStatistics(context.Background(), nil, 0, sink.URL, config.FleetConfig{}, nil))
	require.Zero(t, requests.Load())
}
