#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../.."
base=$(git describe --tags --match 'fleet-v*' --abbrev=0)
version=${VERSION:-${base#fleet-v}-mod-project}
revision=$(git rev-parse HEAD)
if [[ -n "$(git status --porcelain --untracked-files=normal)" ]]; then
  revision="${revision}-dirty"
fi
branch=$(git branch --show-current)
flags="-s -w -X github.com/fleetdm/fleet/v4/server/version.appName=Fleet"
flags+=" -X github.com/fleetdm/fleet/v4/server/version.version=${version}"
flags+=" -X github.com/fleetdm/fleet/v4/server/version.branch=${branch:-detached}"
flags+=" -X github.com/fleetdm/fleet/v4/server/version.revision=${revision}"
flags+=" -X github.com/fleetdm/fleet/v4/server/version.buildDate=$(date -u +%Y-%m-%d)"
flags+=" -X github.com/fleetdm/fleet/v4/server/version.buildUser=mod-project"

yarn generate:osquery-sql-parser
NODE_ENV=production yarn webpack
make generate-go
go build -tags full,fts5,netgo -ldflags "$flags" -o build/fleet ./cmd/fleet
./build/fleet version --full
