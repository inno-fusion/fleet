#!/usr/bin/env python3
"""Verify release archives, checksums, and fail-closed argument handling."""

import hashlib
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import unittest
import zipfile
from unittest.mock import Mock, patch

import runpy
from types import SimpleNamespace

BUILDER = SimpleNamespace(**runpy.run_path(str(Path(__file__).with_name("build-release.py"))))


class ReleaseBuilderTests(unittest.TestCase):
    def test_isolated_source_excludes_untracked_and_ignored_assets(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            subprocess.run(["git", "init", "--quiet", str(root)], check=True)
            (root / "assets").mkdir()
            (root / "assets/committed.txt").write_text("committed")
            (root / ".gitignore").write_text("assets/ignored.js\n")
            subprocess.run(["git", "-C", str(root), "add", "."], check=True)
            subprocess.run(["git", "-C", str(root), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
                            "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "fixture"], check=True)
            revision = subprocess.check_output(["git", "-C", str(root), "rev-parse", "HEAD"], text=True).strip()
            (root / "assets/secret.txt").write_text("must not ship")
            (root / "assets/ignored.js").write_text("stale development bundle")
            with patch.dict(BUILDER.isolated_source.__wrapped__.__globals__, ROOT=root):
                with BUILDER.isolated_source(revision) as source:
                    self.assertEqual((source / "assets/committed.txt").read_text(), "committed")
                    self.assertFalse((source / "assets/secret.txt").exists())
                    self.assertFalse((source / "assets/ignored.js").exists())
                    self.assertEqual(subprocess.check_output(["git", "status", "--porcelain"], cwd=source), b"")
                self.assertFalse(source.exists())
            self.assertEqual((root / "assets/secret.txt").read_text(), "must not ship")

    def test_cli_verification_uses_global_version_flag(self):
        with patch.dict(BUILDER.verify_cli.__globals__, run=Mock()) as scope:
            BUILDER.verify_cli(Path("fleetctl"))
            scope["run"].assert_called_once_with(Path("fleetctl"), "--version")

    def test_server_provenance_requires_clean_revision_and_cgo_fts5(self):
        fields = ["vcs.revision=fixture", "vcs.modified=false", "GOOS=linux", "GOARCH=arm64",
                  "CGO_ENABLED=1", "-tags=full,fts5,netgo"]
        info = "".join("\tbuild\t" + field + "\n" for field in fields)
        with patch("subprocess.check_output", return_value=info):
            BUILDER.verify_build_info(Path("fleet"), "fixture", "linux", "arm64", True)
        for field in fields:
            with self.subTest(missing=field), patch("subprocess.check_output", return_value=info.replace(field, "invalid")):
                with self.assertRaises(RuntimeError):
                    BUILDER.verify_build_info(Path("fleet"), "fixture", "linux", "arm64", True)

    def test_archives_include_original_license_texts(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            binary = root / "fleetctl"
            binary.write_bytes(b"test fixture, not an executable")
            for use_zip in (False, True):
                BUILDER.make_archive(root, "fixture", binary, use_zip)
            with tarfile.open(root / "fixture.tar.gz") as archive:
                license_file = archive.extractfile("fixture/LICENSE-EE")
                binary_file = archive.extractfile("fixture/fleetctl")
                assert license_file is not None and binary_file is not None
                self.assertEqual(license_file.read(), (BUILDER.ROOT / "ee/LICENSE").read_bytes())
                self.assertEqual(binary_file.read(), binary.read_bytes())
            with zipfile.ZipFile(root / "fixture.zip") as archive:
                self.assertEqual(archive.read("fixture/LICENSE"), (BUILDER.ROOT / "LICENSE").read_bytes())

    def test_checksums_are_exact_and_exclude_directories_and_self(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "fixture").write_bytes(b"release fixture")
            (root / "binaries").mkdir()
            BUILDER.write_checksums(root)
            expected = hashlib.sha256(b"release fixture").hexdigest() + "  fixture\n"
            self.assertEqual((root / "checksums.txt").read_text(), expected)
            BUILDER.write_checksums(root)
            self.assertEqual((root / "checksums.txt").read_text(), expected)

    def test_malformed_version_fails_before_build(self):
        with tempfile.TemporaryDirectory() as temporary:
            result = subprocess.run([sys.executable, str(BUILDER.__file__), "--version", "v4.94.1",
                                     "--output", temporary], capture_output=True, text=True)
            self.assertEqual(result.returncode, 2)
            self.assertIn("explicit major.minor.patch", result.stderr)

    def test_existing_output_fails_before_build(self):
        with tempfile.TemporaryDirectory() as temporary:
            (Path(temporary) / "sentinel").write_text("do not overwrite")
            result = subprocess.run([sys.executable, str(BUILDER.__file__), "--version", "4.94.1",
                                     "--output", temporary], capture_output=True, text=True)
            self.assertEqual(result.returncode, 2)
            self.assertIn("must be empty", result.stderr)
            self.assertEqual((Path(temporary) / "sentinel").read_text(), "do not overwrite")


    def test_version_without_upstream_tag_fails_before_build(self):
        with tempfile.TemporaryDirectory() as temporary:
            result = subprocess.run([sys.executable, str(BUILDER.__file__), "--version", "0.0.1",
                                     "--output", temporary], capture_output=True, text=True)
            self.assertEqual(result.returncode, 2)
            self.assertIn("Missing upstream tag fleet-v0.0.1", result.stderr)

if __name__ == "__main__":
    unittest.main()
