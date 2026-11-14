#!/usr/bin/env python3
"""Prove inherited operator configuration cannot escape the smoke sandbox."""

import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

from smoke_env import isolated_environment

ROOT = Path(__file__).resolve().parents[2]
WRAPPER = Path(__file__).with_name("smoke_env.py")


class SmokeEnvironmentTests(unittest.TestCase):
    def test_filters_all_fleet_overrides_without_mutating_parent(self):
        parent = {"PATH": "/bin", "HOME": "/home/test", "FLEET_MYSQL_DATABASE": "sentinel",
                  "FLEET_MDM_ANDROID_PROXY_URL": "https://operator.test", "FLEET_NEW_SETTING": "future"}
        for tier in ("research", "free"):
            clean = isolated_environment(tier, parent)
            self.assertEqual({key for key in clean if key.startswith("FLEET_")},
                             {"FLEET_LICENSE_KEY", "FLEET_LICENSE_LOCAL_RESEARCH"})
            self.assertEqual(clean["FLEET_LICENSE_LOCAL_RESEARCH"], str(tier == "research").lower())
            self.assertEqual(clean["PATH"], parent["PATH"])
            self.assertEqual(parent["FLEET_MYSQL_DATABASE"], "sentinel")
        with self.assertRaises(ValueError):
            isolated_environment("other", parent)

    def test_open_stdin_pipe_does_not_block_startup(self):
        process = subprocess.Popen(["python3", str(WRAPPER), "free", str(ROOT / "build/fleet"),
                                    "version"], stdin=subprocess.PIPE,
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            self.assertEqual(process.wait(timeout=5), 0)
        finally:
            if process.poll() is None:
                process.kill()
                process.wait()
            if process.stdin is not None:
                process.stdin.close()

    def test_closed_stdin_is_reopened_for_fleet(self):
        result = subprocess.run(["/bin/sh", "-c", 'exec 0<&-; exec "$@"', "smoke-wrapper",
                                 "python3", str(WRAPPER), "free", str(ROOT / "build/fleet"),
                                 "version"], capture_output=True, text=True, timeout=5)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_piped_arguments_cannot_override_disposable_config(self):
        fd, path = tempfile.mkstemp(suffix=".json")
        try:
            with os.fdopen(fd, "w") as output:
                json.dump({"mysql": {"database": "fleet_mod_smoke_research",
                                     "address": "127.0.0.1:3317"}}, output)
            result = subprocess.run(["python3", str(WRAPPER), "research", str(ROOT / "build/fleet"),
                                     "config_dump", "--config", path],
                                    input="--mysql_database sentinel_do_not_touch --mysql_address 127.0.0.1:9\n",
                                    capture_output=True, text=True, check=True, timeout=10)
            self.assertNotIn("sentinel_do_not_touch", result.stdout)
            self.assertNotIn("address: 127.0.0.1:9", result.stdout)
            self.assertIn("database: fleet_mod_smoke_research", result.stdout)
            self.assertIn("address: 127.0.0.1:3317", result.stdout)
        finally:
            os.unlink(path)

    def test_actual_binary_resolves_disposable_config_despite_inherited_overrides(self):
        environment = dict(os.environ, FLEET_MYSQL_DATABASE="sentinel_do_not_touch",
                           FLEET_MYSQL_ADDRESS="127.0.0.1:9", FLEET_REDIS_ADDRESS="127.0.0.1:9",
                           FLEET_SERVER_ADDRESS="0.0.0.0:9", FLEET_SERVER_TLS="true")
        config = {"mysql": {"address": "127.0.0.1:3317", "database": "fleet_mod_smoke_research"},
                  "redis": {"address": "127.0.0.1:6379"},
                  "server": {"address": "127.0.0.1:18088", "tls": False}}
        fd, path = tempfile.mkstemp(suffix=".json")
        try:
            with os.fdopen(fd, "w") as output:
                json.dump(config, output)
            result = subprocess.run(["python3", str(WRAPPER), "research", str(ROOT / "build/fleet"),
                                     "config_dump", "--config", path], env=environment,
                                    capture_output=True, text=True, check=True)
            self.assertNotIn("sentinel_do_not_touch", result.stdout)
            self.assertNotIn("127.0.0.1:9", result.stdout)
            self.assertNotIn("0.0.0.0:9", result.stdout)
            self.assertIn("database: fleet_mod_smoke_research", result.stdout)
            self.assertIn("address: 127.0.0.1:3317", result.stdout)
            self.assertIn("address: 127.0.0.1:6379", result.stdout)
            self.assertIn("address: 127.0.0.1:18088", result.stdout)
            self.assertIn("tls: false", result.stdout)
        finally:
            os.unlink(path)


if __name__ == "__main__":
    unittest.main()
