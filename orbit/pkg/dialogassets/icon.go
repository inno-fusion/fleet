// Package dialogassets resolves presentation assets without vendor fallbacks.
package dialogassets

import "github.com/fleetdm/fleet/v4/orbit/pkg/swiftdialog"

// Icon returns the operator-selected logo and its size. Without a logo, hide
// the icon explicitly so swiftDialog does not substitute its own default.
func Icon(logoURL string) (string, uint, error) {
	if logoURL == "" {
		return "none", swiftdialog.DefaultIconSize, nil
	}

	size, err := swiftdialog.GetIconSize(logoURL)
	if err != nil {
		return logoURL, swiftdialog.DefaultIconSize, err
	}
	return logoURL, size, nil
}
