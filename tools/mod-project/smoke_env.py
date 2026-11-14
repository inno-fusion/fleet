#!/usr/bin/env python3
"""Run disposable Fleet smoke commands without inherited Fleet configuration."""

import os
import sys


def isolated_environment(tier, environment):
    if tier not in ("research", "free"):
        raise ValueError("Expected research or free")
    # Environment variables take precedence over YAML, even for database targets.
    clean = {key: value for key, value in environment.items() if not key.startswith("FLEET_")}
    clean["FLEET_LICENSE_KEY"] = ""
    clean["FLEET_LICENSE_LOCAL_RESEARCH"] = str(tier == "research").lower()
    return clean


def main():
    if len(sys.argv) < 3:
        sys.exit("Usage: smoke_env.py research|free command [args...]")
    environment = isolated_environment(sys.argv[1], os.environ)
    # Fleet interprets piped stdin as additional CLI flags; never inherit it here.
    source = os.open(os.devnull, os.O_RDONLY)
    if source != 0:
        os.dup2(source, 0)
        os.close(source)
    else:
        os.set_inheritable(0, True)
    os.execvpe(sys.argv[2], sys.argv[2:], environment)


if __name__ == "__main__":
    main()
