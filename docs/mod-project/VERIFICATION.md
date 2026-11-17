# Verification and maintenance

## Scope

Baseline: Fleet `fleet-v4.95.0`, commit
`5790b6cafd149ad082d5a33a65e6ea016212560d`. This is an opt-in local
research entitlement, selective product-analytics removal, and independently
configurable platform integration patch. It is not a signed Fleet subscription
or a replacement for Apple/Google approval and credentials.

The central backend entitlement and frontend helper include research. Ordinary
signed-license loading remains unchanged when research is off. Invalid research
booleans and simultaneous configured keys fail rather than silently selecting a
tier. Research has no subscription expiry and no licensed-host quota. Security,
roles, enrollment trust, validation, destructive-action safeguards, and integration
prerequisites are not disabled.

## Reproduce

Use Go and Node versions from `go.mod` and `package.json`, Yarn from
`packageManager`, Docker Compose, and Python 3. Install dependencies with
`go mod download` and `yarn install --frozen-lockfile --non-interactive`.

Start the upstream disposable database and a standalone Redis. Ports default to
3307 and 6379; `FLEET_MYSQL_TEST_PORT` can select another MySQL test port. Use a
separate Compose project if these names/ports belong to another workload.

```bash
# Keep COMPOSE_PROJECT_NAME exported for every test command: the MySQL test helpers
# call docker compose themselves and otherwise report "mysql_test is not running".
export COMPOSE_PROJECT_NAME=fleet-mod-ci
export REDIS_TEST_SKIP_CLUSTER=1   # cluster tests need upstream's six-node Redis setup
export TMPDIR="${TMPDIR:-$PWD/build}"
mkdir -p "$TMPDIR"
docker compose up -d mysql_test redis
# Wait for mysqladmin -uroot -ptoor ping inside mysql_test before tests.

MYSQL_TEST=1 REDIS_TEST=1 go test ./ee/server/licensing ./server/contexts/license ./server/fleet \
  ./server/config ./server/mdm/apple/... ./server/mdm/android/service/... \
  ./server/platform/endpointer ./server/authz ./server/service/middleware/auth ./pkg/spec \
  ./pkg/fleetdbase
go test ./ee/server/service -run 'TestPSSO' -count=1
go test ./cmd/fleet -run 'TestAndroid(CronsUseConfiguredProvider|ProviderSwitchPreservesEnterprise)' -count=1

go test -race ./ee/server/licensing ./server/contexts/license ./server/fleet \
  ./server/platform/endpointer ./server/authz ./server/service/middleware/auth

MYSQL_TEST=1 REDIS_TEST=1 go test ./cmd/fleet ./server/service ./server/datastore/mysql \
  -run 'Test.*(Analytics|Statistics|LocalVendorSigning|ModifyAppConfig|MDMAppleAuthorization|GetMDMApple|Features|HistoricalData|PSSO)' \
  -count=1 -v

yarn test --runInBand --watch=false \
  --testPathPattern='Statistics|permissions|utilities/license|MainContent|DeviceUserPage|AccountSidePanel|context/app|AuthenticatedRoutes|LoginPage|Avatar|utilities/helpers|components/App'
yarn tsc --noEmit
bash tools/mod-project/build.sh
python3 tools/mod-project/test_smoke_env.py -v
yarn playwright install chromium
FLEET_MOD_BROWSER_SMOKE=1 bash tools/mod-project/run-smoke.sh research
bash tools/mod-project/run-smoke.sh free
go test -tags full ./server/mail -count=1
go test ./server/service -run 'Test.*(Email|Asset|Subpath|WriteBrowserSecurityHeaders|LocalVendorSigning)' -count=1
go test -race ./orbit/pkg/dialogassets ./orbit/pkg/setup_experience ./orbit/pkg/useraction ./orbit/pkg/swiftdialog -count=1
python3 tools/mod-project/test_build_release.py -v
git diff --check
```

`run-smoke.sh` drops/recreates ONLY `fleet_mod_smoke_research` or
`fleet_mod_smoke_free` in the selected Compose test MySQL service. Never point it
at production. It runs the built server on loopback, generates disposable local
users, removes the temporary secret config and browser credentials, terminates
the server on exit, and prints the retained diagnostic log directory. It refuses
an occupied HTTP port. `FLEET_MOD_SMOKE_PORT` overrides the loopback HTTP port.
Both migration and server subprocesses discard inherited `FLEET_*` configuration
and explicitly set only the selected tier and an empty signed-license key. They
also replace stdin with `/dev/null`: Fleet otherwise interprets piped input as
additional CLI flags, which outrank YAML and can retarget database migrations. The
wrapper retains the parent shell's smoke/Compose settings for orchestration;
they cannot override Fleet's generated database, Redis, or listener settings.
External server HTTP fetches are refused during this credential-free test; this
is not an integration test of those services. Browser smoke records and rejects
all attempted external HTTP requests on the exercised default app routes.

The broad Apple unit command deliberately does not enable Redis integration
tests: some upstream tests require a six-node cluster. Run those separately with
the upstream `docker-compose-redis-cluster.yml` environment. Do not equate a
skipped integration test with a pass. On macOS, upstream supports
`REDIS_TEST_SKIP_CLUSTER=1` for suites that intentionally omit cluster tests.

## Recorded evidence: fleet-v4.95.0 replay (2026-11-17)

Branch `v4.95.0` = `fleet-v4.95.0` plus the seven fork commits from `cherry`
(`fleet-v4.94.1..v4.94.1`), replayed in order, then one adaptation commit and one
hardening commit for 4.95.0.

- Replay: the research/routing commit conflicted in ten files (Android reconcile
  signatures, which now carry upstream's retry counter; statistics payload; Pendo
  removal next to upstream's new session-expiry handler; four test files both sides
  added). Each was resolved keeping upstream's change and the fork's intent. The other
  six commits applied cleanly.
- Adaptation: upstream moved bundled images to `frontend/assets/images`, so the local
  default avatar import moved with them; the Android provider-routing test stubs the
  datastore reads of 4.95.0's new app reconciler.
- Hardening: upstream's new AI-generated-profile counter records nothing (it only fed
  usage statistics); the image base matches upstream's Alpine 3.23.8 security bump;
  `build-release.py` refuses a `--version` that is not the branch's upstream tag and
  records the tag, commit and frontend feature flags; smoke tests detect license
  banners by element and cover 4.95.0's patch-deadline Premium gate in both tiers.
- New 4.95.0 gates reviewed: the licensed-host-limit banner (research omits
  `device_count`, so it never shows; frontend test added), patch deadline settings,
  AI tool software, MCP, EJBCA, Okta Windows conditional access and Google device IDs
  all go through `IsPremium`.
- Results: Go unit/Apple/Android packages, `-race` licensing/authz suites, MySQL
  regressions (316 subtests, 0 skipped), Android routing, Platform SSO and fleetd-base
  tests, mail and agent presentation tests all pass. Frontend: 23 suites, 299 tests;
  `tsc` clean. `build.sh` produced `4.95.0-mod-project` at a clean revision. Research
  HTTP + browser smoke (19 checks, no external browser requests, new patch-deadline
  page without a license gate) and free smoke pass. `test_smoke_env.py` 5/5 and
  `test_build_release.py` 8/8 pass. Redis cluster tests were skipped as documented.

Known follow-ups, recorded rather than changed (the deployment is Mac-only):

- Android: an admin disconnect under a mismatched provider treats "not visible" as
  deleted; the default-policy app-report and per-host-policy migration jobs and the
  software worker use the routed client without re-verifying the enterprise;
  `android_service.NewService` and a nil client in `ReconcileProfilesWithClient` still
  fall back to default routing if a future replay calls them; no test drives a real
  app-list request through the configured provider.
- Windows: a macOS-only fleetd-base mirror publishes an empty MSI entry.
- Apple: local CSR signing is chosen only when a vendor setting is non-empty (see the
  README note on exact variable names); the research license still reports a zero
  expiration date to API consumers; the frontend counts signed trial licenses as
  Premium, which matches the server.

## Recorded evidence: fleet-v4.94.1 (historical)

The re-audit verified the following against the working patch:

- Targeted initial MySQL/service/server regression run: 278 passing tests, no failures or
  skips. A first shell wrapper failed after Go completed because zsh reserves
  `status`; re-running with `result` returned exit 0 and the same 278 passes.
- Full licensing, license context, Fleet types, configuration, endpoint, authz,
  auth middleware, and spec suites returned exit 0.
- Six core licensing/type/endpoint/authorization suites passed with `-race`.
- Apple MDM package subtree returned exit 0, including app metadata, manifests,
  VPP, mobileconfig, and local vendor cryptographic signing tests.
- Real research HTTP smoke: expected tier, no expiry response header, analytics
  false on fresh config and after attempted opt-in, Premium fleet create/delete
  and scoped query creation, anonymous denial, and observer mutation denial.
- Real free HTTP smoke: expected tier, analytics remains false, and Premium fleet
  creation is still rejected without the explicit research opt-in.
- A pre-correction built-browser smoke rendered login and five settings/control
  routes but FAILED its privacy assertion on a Gravatar capability probe. This
  is a regression reproduction, not a passing privacy result.
- The dedicated workflow validates with `actionlint` after fixing an invalid
  job-level `runner.temp` expression and separating cluster-dependent tests.

Post-correction verification:

- Final integrated MySQL/service/server regression run including Android provider
  tests: 296 passes, no failures or skips.
- 1,288 selected entitlement/authorization tests across EE services, core services,
  and fleetctl passed with MySQL/Redis enabled; no failures or skips.
- 267 frontend tests across 22 suites passed, no failures or pending tests.
- Full server-command and Android service/client package suites passed. Local TLS
  tests exercised all three background jobs against direct Google, custom proxy,
  default proxy, and legacy dev routing; no real provider credentials were used.
- Provider-switch tests cover verification and all three reconciliation entry
  points: missing/mismatched enterprise aborts without enrollment mutations.
- Focused Android routing/client-isolation/provider guards passed with `-race`.
- Versioned production dashboard/server build completed as `4.94.1-mod-project`.
- Built dashboard browser smoke passed login and five routes with no external
  HTTP requests and no uncaught runtime errors; research HTTP smoke passed.
- Free HTTP smoke passed after fixing macOS Bash empty-array expansion and
  allowing socket reuse for recently closed ports; occupied ports still fail.
- Embedded transactional-mail tests passed AFTER regenerating bindata. Running
  them against stale generated assets correctly reproduced the old fonts/images.
- Email/subpath/CSP/local-signing service regressions passed; four agent
  presentation packages passed with `-race`. Native SwiftDialog GUI and actual
  mail delivery were not exercised.
- TypeScript, shell syntax, JavaScript syntax, actionlint, `go vet` on affected
  backend/agent packages, and diff hygiene passed. Scoped frontend ESLint passed
  with no errors and ten existing warnings.
- Late independent audit reproduced an inherited-environment override of the
  smoke database with `config_dump` (no non-disposable database was migrated).
  A follow-up fix isolates Fleet subprocess environments and stdin. Five new
  regression tests pass, including the real built binary's resolved database/
  listeners, injected CLI flags, open input pipes, and closed stdin. The piped
  argument regression failed before stdin isolation and passed after the fix.
  Research/free HTTP and research browser smoke then passed with deliberately
  invalid inherited MySQL, Redis, server, license, and Android proxy settings.

Test counts include subtests and selected overlapping coverage; do not add them
into a unique-test total. Build metadata correctly reports dirty until the
custom commit is created. Publication and remote CI are checked separately.
The dedicated workflow only tests/builds: it does not publish images or deploy
to devices. Repository Actions were disabled at publication preflight
(`enabled: false`); no hosted CI success is claimed. Enabling Actions is a
separate repository-policy decision, not silently changed by this patch.

## External acceptance boundary

Synthetic certificates prove CSR format/signature and local fail-closed behavior,
not Apple portal acceptance. Actual vendor certificates, Apple portal upload,
APNs renewal, MDM enrollment, app installation, and Android enrollment/Pub/Sub
require legitimate platform credentials and disposable test devices. These are
not proven by local entitlement or browser tests. Keep the same Apple push
identity when renewing; changing identities can require re-enrollment.

Default operational dependencies remain intentionally available (including
Fleet-maintained app catalogs and agent updates). Operator-configured external
branding and observability are not product analytics. The standalone Apple OTA
enrollment template retains an SRI-pinned functional QR-code library from cdnjs;
its third-party dependency is not covered by the default-dashboard no-egress
smoke and is not Fleet product analytics. The fork is not air-gapped.

## Reapply to upstream

Replay the full ordered range of fork commits from `cherry` (the deployed version
branch), not just the branch tip and not the historical `mod-project` branch, which
holds only the original two commits.

```bash
git fetch upstream --tags && git fetch origin
OLD=4.95.0 NEW=<verified stable upstream version>
git rev-list --count "fleet-v$OLD..origin/cherry"   # the fork commits to replay
git switch -c "v$NEW" "refs/tags/fleet-v$NEW"
git cherry-pick "fleet-v$OLD..origin/cherry"
```

A clean cherry-pick is not proof of compatibility. Resolve conflicts by intent,
then repeat the complete tests/build/smoke. Inspect new raw tier/expiry/quota gates,
all AMAPI client constructors and background workers, license-key consumers,
analytics collection/upload/config coercion, new Fleet-hosted calls, moved assets,
and unsolicited browser defaults. Preserve genuine remote authorization,
user-configured integrations, and RBAC. Review security/dependency changes in the
new release independently, including the Docker base image.

Push the new version branch and `v$NEW-mod` tag, publish the image, deploy, and only
then move `cherry` with `--force-with-lease`. Never reset the user's worktree or
force-push `main`.
