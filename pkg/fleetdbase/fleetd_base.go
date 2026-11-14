// pacakge fleetdbase contains functions to interact with downloads.fleetdm.com
package fleetdbase

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/fleetdm/fleet/v4/server/dev_mode"
)

type Metadata struct {
	MSIURL           string `json:"fleetd_base_msi_url"`
	MSISha256        string `json:"fleetd_base_msi_sha256"`
	PKGURL           string `json:"fleetd_base_pkg_url"`
	PKGSha256        string `json:"fleetd_base_pkg_sha256"`
	ManifestPlistURL string `json:"fleetd_base_manifest_plist_url"`
	Version          string `json:"version"`
}

var configuredBaseURL atomic.Pointer[string]

// SetBaseURL points fleetd-base downloads at an operator-hosted mirror of
// download.fleetdm.com's /stable layout. Called once at server startup from
// config.MDM.FleetdBaseURL; empty keeps the default.
func SetBaseURL(raw string) error {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw != "" {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("fleetd base URL must be an absolute https URL, got %q", raw)
		}
	}
	configuredBaseURL.Store(&raw)
	return nil
}

func getBaseURL() string {
	devURL := dev_mode.Env("FLEET_DEV_DOWNLOAD_FLEETDM_URL")
	if devURL != "" {
		return devURL
	}
	if u := configuredBaseURL.Load(); u != nil && *u != "" {
		return *u
	}
	return "https://download.fleetdm.com"
}

func GetMetadata() (*Metadata, error) {
	baseURL := getBaseURL()
	rawURL := fmt.Sprintf("%s/stable/meta.json", baseURL)

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	resp, err := http.Get(parsedURL.String())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var meta Metadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &meta, nil
}

func GetPKGManifestURL() string {
	baseURL := getBaseURL()
	return fmt.Sprintf("%s/stable/fleetd-base-manifest.plist", baseURL)
}
