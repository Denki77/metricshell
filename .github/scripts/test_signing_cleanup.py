"""Exercise the supply-chain-ci recipe with controlled Docker outcomes."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

IMPLEMENTATION = Path(__file__).resolve().parents[2] / "implementation"


class SigningCleanupTest(unittest.TestCase):
    def test_temporary_keys_are_removed(self):
        for failure in ("", "run", "supply-chain-artifacts", "finalized-release-artifacts"):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                (root / "tmp").mkdir()
                shutil.copy2(IMPLEMENTATION / "Makefile", root / "Makefile")
                (root / "scripts").mkdir()
                shutil.copy2(IMPLEMENTATION / "scripts/render-kubernetes-release.sh", root / "scripts")
                shutil.copytree(IMPLEMENTATION / "examples/kubernetes", root / "examples/kubernetes")
                docker = root / "docker"
                docker.write_text("""#!/bin/sh
set -eu
printf '%s\n' "${3:-}" >> stages
case "$1" in
    run)
        for arg do
            case "$arg" in
                *:/keys) keys=${arg%:/keys};;
            esac
        done
        printf '%s\n' "$keys" > key-directory
        printf 'private-test-marker\n' > "$keys/private.hex"
        printf 'public-test-marker\n' > "$keys/public.hex"
        ;;
    build)
        test "$2" = --target
        if test "$3" = supply-chain-artifacts; then
            mkdir -p dist/supply-chain/release
            touch dist/supply-chain/release/METADATA
        fi
        test "$3" != "$FAIL_STAGE"
        ;;
esac
test "$1" != "$FAIL_STAGE"
""")
                docker.chmod(0o700)
                result = subprocess.run(
                    ["make", "supply-chain-ci",
                     "VERSION=0.9.7", "REVISION=test"],
                    cwd=root, capture_output=True, text=True, timeout=30,
                    env={**os.environ, "TMPDIR": str(root / "tmp"),
                         "PATH": str(root) + os.pathsep + os.environ["PATH"], "FAIL_STAGE": failure},
                )
                self.assertEqual(result.returncode == 0, not failure, result.stderr)
                self.assertFalse(Path((root / "key-directory").read_text().strip()).exists())
                if failure.endswith("-artifacts"):
                    self.assertIn(failure, (root / "stages").read_text().splitlines())
                self.assertNotIn("private-test-marker", result.stdout + result.stderr)
