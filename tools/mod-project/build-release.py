#!/usr/bin/env python3
"""Build versioned fork server/CLI archives without publishing or embedding secrets."""

import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tarfile
import zipfile

ROOT = Path(__file__).resolve().parents[2]


def run(*args, env=None):
    print("+", " ".join(map(str, args)), flush=True)
    subprocess.run(list(map(str, args)), cwd=ROOT, env=env, check=True, stdin=subprocess.DEVNULL)


def verify_build_info(binary, revision, system, arch, server):
    info = subprocess.check_output(["go", "version", "-m", str(binary)], cwd=ROOT, text=True)
    expected = ["vcs.revision=" + revision, "vcs.modified=false", "GOOS=" + system,
                "GOARCH=" + arch, "CGO_ENABLED=" + ("1" if server else "0")]
    if server:
        expected.append("-tags=full,fts5,netgo")
    for field in expected:
        if "\tbuild\t" + field + "\n" not in info:
            raise RuntimeError("Unexpected build provenance for " + str(binary) + ": missing " + field)


def verify_cli(binary):
    run(binary, "--version")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--skip-dashboard", action="store_true")
    args = parser.parse_args()
    if not re.fullmatch(r"\d+\.\d+\.\d+", args.version):
        parser.error("Version must be an explicit major.minor.patch")
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    if any(output.iterdir()):
        parser.error("Output directory must be empty")
    revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    if subprocess.check_output(["git", "status", "--porcelain", "--untracked-files=no"], cwd=ROOT, text=True):
        parser.error("Commit tracked source changes before building a release")
    branch = subprocess.check_output(["git", "branch", "--show-current"], cwd=ROOT, text=True).strip()
    date = datetime.now(timezone.utc).strftime("%Y-%m-%d")
    environment = dict(os.environ)
    environment.setdefault("GOMAXPROCS", "3")
    environment.setdefault("GOFLAGS", "-p=3")
    if not args.skip_dashboard:
        run("yarn", "generate:osquery-sql-parser", env=environment)
        run("yarn", "webpack", env=dict(environment, NODE_ENV="production"))
        run("make", "generate-go", env=environment)
    binaries = output / "binaries"
    binaries.mkdir()
    provenance = {"version": args.version + "-mod", "revision": revision, "branch": branch,
                  "upstream_version": args.version, "build_date": date, "artifacts": []}
    for name, targets in (("fleet", [("linux", "arm64"), ("linux", "amd64")]),
                          ("fleetctl", [(system, arch) for system in ("linux", "darwin", "windows")
                                        for arch in ("arm64", "amd64")])):
        for system, arch in targets:
            destination = binaries / (system + "-" + arch)
            destination.mkdir(exist_ok=True)
            binary = destination / (name + (".exe" if system == "windows" else ""))
            fields = {"appName": name, "version": args.version + "-mod", "branch": branch,
                      "revision": revision, "buildDate": date, "buildUser": "0xMetaLabs"}
            flags = "-s -w " + " ".join("-X github.com/fleetdm/fleet/v4/server/version." + key + "=" + value
                                         for key, value in fields.items())
            target_env = dict(environment, GOOS=system, GOARCH=arch, CGO_ENABLED="0")
            tags = []
            if name == "fleet":
                if not shutil.which("zig"):
                    parser.error("Zig is required for static Linux SQLite/FTS5 server cross-builds")
                target = "aarch64" if arch == "arm64" else "x86_64"
                target_env.update(CGO_ENABLED="1", CC="zig cc -target " + target + "-linux-musl")
                flags += " -linkmode external -extldflags '-static'"
                tags = ["-tags", "full,fts5,netgo"]
            run("go", "build", "-trimpath", *tags, "-ldflags", flags, "-o", binary,
                "./cmd/" + name, env=target_env)
            verify_build_info(binary, revision, system, arch, name == "fleet")
            if system == "darwin":
                continue
            label = name + "_v" + args.version + "_" + system + "_" + arch
            make_archive(output, label, binary, system == "windows")
    universal = binaries / "darwin-universal" / "fleetctl"
    universal.parent.mkdir()
    run("lipo", "-create", binaries / "darwin-arm64/fleetctl", binaries / "darwin-amd64/fleetctl",
        "-output", universal)
    run("codesign", "--force", "--sign", "-", universal)
    run("codesign", "--verify", "--strict", universal)
    verify_cli(universal)
    make_archive(output, "fleetctl_v" + args.version + "_macos_universal", universal, False)
    make_archive(output, "fleetctl_v" + args.version + "_macos_universal", universal, True)
    licenses = output / "licenses"
    licenses.mkdir()
    shutil.copy2(ROOT / "LICENSE", licenses / "LICENSE")
    shutil.copy2(ROOT / "ee/LICENSE", licenses / "LICENSE-EE")
    shutil.copy2(ROOT / "tools/mod-project/Dockerfile", output / "Dockerfile")
    provenance["macos_signing"] = "ad-hoc only; no Developer ID or notarization claimed"
    for asset in sorted(output.iterdir()):
        if asset.is_file():
            provenance["artifacts"].append({"name": asset.name, "sha256": hashlib.sha256(asset.read_bytes()).hexdigest(),
                                            "size": asset.stat().st_size})
    (output / "build-provenance.json").write_text(json.dumps(provenance, indent=2) + "\n")
    write_checksums(output)


def make_archive(output, label, binary, use_zip):
    files = [(binary, binary.name), (ROOT / "LICENSE", "LICENSE"), (ROOT / "ee/LICENSE", "LICENSE-EE"),
             (ROOT / "docs/mod-project/README.md", "MOD-README.md")]
    if use_zip:
        with zipfile.ZipFile(output / (label + ".zip"), "w", zipfile.ZIP_DEFLATED) as archive:
            for source, name in files:
                archive.write(source, label + "/" + name)
    else:
        with tarfile.open(output / (label + ".tar.gz"), "w:gz") as archive:
            for source, name in files:
                archive.add(source, arcname=label + "/" + name)


def write_checksums(output):
    lines = [hashlib.sha256(path.read_bytes()).hexdigest() + "  " + path.name
             for path in sorted(output.iterdir()) if path.is_file() and path.name != "checksums.txt"]
    (output / "checksums.txt").write_text("\n".join(lines) + "\n")


if __name__ == "__main__":
    main()
