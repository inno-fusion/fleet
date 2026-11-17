package mysql

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fleetdm/fleet/v4/server/config"
	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/fleetdm/fleet/v4/server/test"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatistics(t *testing.T) {
	ds := CreateMySQLDS(t)

	cases := []struct {
		name string
		fn   func(t *testing.T, ds *Datastore)
	}{
		{"FleetMaintainedAppsInUse", testFleetMaintainedAppsInUse},
		{"PoliciesAutomationEnabledSoftware", testPoliciesAutomationEnabledSoftware},
		{"FleetMDMEnrolled", testStatisticsFleetMDMEnrolled},
		{"AIGeneratedProfileCounts", testStatisticsAIGeneratedProfileCounts},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer TruncateTables(t, ds)
			c.fn(t, ds)
		})
	}
}

func testFleetMaintainedAppsInUse(t *testing.T, ds *Datastore) {
	ctx := context.Background()

	fmaUsage := func(name string, patchPolicy, softwareAutomation bool) fleet.FleetMaintainedAppUsage {
		return fleet.FleetMaintainedAppUsage{Name: name, PatchPolicy: patchPolicy, SoftwareAutomation: softwareAutomation}
	}

	expectMaintainedApps := func(mac, win []fleet.FleetMaintainedAppUsage) {
		t.Helper()
		gotMac, gotWin, err := fleetMaintainedAppsInUseDB(ctx, ds.reader(ctx))
		require.NoError(t, err)
		require.Equal(t, mac, gotMac)
		require.Equal(t, win, gotWin)
	}

	noApps := []fleet.FleetMaintainedAppUsage{}
	macNoPolicies := []fleet.FleetMaintainedAppUsage{fmaUsage("slack/darwin", false, false), fmaUsage("zoom/darwin", false, false)}
	macWithPatchPolicies := []fleet.FleetMaintainedAppUsage{fmaUsage("slack/darwin", true, true), fmaUsage("zoom/darwin", true, false)}
	winNoPolicies := []fleet.FleetMaintainedAppUsage{fmaUsage("microsoft-teams/windows", false, false), fmaUsage("zoom/windows", false, false)}

	// No fleet-maintained apps - should return empty slices (not nil)
	macOSApps, windowsApps, err := fleetMaintainedAppsInUseDB(ctx, ds.reader(ctx))
	require.NoError(t, err)
	assert.NotNil(t, macOSApps)
	assert.NotNil(t, windowsApps)
	assert.Empty(t, macOSApps)
	assert.Empty(t, windowsApps)

	appDarwin1, err := ds.UpsertMaintainedApp(ctx, &fleet.MaintainedApp{
		Name:             "Zoom",
		Slug:             "zoom/darwin",
		Platform:         "darwin",
		UniqueIdentifier: "us.zoom.xos",
	})
	require.NoError(t, err)
	appDarwin2, err := ds.UpsertMaintainedApp(ctx, &fleet.MaintainedApp{
		Name:             "Slack",
		Slug:             "slack/darwin",
		Platform:         "darwin",
		UniqueIdentifier: "com.tinyspeck.slackmacgap",
	})
	require.NoError(t, err)
	appWindows1, err := ds.UpsertMaintainedApp(ctx, &fleet.MaintainedApp{
		Name:             "Microsoft Teams",
		Slug:             "microsoft-teams/windows",
		Platform:         "windows",
		UniqueIdentifier: "msteams",
	})
	require.NoError(t, err)
	appWindows2, err := ds.UpsertMaintainedApp(ctx, &fleet.MaintainedApp{
		Name:             "Zoom",
		Slug:             "zoom/windows",
		Platform:         "windows",
		UniqueIdentifier: "zoom-windows",
	})
	require.NoError(t, err)
	appLinux, err := ds.UpsertMaintainedApp(ctx, &fleet.MaintainedApp{
		Name:             "Linux App",
		Slug:             "linux-app/linux",
		Platform:         "linux",
		UniqueIdentifier: "linux.app",
	})
	require.NoError(t, err)

	// Apps exist but no software installers - should still return empty
	expectMaintainedApps(noApps, noApps)

	// Create script content (required for software installers)
	var installScriptID, uninstallScriptID int64
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		result, err := q.ExecContext(ctx, `INSERT INTO script_contents (md5_checksum, contents) VALUES (UNHEX(?), ?)`,
			"d41d8cd98f00b204e9800998ecf8427e", "echo 'install'")
		if err != nil {
			return err
		}
		installScriptID, _ = result.LastInsertId()
		return nil
	})
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		result, err := q.ExecContext(ctx, `INSERT INTO script_contents (md5_checksum, contents) VALUES (UNHEX(?), ?)`,
			"e10adc3949ba59abbe56e057f20f883e", "echo 'uninstall'")
		if err != nil {
			return err
		}
		uninstallScriptID, _ = result.LastInsertId()
		return nil
	})

	// Create software installers that reference fleet-maintained apps
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		_, err := q.ExecContext(ctx, `
			INSERT INTO software_installers (
				team_id, global_or_team_id, filename, version, platform,
				install_script_content_id, uninstall_script_content_id,
				storage_id, package_ids, fleet_maintained_app_id, patch_query
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, nil, 0, "zoom.pkg", "1.0", "darwin", installScriptID, uninstallScriptID, "storage1", "[]", appDarwin1.ID, "")
		return err
	})
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		_, err := q.ExecContext(ctx, `
			INSERT INTO software_installers (
				team_id, global_or_team_id, filename, version, platform,
				install_script_content_id, uninstall_script_content_id,
				storage_id, package_ids, fleet_maintained_app_id, patch_query
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, nil, 0, "slack.pkg", "1.0", "darwin", installScriptID, uninstallScriptID, "storage2", "[]", appDarwin2.ID, "")
		return err
	})
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		_, err := q.ExecContext(ctx, `
			INSERT INTO software_installers (
				team_id, global_or_team_id, filename, version, platform,
				install_script_content_id, uninstall_script_content_id,
				storage_id, package_ids, fleet_maintained_app_id, patch_query
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, nil, 0, "teams.exe", "1.0", "windows", installScriptID, uninstallScriptID, "storage3", "[]", appWindows1.ID, "")
		return err
	})
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		_, err := q.ExecContext(ctx, `
			INSERT INTO software_installers (
				team_id, global_or_team_id, filename, version, platform,
				install_script_content_id, uninstall_script_content_id,
				storage_id, package_ids, fleet_maintained_app_id, patch_query
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, nil, 0, "zoom.exe", "1.0", "windows", installScriptID, uninstallScriptID, "storage4", "[]", appWindows2.ID, "")
		return err
	})
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		_, err := q.ExecContext(ctx, `
			INSERT INTO software_installers (
				team_id, global_or_team_id, filename, version, platform,
				install_script_content_id, uninstall_script_content_id,
				storage_id, package_ids, fleet_maintained_app_id, patch_query
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, nil, 0, "linux.deb", "1.0", "linux", installScriptID, uninstallScriptID, "storage5", "[]", appLinux.ID, "")
		return err
	})

	// Apps with installers - should return correct apps grouped by platform
	expectMaintainedApps(macNoPolicies, winNoPolicies)

	// Create duplicate installers for same app
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		_, err := q.ExecContext(ctx, `
			INSERT INTO software_installers (
				team_id, global_or_team_id, filename, version, platform,
				install_script_content_id, uninstall_script_content_id,
				storage_id, package_ids, fleet_maintained_app_id, patch_query
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, nil, 0, "zoom-v2.pkg", "2.0", "darwin", installScriptID, uninstallScriptID, "storage6", "[]", appDarwin1.ID, "")
		return err
	})
	expectMaintainedApps(macNoPolicies, winNoPolicies)

	// Create an installer with NULL fleet_maintained_app_id (should be ignored)
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		_, err := q.ExecContext(ctx, `
			INSERT INTO software_installers (
				team_id, global_or_team_id, filename, version, platform,
				install_script_content_id, uninstall_script_content_id,
				storage_id, package_ids, fleet_maintained_app_id, patch_query
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, nil, 0, "custom.pkg", "1.0", "darwin", installScriptID, uninstallScriptID, "storage7", "[]", nil, "")
		return err
	})

	// Should return the same results (NULL fleet_maintained_app_id is filtered out)
	expectMaintainedApps(macNoPolicies, winNoPolicies)

	// Give the darwin apps software titles so patch policies can point at them. Both zoom
	// installers (the 1.0 and the cached 2.0) share one title, as real FMA versions do.
	var zoomTitleID, slackTitleID, slackInstallerID uint
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		res, err := q.ExecContext(ctx, `INSERT INTO software_titles (name, source, bundle_identifier) VALUES ('Zoom', 'apps', 'us.zoom.xos')`)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		zoomTitleID = uint(id) //nolint:gosec // test fixture

		res, err = q.ExecContext(ctx, `INSERT INTO software_titles (name, source, bundle_identifier) VALUES ('Slack', 'apps', 'com.tinyspeck.slackmacgap')`)
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		slackTitleID = uint(id) //nolint:gosec // test fixture

		if _, err := q.ExecContext(ctx, `UPDATE software_installers SET title_id = ? WHERE fleet_maintained_app_id = ?`, zoomTitleID, appDarwin1.ID); err != nil {
			return err
		}
		if _, err := q.ExecContext(ctx, `UPDATE software_installers SET title_id = ? WHERE fleet_maintained_app_id = ?`, slackTitleID, appDarwin2.ID); err != nil {
			return err
		}
		return sqlx.GetContext(ctx, q, &slackInstallerID, `SELECT id FROM software_installers WHERE fleet_maintained_app_id = ?`, appDarwin2.ID)
	})

	// A dynamic policy that installs the app is not a patch policy, and must leave both
	// flags alone. Asserted before any patch policy exists, so the per-slug OR can't mask it.
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		_, err := q.ExecContext(ctx, `
			INSERT INTO policies (name, query, description, checksum, team_id, software_installer_id)
			VALUES ('install slack', 'SELECT 1', '', UNHEX(MD5('install slack')), ?, ?)`, fleet.PolicyNoTeamID, slackInstallerID)
		return err
	})

	expectMaintainedApps(macNoPolicies, winNoPolicies)

	// A patch policy with no software automation, and one with a software automation.
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		_, err := q.ExecContext(ctx, `
			INSERT INTO policies (name, query, description, checksum, type, team_id, patch_software_title_id)
			VALUES ('patch zoom', 'SELECT 1', '', UNHEX(MD5('patch zoom')), 'patch', ?, ?)`, fleet.PolicyNoTeamID, zoomTitleID)
		return err
	})
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		_, err := q.ExecContext(ctx, `
			INSERT INTO policies (name, query, description, checksum, type, team_id, patch_software_title_id, software_installer_id)
			VALUES ('patch slack', 'SELECT 1', '', UNHEX(MD5('patch slack')), 'patch', ?, ?, ?)`, fleet.PolicyNoTeamID, slackTitleID, slackInstallerID)
		return err
	})

	expectMaintainedApps(macWithPatchPolicies, winNoPolicies)

	// The same app installed on another team, with no patch policy of its own. The slug must
	// still appear exactly once, with the booleans OR'd across teams -- this is what the
	// GROUP BY buys us, and what plain DISTINCT would get wrong.
	tm, err := ds.NewTeam(ctx, &fleet.Team{Name: "fma stats team"})
	require.NoError(t, err)
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		_, err := q.ExecContext(ctx, `
			INSERT INTO software_installers (
				team_id, global_or_team_id, title_id, filename, version, platform,
				install_script_content_id, uninstall_script_content_id,
				storage_id, package_ids, fleet_maintained_app_id, patch_query
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, tm.ID, tm.ID, zoomTitleID, "zoom.pkg", "1.0", "darwin", installScriptID, uninstallScriptID, "storage8", "[]", appDarwin1.ID, "")
		return err
	})

	expectMaintainedApps(macWithPatchPolicies, winNoPolicies)

	// A patch policy scoped to a team must not mark a different team's installer. Use
	// microsoft-teams, which has no patch policy of its own -- asserting this against an app
	// that already reports true would be masked by the per-slug OR.
	var teamsTitleID uint
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		res, err := q.ExecContext(ctx, `INSERT INTO software_titles (name, source, bundle_identifier) VALUES ('Microsoft Teams', 'programs', 'msteams.exe')`)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		teamsTitleID = uint(id) //nolint:gosec // test fixture
		_, err = q.ExecContext(ctx, `UPDATE software_installers SET title_id = ? WHERE fleet_maintained_app_id = ?`, teamsTitleID, appWindows1.ID)
		return err
	})
	// microsoft-teams is installed on "no team" only; scope the patch policy to the team.
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		_, err := q.ExecContext(ctx, `
			INSERT INTO policies (name, query, description, checksum, type, team_id, patch_software_title_id)
			VALUES ('patch teams', 'SELECT 1', '', UNHEX(MD5('patch teams')), 'patch', ?, ?)`, tm.ID, teamsTitleID)
		return err
	})
	expectMaintainedApps(macWithPatchPolicies, winNoPolicies)
}

func testPoliciesAutomationEnabledSoftware(t *testing.T, ds *Datastore) {
	ctx := context.Background()

	count, err := amountPoliciesAutomationEnabledSoftwareDB(ctx, ds.reader(ctx))
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Set up an installer and a VPP app so the automation columns can be populated.
	var installScriptID, uninstallScriptID int64
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		res, err := q.ExecContext(ctx, `INSERT INTO script_contents (md5_checksum, contents) VALUES (UNHEX(?), ?)`,
			"d41d8cd98f00b204e9800998ecf8427e", "echo 'install'")
		if err != nil {
			return err
		}
		installScriptID, _ = res.LastInsertId()
		res, err = q.ExecContext(ctx, `INSERT INTO script_contents (md5_checksum, contents) VALUES (UNHEX(?), ?)`,
			"e10adc3949ba59abbe56e057f20f883e", "echo 'uninstall'")
		if err != nil {
			return err
		}
		uninstallScriptID, _ = res.LastInsertId()
		return nil
	})

	var installerID, titleID uint
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		res, err := q.ExecContext(ctx, `INSERT INTO software_titles (name, source, bundle_identifier) VALUES ('Ruby', 'apps', 'org.ruby.lang')`)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		titleID = uint(id) //nolint:gosec // test fixture

		res, err = q.ExecContext(ctx, `
			INSERT INTO software_installers (
				team_id, global_or_team_id, title_id, filename, version, platform,
				install_script_content_id, uninstall_script_content_id,
				storage_id, package_ids, patch_query
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, nil, 0, titleID, "ruby.pkg", "1.0", "darwin", installScriptID, uninstallScriptID, "storage-ruby", "[]", "")
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		installerID = uint(id) //nolint:gosec // test fixture
		return nil
	})

	var vppAppsTeamsID uint
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		if _, err := q.ExecContext(ctx, `INSERT INTO vpp_apps (adam_id, platform, name) VALUES ('12345', 'darwin', 'Numbers')`); err != nil {
			return err
		}
		res, err := q.ExecContext(ctx, `INSERT INTO vpp_apps_teams (adam_id, platform, global_or_team_id, name) VALUES ('12345', 'darwin', 0, 'Default version')`)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		vppAppsTeamsID = uint(id) //nolint:gosec // test fixture
		return nil
	})

	insertPolicy := func(name, columns, values string, args ...any) {
		t.Helper()
		ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
			_, err := q.ExecContext(ctx, fmt.Sprintf(`
				INSERT INTO policies (name, query, description, checksum%s)
				VALUES (?, 'SELECT 1', '', UNHEX(MD5(?))%s)`, columns, values),
				append([]any{name, name}, args...)...)
			return err
		})
	}

	// A dynamic policy with no automation columns set is not a software automation.
	insertPolicy("plain", "", "")
	count, err = amountPoliciesAutomationEnabledSoftwareDB(ctx, ds.reader(ctx))
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// A dynamic policy that installs a package counts.
	insertPolicy("install ruby", ", software_installer_id", ", ?", installerID)
	count, err = amountPoliciesAutomationEnabledSoftwareDB(ctx, ds.reader(ctx))
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// So does one that installs a VPP app.
	insertPolicy("install vpp", ", vpp_apps_teams_id", ", ?", vppAppsTeamsID)
	count, err = amountPoliciesAutomationEnabledSoftwareDB(ctx, ds.reader(ctx))
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// A patch policy with no install automation does not count: automation_type=software no
	// longer matches on p.type, automation_type=patch covers those instead.
	insertPolicy("patch ruby", ", type, patch_software_title_id", ", 'patch', ?", titleID)
	count, err = amountPoliciesAutomationEnabledSoftwareDB(ctx, ds.reader(ctx))
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// A patch policy that also installs a package does count.
	var pythonTitleID uint
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		res, err := q.ExecContext(ctx, `INSERT INTO software_titles (name, source, bundle_identifier) VALUES ('Python', 'apps', 'org.python.lang')`)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		pythonTitleID = uint(id) //nolint:gosec // test fixture
		return nil
	})
	insertPolicy("patch python", ", type, patch_software_title_id, software_installer_id", ", 'patch', ?, ?", pythonTitleID, installerID)
	count, err = amountPoliciesAutomationEnabledSoftwareDB(ctx, ds.reader(ctx))
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Script and calendar automations are separate filters and must not count.
	var scriptID uint
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		res, err := q.ExecContext(ctx, `INSERT INTO scripts (name, global_or_team_id, script_content_id) VALUES ('run.sh', 0, ?)`, installScriptID)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		scriptID = uint(id) //nolint:gosec // test fixture
		return nil
	})
	insertPolicy("run script", ", script_id", ", ?", scriptID)
	insertPolicy("calendar", ", calendar_events_enabled", ", 1")
	count, err = amountPoliciesAutomationEnabledSoftwareDB(ctx, ds.reader(ctx))
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func testStatisticsFleetMDMEnrolled(t *testing.T, ds *Datastore) {
	ctx := t.Context()

	// With no hosts, both counts are zero (not null/missing).
	macOS, windows, err := numHostsFleetMDMEnrolledDB(ctx, ds.reader(ctx))
	require.NoError(t, err)
	assert.Zero(t, macOS)
	assert.Zero(t, windows)

	// Each host exercises one branch of the query; only enrolled, non-server, Fleet-MDM darwin/windows hosts are counted.
	cases := []struct {
		name             string
		platform         string
		isServer         bool
		enrolled         bool
		installedFromDep bool
		mdmName          string // "" means no MDM data at all
	}{
		{"macOS Fleet MDM", "darwin", false, true, false, fleet.WellKnownMDMFleet},        // counted (macOS)
		{"windows Fleet MDM", "windows", false, true, false, fleet.WellKnownMDMFleet},     // counted (Windows)
		{"macOS third-party MDM", "darwin", false, true, false, fleet.WellKnownMDMIntune}, // excluded: not Fleet
		// ABM pending: DEP-assigned (installed_from_dep=1) but not yet enrolled (enrolled=0).
		{"macOS ABM pending", "darwin", false, false, true, fleet.WellKnownMDMFleet},   // excluded: not enrolled
		{"windows server host", "windows", true, true, false, fleet.WellKnownMDMFleet}, // excluded: is_server
		{"iOS Fleet MDM", "ios", false, true, false, fleet.WellKnownMDMFleet},          // excluded: not macOS/Windows
		{"macOS no MDM", "darwin", false, false, false, ""},                            // excluded: no MDM data
	}
	for i, c := range cases {
		key := fmt.Sprintf("mdm-stats-%d", i)
		h := test.NewHost(t, ds, key, "", key, key, time.Now(), test.WithPlatform(c.platform))
		if c.mdmName != "" {
			require.NoError(t, ds.SetOrUpdateMDMData(ctx, h.ID, c.isServer, c.enrolled, "https://fleet.example.com", c.installedFromDep, c.mdmName, "", fleet.PersonalEnrollmentTypeNone), c.name)
		}
	}

	macOS, windows, err = numHostsFleetMDMEnrolledDB(ctx, ds.reader(ctx))
	require.NoError(t, err)
	assert.Equal(t, 1, macOS)
	assert.Equal(t, 1, windows)
}

func testStatisticsAIGeneratedProfileCounts(t *testing.T, ds *Datastore) {
	ctx := t.Context()

	// Generating profiles must not collect analytics: valid platforms succeed but store nothing.
	const perPlatform = 20
	var wg sync.WaitGroup
	errs := make(chan error, 2*perPlatform)
	for range perPlatform {
		for _, platform := range []string{fleet.AutofillConfigProfilePlatformApple, fleet.AutofillConfigProfilePlatformWindows} {
			wg.Go(func() {
				errs <- ds.IncrementAIGeneratedProfileCount(ctx, platform)
			})
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Error(t, ds.IncrementAIGeneratedProfileCount(ctx, "android"))

	var rows int
	ExecAdhocSQL(t, ds, func(q sqlx.ExtContext) error {
		return sqlx.GetContext(ctx, q, &rows, `SELECT COUNT(*) FROM aggregated_stats WHERE type = ?`, aggregatedStatsTypeAIGeneratedProfiles)
	})
	assert.Zero(t, rows)

	_, shouldSend, err := ds.ShouldSendStatistics(ctx, time.Millisecond, config.TestConfig())
	require.NoError(t, err)
	assert.False(t, shouldSend)
}
