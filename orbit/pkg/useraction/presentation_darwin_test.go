//go:build darwin

package useraction

import (
	"strings"
	"testing"

	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/stretchr/testify/require"
)

type presentationDialog struct {
	flags []string
}

func (d *presentationDialog) CanRun() bool { return true }
func (d *presentationDialog) Exit()        {}
func (d *presentationDialog) render(flags ...string) (chan swiftDialogExitCode, chan error) {
	d.flags = flags
	return make(chan swiftDialogExitCode, 1), make(chan error, 1)
}

func presentationFlag(flags []string, name string) string {
	for i, flag := range flags {
		if flag == name && i+1 < len(flags) {
			return flags[i+1]
		}
	}
	return ""
}

func TestMigrationPresentationHasNoRemoteResources(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version int
		manual  bool
		text    string
	}{
		{"pre-Sonoma", 13, false, "Notification Center"},
		{"pre-Sonoma manual", 13, true, "Notification Center"},
		{"manual", 14, true, "My device"},
		{"ADE", 14, false, "Remote Management"},
		{"unknown macOS version", 0, false, "Remote Management"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &presentationDialog{}
			m := &swiftDialogMDMMigrator{
				baseDialog: d,
				props: MDMMigratorProps{OrgInfo: fleet.DesktopOrgInfo{
					ContactURL: "https://help.example.com",
				}},
			}
			message, flags, err := m.getMessageAndFlags(tc.version, tc.manual)
			require.NoError(t, err)
			require.Contains(t, message.String(), tc.text)
			require.Contains(t, message.String(), "**Start**")
			require.NotContains(t, message.String(), "![")
			require.NotContains(t, message.String(), "https://")
			m.render(message.String(), flags...)
			require.Equal(t, "none", presentationFlag(d.flags, "--icon"))
			require.Equal(t, "Start", presentationFlag(d.flags, "--button1text"))
			require.Equal(t, "Later", presentationFlag(d.flags, "--button2text"))
			require.Equal(t, "https://help.example.com", presentationFlag(d.flags, "--infobuttonaction"))
			require.Contains(t, d.flags, "--blurscreen")
			require.Contains(t, d.flags, "--ontop")

			m.props.DisableTakeover = true
			_, flags, err = m.getMessageAndFlags(tc.version, tc.manual)
			require.NoError(t, err)
			require.NotContains(t, flags, "--blurscreen")
			require.NotContains(t, flags, "--ontop")
		})
	}
}

func TestMigrationStatusDialogsHaveNoRemoteResources(t *testing.T) {
	d := &presentationDialog{}
	m := &swiftDialogMDMMigrator{baseDialog: d}
	m.renderLoadingSpinner()
	require.Equal(t, "none", presentationFlag(d.flags, "--icon"))
	require.Contains(t, presentationFlag(d.flags, "--message"), "90 seconds")
	require.Contains(t, d.flags, "--button1disabled")
	require.NotContains(t, strings.Join(d.flags, " "), "https://")
	require.NotContains(t, strings.Join(d.flags, " "), "![")

	m.renderError()
	require.Equal(t, "Close", presentationFlag(d.flags, "--button1text"))
	require.NotContains(t, strings.Join(d.flags, " "), "https://")
	m.props.OrgInfo.ContactURL = "https://help.example.com"
	m.renderError()
	require.Equal(t, "Contact IT", presentationFlag(d.flags, "--button1text"))
	require.Equal(t, m.props.OrgInfo.ContactURL, presentationFlag(d.flags, "--button1action"))

	offline := &swiftDialogMDMMigrationOffline{}
	flags, err := offline.getFlags()
	require.NoError(t, err)
	require.Equal(t, "none", presentationFlag(flags, "--icon"))
	require.NotContains(t, strings.Join(flags, " "), "https://")
}

func TestMigrationPreservesOperatorLogoSelection(t *testing.T) {
	for _, tc := range []struct {
		name string
		info fleet.DesktopOrgInfo
		dark bool
		want string
	}{
		{name: "no light logo"},
		{name: "no dark logo", dark: true},
		{name: "light logo", info: fleet.DesktopOrgInfo{OrgLogoURLLightMode: "https://org.example.com/light.png"}, want: "https://org.example.com/light.png"},
		{name: "dark logo", dark: true, info: fleet.DesktopOrgInfo{OrgLogoURLDarkMode: "https://org.example.com/dark.png"}, want: "https://org.example.com/dark.png"},
		{name: "legacy light logo", info: fleet.DesktopOrgInfo{OrgLogoURLLightBackground: "https://org.example.com/light.png"}, want: "https://org.example.com/light.png"},
		{name: "legacy dark logo", dark: true, info: fleet.DesktopOrgInfo{OrgLogoURL: "https://org.example.com/dark.png"}, want: "https://org.example.com/dark.png"},
		{name: "modern takes precedence", dark: true, info: fleet.DesktopOrgInfo{OrgLogoURLDarkMode: "https://org.example.com/current.png", OrgLogoURL: "https://org.example.com/legacy.png"}, want: "https://org.example.com/current.png"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &swiftDialogMDMMigrator{props: MDMMigratorProps{OrgInfo: tc.info}}
			require.Equal(t, tc.want, m.iconURL(tc.dark))
		})
	}
}
