#!/usr/bin/env python3
"""Exercise a running disposable Fleet server without platform/device mutations."""

import argparse
import json
import secrets
import urllib.error
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--url", default="http://127.0.0.1:18088")
    parser.add_argument("--expected-tier", choices=("research", "free"), default="research")
    parser.add_argument("--browser", action="store_true", help="Exercise the built dashboard with Playwright")
    args = parser.parse_args()
    base = args.url.rstrip("/")
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

    def request(method, path, payload=None, token=None, expected=(200,)):
        headers = {"Content-Type": "application/json"}
        if token:
            headers["Authorization"] = "Bearer " + token
        data = json.dumps(payload).encode() if payload is not None else None
        req = urllib.request.Request(base + path, data=data, headers=headers, method=method)
        try:
            response = opener.open(req, timeout=30)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            body = response.read()
            status = response.status
            response_headers = response.headers
        assert status in expected, f"{method} {path}: HTTP {status}: {body[:600]!r}"
        return json.loads(body) if body else {}, response_headers

    def check(condition, message):
        assert condition, message
        print("PASS " + message)

    request("GET", "/healthz")
    password = "Smoke-" + secrets.token_urlsafe(24)
    setup, _ = request("POST", "/api/v1/setup", {
        "admin": {"name": "Mod smoke admin", "email": "admin@example.test", "password": password},
        "org_info": {"org_name": "Mod smoke"},
        "server_url": base,
    })
    token = setup["token"]
    # Do not print credentials; a disposable database contains the generated users.
    request("PATCH", "/api/v1/fleet/config", {"server_settings": {"server_url": base}}, token)
    config, headers = request("GET", "/api/v1/fleet/config", token=token)
    check(config["license"]["tier"] == args.expected_tier, "actual HTTP config exposes expected entitlement")
    check(headers.get("X-Fleet-License") is None, "research/free config has no expired-license response header")
    check(config["server_settings"]["enable_analytics"] is False, "analytics disabled on fresh config")
    request("PATCH", "/api/v1/fleet/config", {"server_settings": {"enable_analytics": True}}, token)
    config, _ = request("GET", "/api/v1/fleet/config", token=token)
    check(config["server_settings"]["enable_analytics"] is False, "API cannot re-enable analytics")
    request("POST", "/api/v1/fleet/fleets", {"name": "Unauthorized"}, expected=(401,))
    check(True, "anonymous premium mutation denied")
    if args.expected_tier == "research":
        team_response, _ = request("POST", "/api/v1/fleet/fleets", {"name": "Research smoke team"}, token)
        team = team_response.get("fleet", team_response.get("team"))
        assert isinstance(team, dict), "create fleet returned no fleet object"
        check(team["id"] > 0, "actual EE service creates a fleet without a signed license")
        query, _ = request("POST", "/api/v1/fleet/queries", {
            "name": "Research smoke query", "query": "SELECT 1;", "team_id": team["id"],
            "interval": 3600, "platform": "darwin", "description": "Disposable smoke test",
        }, token)
        check(bool(query.get("query", query.get("report"))), "team-scoped query creation works")
        user, _ = request("POST", "/api/v1/fleet/users/admin", {
            "name": "Mod smoke observer", "email": "observer@example.test", "password": password,
            "global_role": "observer", "api_only": False, "admin_forced_password_reset": False,
        }, token)
        check(bool(user["user"]["id"]), "observer created for negative RBAC check")
        login, _ = request("POST", "/api/v1/fleet/login", {"email": "observer@example.test", "password": password})
        request("POST", "/api/v1/fleet/fleets", {"name": "Forbidden"}, login["token"], expected=(403,))
        check(True, "research entitlement does not bypass observer RBAC")
        request("GET", f"/api/v1/fleet/software/patch_deadline_settings?fleet_id={team['id']}", token=token)
        check(True, "Premium patch deadline settings (new in 4.95.0) work in research")
        request("DELETE", f"/api/v1/fleet/fleets/{team['id']}", token=token, expected=(200, 204))
        check(True, "created fleet removed through EE service")
    else:
        request("POST", "/api/v1/fleet/fleets", {"name": "Free must reject"}, token, expected=(402, 403))
        check(True, "no opt-in preserves the free-edition premium gate")
        request("GET", "/api/v1/fleet/software/patch_deadline_settings", token=token, expected=(402,))
        check(True, "patch deadline settings (new in 4.95.0) stay Premium-only in free")
    print("SMOKE PASS: " + args.expected_tier)
    if args.browser:
        import os
        import pathlib
        import subprocess
        import tempfile

        fd, path = tempfile.mkstemp(prefix="fleet-browser-", suffix=".json")
        try:
            with os.fdopen(fd, "w") as output:
                json.dump({"url": base, "email": "admin@example.test", "password": password}, output)
            subprocess.run(["node", str(pathlib.Path(__file__).with_name("browser-smoke.cjs")), path], check=True)
        finally:
            os.unlink(path)


if __name__ == "__main__":
    main()
