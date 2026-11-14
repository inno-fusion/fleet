# Verification and maintenance

## Scope

Baseline: Fleet `fleet-v4.94.1`, commit
`7e7d17b75abd7a81f8320ca75531b47f0933bc1b`. This is an opt-in local
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
export COMPOSE_PROJECT_NAME=fleet-mod-ci
export TMPDIR="${TMPDIR:-$PWD/build}"
mkdir -p "$TMPDIR"
docker compose up -d mysql_test redis
# Wait for mysqladmin -uroot -ptoor ping inside mysql_test before tests.

go test ./ee/server/licensing ./server/contexts/license ./server/fleet \
  ./server/config ./server/mdm/apple/... ./server/mdm/android/service/... \
  ./server/platform/endpointer ./server/authz ./server/service/middleware/auth ./pkg/spec

go test -race ./ee/server/licensing ./server/contexts/license ./server/fleet \
  ./server/platform/endpointer ./server/authz ./server/service/middleware/auth

MYSQL_TEST=1 REDIS_TEST=1 go test ./cmd/fleet ./server/service ./server/datastore/mysql \
  -run 'Test.*(Analytics|Statistics|LocalVendorSigning|ModifyAppConfig|MDMAppleAuthorization|GetMDMApple|Features|HistoricalData)' \
  -count=1 -v

yarn test --runInBand --watch=false \
  --testPathPattern='Statistics|permissions|utilities/license|MainContent|DeviceUserPage|AccountSidePanel|context/app|AuthenticatedRoutes|LoginPage|Avatar|utilities/helpers|components/App'
yarn tsc --noEmit
bash tools/mod-project/build.sh
python3 tools/mod-project/test_smoke_env.py -v
yarn playwright install chromium
FLEET_MOD_BROWSER_SMOKE=1 bash tools/mod-project/run-smoke.sh research
bash tools/mod-project/run-smoke.sh free
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

## Recorded evidence

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

The branch contains the complete feature commit followed by a smoke-isolation
correction. The correction was discovered after publication, so published history
was not rewritten. Replay the full ordered range above the recorded upstream
base, not just the branch tip. Keep code, regressions, build/CI and documentation
together when consolidating the patch for a later release.

```bash
git fetch origin mod-project
base=7e7d17b75abd7a81f8320ca75531b47f0933bc1b
# Fetch/verify a specific new stable upstream tag separately.
git switch -c mod-project-next VERIFIED_UPSTREAM_TAG
git cherry-pick "$base..origin/mod-project"
```

A clean cherry-pick is not proof of compatibility. Resolve conflicts by intent,
then repeat the complete tests/build/smoke. Inspect new raw tier/expiry/quota gates,
all AMAPI client constructors and background workers, license-key consumers,
analytics collection/upload/config coercion, and unsolicited browser defaults.
Preserve genuine remote authorization, user-configured integrations, and RBAC.
Review security/dependency changes in the new release independently.

For exact-baseline replay verification, apply the full range to a disposable
worktree at the recorded base with `git cherry-pick --no-commit`, then compare
`git write-tree` with the branch tip's tree. Remove only that disposable
worktree after checking equality. Never reset the user's worktree or force-push
`main`. Push only `refs/heads/mod-project` and verify the remote commit SHA and
Actions results before reporting publication success.
