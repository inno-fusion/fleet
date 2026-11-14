package config

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestIndependentMDMEnvironment(t *testing.T) {
	for key, value := range map[string]string{
		"FLEET_MDM_APPLE_MDM_VENDOR_CERT": "/run/secrets/vendor.pem",
		"FLEET_MDM_APPLE_MDM_VENDOR_KEY": "/run/secrets/vendor.key",
		"FLEET_MDM_APPLE_MDM_VENDOR_CHAIN": "/run/secrets/apple-chain.pem",
		"FLEET_MDM_ANDROID_GOOGLE_SERVICE_CREDENTIALS": "/run/secrets/google.json",
		"FLEET_MDM_ANDROID_PROXY_URL": "https://mdm.example.test/api/android/",
	} {
		t.Setenv(key, value)
	}
	cmd := &cobra.Command{}
	cmd.PersistentFlags().StringP("config", "c", "", "Config file")
	cfg := NewManager(cmd).LoadConfig()
	require.Equal(t, "/run/secrets/vendor.pem", cfg.MDM.AppleMDMVendorCert)
	require.Equal(t, "/run/secrets/vendor.key", cfg.MDM.AppleMDMVendorKey)
	require.Equal(t, "/run/secrets/apple-chain.pem", cfg.MDM.AppleMDMVendorChain)
	require.Equal(t, "/run/secrets/google.json", cfg.MDM.AndroidGoogleServiceCredentials)
	require.Equal(t, "https://mdm.example.test/api/android/", cfg.MDM.AndroidProxyURL)
}
