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

import runpy
from types import SimpleNamespace

BUILDER = SimpleNamespace(**runpy.run_path(str(Path(__file__).with_name("build-release.py"))))


class ReleaseBuilderTests(unittest.TestCase):
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


if __name__ == "__main__":
    unittest.main()
