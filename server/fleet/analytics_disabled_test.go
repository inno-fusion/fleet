package fleet

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProductAnalyticsDefaultsAndSerialization(t *testing.T) {
	c := AppConfig{ServerSettings: ServerSettings{EnableAnalytics: true}}
	c.ApplyDefaultsForNewInstalls()
	require.False(t, c.ServerSettings.EnableAnalytics)
	require.True(t, c.Features.EnableHostUsers)
	require.True(t, c.Features.EnableSoftwareInventory)
	require.True(t, c.Features.HistoricalData.Uptime)
	require.True(t, c.Features.HistoricalData.Vulnerabilities)
	require.NotNil(t, c.AgentOptions)
	require.Contains(t, string(*c.AgentOptions), `"distributed_plugin": "tls"`)

	// A stale in-memory opt-in must not survive any config persistence or API
	// serialization, without changing unrelated device collection settings.
	c.ServerSettings.EnableAnalytics = true
	data, err := json.Marshal(c)
	require.NoError(t, err)
	var persisted AppConfig
	require.NoError(t, json.Unmarshal(data, &persisted))
	require.False(t, persisted.ServerSettings.EnableAnalytics)
	require.Equal(t, c.Features, persisted.Features)
	require.True(t, c.ServerSettings.EnableAnalytics, "marshaling must not mutate its input")
}
