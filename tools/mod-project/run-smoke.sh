#!/usr/bin/env bash
# Destructive only to the explicitly reserved disposable smoke database.
set -euo pipefail
cd "$(dirname "$0")/../.."
export TMPDIR=${TMPDIR:-/tmp}
tier=${1:-research}
case "$tier" in research|free) ;; *) printf 'Expected research or free\n' >&2; exit 2 ;; esac
port=${FLEET_MOD_SMOKE_PORT:-18088}
mysql_port=${FLEET_MYSQL_TEST_PORT:-3307}
python3 - "$port" <<'PY'
import socket, sys
with socket.socket() as listener:
    listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    listener.bind(("127.0.0.1", int(sys.argv[1])))
PY
database="fleet_mod_smoke_${tier}"
work=$(mktemp -d "${TMPDIR:-/tmp}/fleet-mod-smoke.XXXXXX")
pid=""
cleanup() {
  result=$?
  trap - EXIT
  if [[ -n "$pid" ]]; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
  rm -f "$work/config.yml"
  if [[ "$result" != 0 ]]; then printf 'Failed smoke logs: %s\n' "$work" >&2; fi
  exit "$result"
}
trap cleanup EXIT
export FLEET_MOD_SMOKE_WORK="$work" FLEET_MOD_SMOKE_DATABASE="$database"
export FLEET_MOD_SMOKE_MYSQL_PORT="$mysql_port" FLEET_MOD_SMOKE_PORT="$port"
python3 - <<'PY'
import json, os, secrets
config = {
    "mysql": {"address": "127.0.0.1:" + os.environ["FLEET_MOD_SMOKE_MYSQL_PORT"], "username": "root", "password": "toor", "database": os.environ["FLEET_MOD_SMOKE_DATABASE"]},
    "redis": {"address": "127.0.0.1:6379"},
    "server": {"address": "127.0.0.1:" + os.environ["FLEET_MOD_SMOKE_PORT"], "tls": False, "private_key": secrets.token_urlsafe(32)},
    "logging": {"disable_banner": True},
    "vulnerabilities": {"disable_schedule": True},
    "filesystem": {"result_log_file": os.environ["FLEET_MOD_SMOKE_WORK"] + "/results.log", "status_log_file": os.environ["FLEET_MOD_SMOKE_WORK"] + "/status.log"},
}
p = os.path.join(os.environ["FLEET_MOD_SMOKE_WORK"], "config.yml")
fd = os.open(p, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, "w") as f:
    json.dump(config, f)
PY
docker compose exec -T mysql_test mysql -uroot -ptoor -e "DROP DATABASE IF EXISTS ${database}; CREATE DATABASE ${database};"
# Refuse functional external fetches during this credential-free smoke; loopback is allowed.
export HTTP_PROXY=http://127.0.0.1:9 HTTPS_PROXY=http://127.0.0.1:9 NO_PROXY=127.0.0.1,localhost
python3 tools/mod-project/smoke_env.py "$tier" ./build/fleet prepare db --config "$work/config.yml" --no-prompt > "$work/prepare.log" 2>&1
python3 tools/mod-project/smoke_env.py "$tier" ./build/fleet serve --config "$work/config.yml" > "$work/server.log" 2>&1 &
pid=$!
if ! python3 - "$port" "$pid" <<'PY'
import os, sys, time, urllib.request
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
for _ in range(60):
    try:
        os.kill(int(sys.argv[2]), 0)
    except OSError:
        sys.exit("Fleet exited before becoming healthy; inspect smoke logs")
    try:
        with opener.open("http://127.0.0.1:" + sys.argv[1] + "/healthz", timeout=1) as r:
            if r.status == 200:
                sys.exit(0)
    except (OSError, ValueError):
        pass
    time.sleep(1)
sys.exit("Fleet did not become healthy; inspect smoke logs")
PY
then
  printf 'Smoke logs: %s\n' "$work" >&2
  exit 1
fi
if [[ "$tier" == research && "${FLEET_MOD_BROWSER_SMOKE:-0}" == 1 ]]; then
  python3 tools/mod-project/smoke.py --url "http://127.0.0.1:$port" --expected-tier "$tier" --browser
else
  python3 tools/mod-project/smoke.py --url "http://127.0.0.1:$port" --expected-tier "$tier"
fi
printf 'Smoke logs: %s\n' "$work"
