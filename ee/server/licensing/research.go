package licensing

import (
	"fmt"
	"os"
	"strconv"

	"github.com/fleetdm/fleet/v4/server/fleet"
)

// LocalResearchEnv opts this process into local development/testing entitlements.
// Leave it unset for normal signed-license behavior. This does not grant a Fleet
// subscription or relax authentication, authorization, or integration setup.
const LocalResearchEnv = "FLEET_LICENSE_LOCAL_RESEARCH"

func localResearchEnabled() (bool, error) {
	value := os.Getenv(LocalResearchEnv)
	if value == "" {
		return false, nil
	}
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", LocalResearchEnv)
	}
	return enabled, nil
}

func newLocalResearchLicense() *fleet.LicenseInfo {
	// Zero devices leaves the licensed-host quota unset; zero expiration is
	// deliberate and IsExpired handles this distinct, non-trial tier.
	return &fleet.LicenseInfo{
		Tier:                  fleet.TierResearch,
		Organization:          "Local development and testing",
		Note:                  "Development and testing only; not a Fleet subscription. No authorization for Fleet-hosted remote services.",
		AllowDisableTelemetry: true,
	}
}
