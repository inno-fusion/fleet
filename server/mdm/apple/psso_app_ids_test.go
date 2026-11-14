package apple_mdm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPSSOAppIDs(t *testing.T) {
	require.NoError(t, SetPSSOAppIDsForTest(t, ""))
	require.Nil(t, PSSOAppIDs())
	require.True(t, PSSOExtensionIdentifierAllowed("com.fleetdm.fleet-desktop.pssoextension"))
	require.False(t, PSSOExtensionIdentifierAllowed("com.example.fleet-desktop.pssoextension"))

	require.NoError(t, SetPSSOAppIDs(" CQ629MZD6T.com.example.fleet-desktop, CQ629MZD6T.com.example.fleet-desktop.pssoextension ,"))
	require.Equal(t, []string{"CQ629MZD6T.com.example.fleet-desktop", "CQ629MZD6T.com.example.fleet-desktop.pssoextension"}, PSSOAppIDs())
	require.True(t, PSSOExtensionIdentifierAllowed("com.example.fleet-desktop.pssoextension"))
	require.True(t, PSSOExtensionIdentifierAllowed("com.fleetdm.fleet-desktop.pssoextension"))
	require.False(t, PSSOExtensionIdentifierAllowed("com.example.other"))
	require.False(t, PSSOExtensionIdentifierAllowed("CQ629MZD6T.com.example.fleet-desktop.pssoextension"))

	for _, bad := range []string{"com.example.app", "cq629mzd6t.com.example.app", "CQ629MZD6T.", "CQ629MZD6T.com.example app", "SHORT.com.example.app"} {
		require.Error(t, SetPSSOAppIDs(bad), bad)
	}
}
