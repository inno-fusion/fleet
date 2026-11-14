package apple_mdm

import (
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
)

var pssoAppIDs atomic.Pointer[[]string]

var pssoAppIDPattern = regexp.MustCompile(`^[A-Z0-9]{10}\.[A-Za-z0-9][A-Za-z0-9.-]*$`)

// SetPSSOAppIDs sets the comma-separated <TeamID>.<BundleID> app IDs that replace
// Fleet's built-in Platform SSO identities, for a Fleet Desktop app and PSSO
// extension signed under another Apple Developer team. Called once at server
// startup from config.MDM.ApplePSSOAppIDs.
func SetPSSOAppIDs(raw string) error {
	var ids []string
	for id := range strings.SplitSeq(raw, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !pssoAppIDPattern.MatchString(id) {
			return fmt.Errorf("invalid Platform SSO app ID %q: want <TeamID>.<BundleID>", id)
		}
		ids = append(ids, id)
	}
	pssoAppIDs.Store(&ids)
	return nil
}

// PSSOAppIDs returns the configured app IDs, or nil when Fleet's built-in ones apply.
func PSSOAppIDs() []string {
	if ids := pssoAppIDs.Load(); ids != nil && len(*ids) > 0 {
		return *ids
	}
	return nil
}

// PSSOExtensionIdentifierAllowed reports whether a Platform SSO payload with this
// ExtensionIdentifier may carry Fleet's device registration token.
func PSSOExtensionIdentifierAllowed(extensionID string) bool {
	if strings.HasPrefix(extensionID, "com.fleetdm") {
		return true
	}
	for _, id := range PSSOAppIDs() {
		if _, bundleID, _ := strings.Cut(id, "."); bundleID == extensionID {
			return true
		}
	}
	return false
}

// SetPSSOAppIDsForTest sets the app IDs for the duration of a test.
func SetPSSOAppIDsForTest(t tbCleanup, raw string) error {
	prev := pssoAppIDs.Load()
	t.Cleanup(func() { pssoAppIDs.Store(prev) })
	return SetPSSOAppIDs(raw)
}
