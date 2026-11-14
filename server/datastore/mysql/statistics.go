package mysql

import (
	"context"
	"time"

	"github.com/fleetdm/fleet/v4/server/config"
	"github.com/fleetdm/fleet/v4/server/contexts/ctxerr"
	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/jmoiron/sqlx"
)

func (ds *Datastore) ShouldSendStatistics(_ context.Context, _ time.Duration, _ config.FleetConfig) (fleet.StatisticsPayload, bool, error) {
	// Do not construct vendor analytics or create an anonymous identifier, even
	// when invoked directly instead of through the disabled upload schedule.
	return fleet.StatisticsPayload{}, false, nil
}

func (ds *Datastore) RecordStatisticsSent(ctx context.Context) error {
	_, err := ds.writer(ctx).ExecContext(ctx, `UPDATE statistics SET updated_at = CURRENT_TIMESTAMP LIMIT 1`)
	return ctxerr.Wrap(ctx, err, "update statistics")
}

func (ds *Datastore) CleanupStatistics(ctx context.Context) error {
	// reset weekly count of policy violation days
	if err := ds.InitializePolicyViolationDays(ctx); err != nil {
		return err
	}
	return nil
}

func (ds *Datastore) GetTableRowCounts(ctx context.Context) (map[string]uint, error) {
	return ds.getTableRowCountsViaInformationSchema(ctx)
}

func (ds *Datastore) getTableRowCountsViaInformationSchema(ctx context.Context) (map[string]uint, error) {
	var results []struct {
		Table string `db:"TABLE_NAME"`
		Rows  uint   `db:"table_rows"`
	}

	if err := sqlx.SelectContext(
		ctx,
		ds.reader(ctx),
		&results,
		"SELECT table_name, COALESCE(table_rows, 0) table_rows FROM information_schema.tables WHERE table_schema = (SELECT DATABASE())",
	); err != nil {
		return nil, err
	}

	byName := make(map[string]uint)
	for _, row := range results {
		byName[row.Table] = row.Rows
	}
	return byName, nil
}

// fleetMaintainedAppsInUseDB returns the Fleet-maintained apps in use grouped by platform
// (darwin for macOS, windows for Windows), along with whether a patch policy covers each app
// and whether that patch policy carries a software automation.
func fleetMaintainedAppsInUseDB(ctx context.Context, db sqlx.QueryerContext) (macOSApps []fleet.FleetMaintainedAppUsage, windowsApps []fleet.FleetMaintainedAppUsage, err error) {
	// One slug backs several software_installers rows (one per team, plus cached versions), so
	// aggregate per slug: without the GROUP BY, rows whose booleans differ by team would emit
	// the same slug more than once.
	const query = `
		SELECT
			fma.slug AS name,
			fma.platform,
			MAX(p.id IS NOT NULL) AS patch_policy,
			MAX(` + policiesSoftwareAutomationClause + `) AS software_automation
		FROM software_installers si
		INNER JOIN fleet_maintained_apps fma ON si.fleet_maintained_app_id = fma.id
		LEFT JOIN policies p
			ON p.patch_software_title_id = si.title_id
			AND p.type = ?
			AND si.global_or_team_id = p.team_id
		WHERE si.fleet_maintained_app_id IS NOT NULL AND fma.platform IN ('darwin', 'windows')
		GROUP BY fma.slug, fma.platform
		ORDER BY fma.platform, fma.slug
	`

	type appResult struct {
		fleet.FleetMaintainedAppUsage
		Platform string `db:"platform"`
	}

	var results []appResult
	if err := sqlx.SelectContext(ctx, db, &results, query, fleet.PolicyTypePatch); err != nil {
		return nil, nil, ctxerr.Wrap(ctx, err, "selecting fleet maintained apps in use")
	}

	// Initialize as empty slices (not nil) so they serialize as [] instead of null
	macOSApps = make([]fleet.FleetMaintainedAppUsage, 0)
	windowsApps = make([]fleet.FleetMaintainedAppUsage, 0)

	for _, app := range results {
		switch app.Platform {
		case "darwin":
			macOSApps = append(macOSApps, app.FleetMaintainedAppUsage)
		case "windows":
			windowsApps = append(windowsApps, app.FleetMaintainedAppUsage)
		}
	}

	return macOSApps, windowsApps, nil
}
