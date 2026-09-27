"""Exercise release planning and publication records without remote mutations."""

from __future__ import annotations

import contextlib
import io
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from scripts import release


class ReleaseTests(unittest.TestCase):
    def setUp(self) -> None:
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        previous = Path.cwd()
        os.chdir(self.root)
        self.addCleanup(os.chdir, previous)
        environment = patch.dict(
            os.environ,
            {"RUNNER_TEMP": str(self.root), "TMPDIR": str(self.root)},
            clear=True,
        )
        environment.start()
        self.addCleanup(environment.stop)

    def outputs(self, action) -> dict[str, str]:
        output = self.root / "outputs"
        output.write_text("", encoding="utf-8")
        with patch.dict(os.environ, {"GITHUB_OUTPUT": str(output)}):
            action()
        return dict(line.split("=", 1) for line in output.read_text().splitlines())

    def git(self, *args: str) -> str:
        return subprocess.run(
            ["git", *args], check=True, capture_output=True, text=True
        ).stdout.strip()

    def repository(self) -> tuple[str, str]:
        self.git("init", "--quiet")
        self.git("config", "user.name", "Release tests")
        self.git("config", "user.email", "release-tests@example.invalid")
        self.git("commit", "--quiet", "--allow-empty", "-m", "First")
        first = self.git("rev-parse", "HEAD")
        self.git("tag", "v1.2.3")
        self.git("commit", "--quiet", "--allow-empty", "-m", "Second")
        return first, self.git("rev-parse", "HEAD")

    def write_receipt(self, filename: str, client: str, modules: str) -> None:
        directory = self.root / "docs-releases"
        directory.mkdir(exist_ok=True)
        (directory / filename).write_text(
            json.dumps({"client_tag": client, "modules_tag": modules}), encoding="utf-8"
        )

    def test_module_pin_comes_from_checked_out_taskfile(self) -> None:
        (self.root / "Taskfile.yml").write_text("vars:\n  modules_version: 'v4'\n")
        with patch.dict(os.environ, {"CLIENT_TAG": "v1.2.3"}):
            self.assertEqual(self.outputs(release.modules), {"tag": "v4"})

    def test_modules_event_uses_supplied_catalog(self) -> None:
        with patch.dict(os.environ, {"MODULES_TAG": "v7"}):
            self.assertEqual(self.outputs(release.modules), {"tag": "v7"})

    def test_modules_requires_one_source(self) -> None:
        for values in ({}, {"CLIENT_TAG": "v1.2.3", "MODULES_TAG": "v7"}):
            with self.subTest(values=values), patch.dict(os.environ, values):
                with self.assertRaisesRegex(ValueError, "exactly one"):
                    release.modules()

    def test_missing_or_duplicate_module_pin_fails(self) -> None:
        for text in ("vars:\n", "  modules_version: 'v4'\n  modules_version: 'v5'\n"):
            with self.subTest(text=text), patch.dict(os.environ, {"CLIENT_TAG": "v1.2.3"}):
                (self.root / "Taskfile.yml").write_text(text)
                with self.assertRaisesRegex(ValueError, "one modules_version"):
                    release.modules()

    def test_module_release_must_match_and_be_published_stable(self) -> None:
        valid = {"tag_name": "v7", "draft": False, "prerelease": False, "published_at": "date"}
        with patch.dict(os.environ, {"MODULES_TAG": "v7"}):
            with patch.object(release, "command", return_value=json.dumps(valid)) as command:
                release.verify_modules()
                command.assert_called_once_with("gh", "api", "repos/limanix/modules/releases/tags/v7")
            for change in (
                {"tag_name": "v6"}, {"draft": True}, {"prerelease": True},
                {"published_at": None}, {"draft": 0},
            ):
                with self.subTest(change=change), patch.object(
                    release, "command", return_value=json.dumps(valid | change)
                ):
                    with self.assertRaisesRegex(ValueError, "published stable"):
                        release.verify_modules()

    def test_tag_plan_uses_checked_out_commit_without_rebuild(self) -> None:
        first, _ = self.repository()
        self.git("checkout", "--quiet", "v1.2.3")
        with patch.dict(os.environ, {"CLIENT_TAG": "v1.2.3", "MODULES_TAG": "v4"}):
            result = json.loads(self.outputs(release.plan)["releases"])
        self.assertEqual(result, [{
            "source_tag": "v1.2.3", "commit_sha": first,
            "release_tag": "v1.2.3", "error": "",
        }])

    def test_event_plan_preserves_other_rows_on_invalid_version(self) -> None:
        tags = [
            {"tag": tag, "commit_sha": f"commit-{index}"}
            for index, tag in enumerate(("v1.3.0", "v1.2.4+10", "v1.2.3+0", "v1.2.2+01", "v1.2.1-alpha"))
        ]
        with patch.dict(os.environ, {"MODULES_TAG": "v7", "TAGS": json.dumps(tags)}):
            result = json.loads(self.outputs(release.plan)["releases"])
        self.assertEqual([row["release_tag"] for row in result], ["v1.3.0+1", "v1.2.4+11", "", "", ""])
        self.assertEqual([row["commit_sha"] for row in result], [row["commit_sha"] for row in tags])
        self.assertTrue(all(row["error"] for row in result[2:]))

    def test_rebuild_counter_keeps_signed_64_bit_limit(self) -> None:
        self.assertEqual(
            release.next_release(f"v1.2.3+{release.MAX_REBUILD - 1}"),
            f"v1.2.3+{release.MAX_REBUILD}",
        )
        for counter in (str(release.MAX_REBUILD), str(release.MAX_REBUILD + 1), "9" * 5000):
            with self.subTest(digits=len(counter)):
                with self.assertRaisesRegex(ValueError, "too large"):
                    release.next_release(f"v1.2.3+{counter}")

    def test_event_plan_keeps_overflow_as_item_error_and_allows_empty_selection(self) -> None:
        tags = [
            {"tag": f"v1.3.0+{release.MAX_REBUILD}", "commit_sha": "first"},
            {"tag": "v1.2.3", "commit_sha": "second"},
        ]
        with patch.dict(os.environ, {"MODULES_TAG": "v7", "TAGS": json.dumps(tags)}):
            result = json.loads(self.outputs(release.plan)["releases"])
        self.assertEqual(result[0]["release_tag"], "")
        self.assertIn("too large", result[0]["error"])
        self.assertEqual(result[1]["release_tag"], "v1.2.3+1")
        with patch.dict(os.environ, {"MODULES_TAG": "v7", "TAGS": "[]"}):
            self.assertEqual(self.outputs(release.plan), {"releases": "[]"})

    def test_preparation_error_stops_verify_before_git(self) -> None:
        with patch.dict(os.environ, {"PREPARATION_ERROR": "invalid item"}):
            with patch.object(release, "command") as command:
                with self.assertRaisesRegex(ValueError, "invalid item"):
                    release.verify()
                command.assert_not_called()

    def test_verify_rejects_moved_source_tag(self) -> None:
        first, second = self.repository()
        self.git("tag", "--force", "v1.2.3", second)
        with patch.dict(os.environ, {"SOURCE_TAG": "v1.2.3", "SOURCE_SHA": first, "RELEASE_TAG": "v1.2.3+1"}):
            with self.assertRaisesRegex(ValueError, "Source tag no longer"):
                release.verify()

    def test_verify_allows_missing_or_same_commit_target_and_rejects_collision(self) -> None:
        first, second = self.repository()
        values = {"SOURCE_TAG": "v1.2.3", "SOURCE_SHA": first, "RELEASE_TAG": "v1.2.3+1"}
        with patch.dict(os.environ, values):
            release.verify()
            self.git("tag", "v1.2.3+1", first)
            release.verify()
            self.git("tag", "--force", "v1.2.3+1", second)
            with self.assertRaisesRegex(ValueError, "Target tag already"):
                release.verify()

    def test_receipt_records_exact_published_pair(self) -> None:
        with patch.dict(os.environ, {"CLIENT_TAG": "v1.2.3+4", "MODULES_TAG": "v7"}):
            release.receipt()
        self.assertEqual(
            json.loads((self.root / "docs-release-v1.2.3+4.json").read_text()),
            {"client_tag": "v1.2.3+4", "modules_tag": "v7"},
        )

    def test_invalid_receipt_does_not_write_file(self) -> None:
        with patch.dict(os.environ, {"CLIENT_TAG": "../invalid", "MODULES_TAG": "v7"}):
            with self.assertRaisesRegex(ValueError, "Invalid client tag"):
                release.receipt()
        self.assertEqual(list(self.root.iterdir()), [])

    def test_docs_event_deduplicates_and_sorts_published_pairs(self) -> None:
        self.write_receipt("one.json", "v1.3.0+1", "v7")
        self.write_receipt("two.json", "v1.2.3+4", "v7")
        self.write_receipt("retry.json", "v1.3.0+1", "v7")
        self.assertEqual(self.outputs(release.docs_event), {"ready": "true"})
        self.assertEqual(json.loads((self.root / "docs-event.json").read_text()), {
            "event_type": "limanix-client-release",
            "client_payload": {"releases": [
                {"client_tag": "v1.2.3+4", "modules_tag": "v7"},
                {"client_tag": "v1.3.0+1", "modules_tag": "v7"},
            ]},
        })

    def test_conflicting_receipts_fail_before_event_is_written(self) -> None:
        self.write_receipt("one.json", "v1.2.3+4", "v7")
        self.write_receipt("two.json", "v1.2.3+4", "v8")
        with self.assertRaisesRegex(ValueError, "Conflicting"):
            release.docs_event()
        self.assertFalse((self.root / "docs-event.json").exists())

    def test_event_rejects_extra_keys_and_invalid_tags(self) -> None:
        invalid = [
            {"client_tag": "v1.2.3", "modules_tag": "v7", "extra": True},
            {"client_tag": "v1.2.3+0", "modules_tag": "v7"},
            {"client_tag": "v1.2.3", "modules_tag": "v07"},
        ]
        directory = self.root / "docs-releases"
        directory.mkdir()
        for pair in invalid:
            with self.subTest(pair=pair):
                (directory / "one.json").write_text(json.dumps(pair))
                with self.assertRaises(ValueError):
                    release.docs_event()
                self.assertFalse((self.root / "docs-event.json").exists())

    def test_no_receipts_does_not_mark_event_ready(self) -> None:
        with contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(self.outputs(release.docs_event), {})
        self.assertFalse((self.root / "docs-event.json").exists())

    def test_latest_uses_selector_order_and_does_nothing_when_empty(self) -> None:
        tags = [
            {"tag": "v1.3.0+1", "commit_sha": "first"},
            {"tag": "v1.2.3+10", "commit_sha": "second"},
        ]
        with patch.dict(os.environ, {"TAGS": json.dumps(tags)}):
            with patch.object(release, "command") as command:
                release.latest()
                command.assert_called_once_with("gh", "release", "edit", "v1.3.0+1", "--latest")
        with patch.dict(os.environ, {"TAGS": "[]"}), patch.object(release, "command") as command:
            release.latest()
            command.assert_not_called()

    def test_cli_consumes_environment_and_reports_errors(self) -> None:
        script = str(Path(release.__file__).resolve())
        values = {"MODULES_TAG": "v7", "TAGS": '[{"tag":"v1.2.3","commit_sha":"source"}]'}
        with patch.dict(os.environ, values):
            result = subprocess.run(
                [sys.executable, script, "plan"], capture_output=True, text=True
            )
        self.assertEqual(result.returncode, 0, result.stderr)
        name, content = result.stdout.strip().split("=", 1)
        self.assertEqual(name, "releases")
        self.assertEqual(json.loads(content)[0]["release_tag"], "v1.2.3+1")
        result = subprocess.run(
            [sys.executable, script, "receipt"], capture_output=True, text=True
        )
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stdout, "")
        self.assertIn("release/receipt: Missing CLIENT_TAG", result.stderr)


if __name__ == "__main__":
    unittest.main()
