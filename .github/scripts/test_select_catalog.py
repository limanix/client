"""Check published catalog selection with local Git and a network-free API stub."""

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

import plan_eval
import select_catalog


def release(tag, **overrides):
    return {
        "tag_name": tag,
        "draft": False,
        "published_at": "2026-10-02T00:00:00Z",
        **overrides,
    }


class PublishedTagsTests(unittest.TestCase):
    def test_only_completed_exact_catalog_releases_are_eligible(self):
        records = [release("v9"), release("v10", prerelease=True)]
        records.extend(
            release(tag)
            for tag in ["v0", "v01", "v1.2.3", "v2+1", "main", "HEAD", "V3"]
        )
        records.extend(
            [
                release("v11", draft=True),
                release("v12", draft=0),
                {"tag_name": "v13", "published_at": "2026-10-02T00:00:00Z"},
                release("v14", published_at=None),
                release("v15", published_at=""),
                release("v16", published_at="   "),
                release("v17", published_at=42),
            ]
        )
        self.assertEqual(select_catalog.published_tags([records]), {"v9", "v10"})

    def test_pages_are_combined_and_duplicate_releases_are_deduplicated(self):
        pages = [[release("v9")], [], [release("v10"), release("v9")]]
        self.assertEqual(select_catalog.published_tags(pages), {"v9", "v10"})
        self.assertEqual(select_catalog.published_tags([]), set())

    def test_malformed_api_shapes_are_rejected(self):
        for pages in [None, {}, [{}], [[None]], [[[]]], [["v10"]]]:
            with self.subTest(pages=pages), self.assertRaises(ValueError):
                select_catalog.published_tags(pages)


class CatalogSelectionTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(
            prefix="limanix-catalog-test-", dir="/tmp"
        )
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.environment = {
            key: value
            for key, value in os.environ.items()
            if not key.startswith("GIT_")
        }
        self.environment.update(
            GIT_CONFIG_NOSYSTEM="1",
            GIT_CONFIG_GLOBAL=os.devnull,
            PYTHONDONTWRITEBYTECODE="1",
            GITHUB_STEP_SUMMARY="",
        )
        self.modules = self.repository("modules")
        self.client = self.repository("client")
        self.old_commit = self.commit(
            self.modules, "catalog/go/module.toml", 'versions = ["1.27"]\n'
        )
        self.git(self.modules, "tag", "v9")
        self.current_commit = self.commit(
            self.modules, "catalog/go/module.toml", 'versions = ["1.26", "1.27"]\n'
        )
        self.git(self.modules, "tag", "-a", "v10", "-m", "published catalog")
        self.client_commit = self.commit(
            self.client, plan_eval.EVALUATOR, plan_eval.SHARD_PROTOCOL + "\n"
        )
        self.git(self.client, "tag", "v1.2.3")
        self.git(self.client, "tag", "v10")
        self.pages = [[release("v9")], [release("v10")]]

        binary_directory = self.root / "bin"
        binary_directory.mkdir()
        api = binary_directory / "gh"
        api.write_text(
            f"#!{sys.executable}\n"
            "import json, os, pathlib, sys\n"
            "with pathlib.Path(os.environ['CATALOG_FIXTURE_CALLS']).open('a') as log:\n"
            "    log.write(json.dumps(sys.argv[1:]) + '\\n')\n"
            "sys.stdout.write(os.environ['CATALOG_FIXTURE_RESPONSE'])\n"
            "sys.stderr.write(os.environ.get('CATALOG_FIXTURE_ERROR', ''))\n"
            "sys.exit(int(os.environ.get('CATALOG_FIXTURE_STATUS', '0')))\n"
        )
        api.chmod(0o755)
        self.api_calls = self.root / "api-calls.jsonl"
        self.environment.update(
            PATH=str(binary_directory) + os.pathsep + self.environment.get("PATH", ""),
            CATALOG_FIXTURE_CALLS=str(self.api_calls),
            CATALOG_FIXTURE_RESPONSE=json.dumps(self.pages),
        )

    def repository(self, name):
        root = self.root / name
        root.mkdir()
        self.git(root, "init", "--quiet")
        return root

    def git(self, root, *arguments):
        return subprocess.check_output(
            [
                "git",
                "-c",
                "user.name=Fixture",
                "-c",
                "user.email=fixture@example.invalid",
                "-c",
                "core.hooksPath=/dev/null",
                "-c",
                "commit.gpgsign=false",
                "-c",
                "tag.gpgsign=false",
                "-c",
                "init.defaultBranch=fixture",
                "-C",
                str(root),
                *arguments,
            ],
            text=True,
            stderr=subprocess.PIPE,
            env=self.environment,
            timeout=10,
        ).strip()

    def commit(self, root, path, contents):
        target = root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(contents)
        self.git(root, "add", "--", path)
        self.git(root, "commit", "--quiet", "-m", "fixture")
        return self.git(root, "rev-parse", "HEAD")

    def invoke(self, *arguments, pages=None, response=None, status=0, modules=None):
        environment = self.environment.copy()
        if response is not None:
            environment["CATALOG_FIXTURE_RESPONSE"] = response
        elif pages is not None:
            environment["CATALOG_FIXTURE_RESPONSE"] = json.dumps(pages)
        environment["CATALOG_FIXTURE_STATUS"] = str(status)
        return subprocess.run(
            [
                sys.executable,
                "-B",
                select_catalog.__file__,
                "--modules-root",
                str(self.modules if modules is None else modules),
                *arguments,
            ],
            cwd=self.client,
            env=environment,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=10,
            check=False,
        )

    def assert_rejected(self, result):
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(result.stdout, "", "Failed selections must emit no outputs")
        self.assertTrue(result.stderr)
        self.assertNotIn("Traceback", result.stderr)

    def select(self, requested_tag="", pages=None):
        with (
            patch.dict(os.environ, self.environment, clear=True),
            contextlib.chdir(self.client),
        ):
            return select_catalog.select_catalog(
                self.modules, requested_tag, self.pages if pages is None else pages
            )

    def test_numeric_default_uses_modules_tags_and_peels_annotated_tag(self):
        self.assertNotEqual(
            self.git(self.modules, "rev-parse", "refs/tags/v10"), self.current_commit
        )
        self.assertNotEqual(self.client_commit, self.current_commit)
        self.assertEqual(self.select(), ("v10", self.current_commit))

    def test_explicit_older_published_release_keeps_its_exact_commit(self):
        self.assertEqual(self.select("v9"), ("v9", self.old_commit))

    def test_client_semver_and_unrelated_tags_do_not_hide_published_catalogs(self):
        self.git(self.client, "tag", "-d", "v10")
        self.git(self.client, "tag", "v5000")
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            result.stdout,
            f"modules-tag=v10\ncatalog-commit-sha={self.current_commit}\n",
        )

    def test_unpublished_head_does_not_replace_selected_catalog_commit(self):
        head = self.commit(self.modules, "unpublished.txt", "later local source\n")
        self.git(self.modules, "tag", "v20")
        self.assertNotEqual(head, self.current_commit)
        self.assertEqual(self.select(), ("v10", self.current_commit))

    def test_cli_emits_only_verified_tag_and_commit_from_modules_checkout(self):
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            result.stdout,
            f"modules-tag=v10\ncatalog-commit-sha={self.current_commit}\n",
        )
        calls = [json.loads(line) for line in self.api_calls.read_text().splitlines()]
        self.assertEqual(
            calls,
            [
                [
                    "api",
                    "--paginate",
                    "--slurp",
                    "repos/limanix/modules/releases?per_page=100",
                ]
            ],
        )

    def test_missing_fetched_tag_does_not_fall_back_to_branch_or_older_release(self):
        self.git(self.modules, "branch", "v11")
        pages = [self.pages[0], [release("v10"), release("v11")]]
        for arguments in [(), ("--modules-tag", "v11")]:
            with self.subTest(arguments=arguments):
                self.assert_rejected(self.invoke(*arguments, pages=pages))

    def test_explicit_tag_requires_completed_publication_without_fallback(self):
        for record in [release("v9", draft=True), release("v9", published_at=None)]:
            with self.subTest(record=record):
                self.assert_rejected(
                    self.invoke("--modules-tag", "v9", pages=[[record, release("v10")]])
                )
        self.assert_rejected(
            self.invoke("--modules-tag", "v9", pages=[[release("v10")]])
        )

    def test_invalid_explicit_tags_fail_before_api_lookup_without_fallback(self):
        for tag in ["main", "HEAD", "latest", "v0", "v01", "v1.2.3", "v10;echo unsafe"]:
            with self.subTest(tag=tag):
                self.assert_rejected(self.invoke("--modules-tag", tag))
        self.assertFalse(self.api_calls.exists())

    def test_no_published_catalog_fails_without_outputs(self):
        for pages in [[], [[]], [[release("v10", draft=True)]], [[release("v1.2.3")]]]:
            with self.subTest(pages=pages):
                self.assert_rejected(self.invoke(pages=pages))

    def test_missing_nonrepository_and_nested_paths_are_rejected(self):
        empty = self.root / "not-a-repository"
        empty.mkdir()
        for modules in [self.root / "missing", empty, self.modules / "catalog"]:
            with self.subTest(modules=modules):
                self.assert_rejected(self.invoke(modules=modules))

    def test_bad_api_json_and_record_shapes_fail_without_outputs(self):
        for response in ["{", "null", "{}", '[{"tag_name":"v10"}]', "[[null]]"]:
            with self.subTest(response=response):
                self.assert_rejected(self.invoke(response=response))

    def test_failed_api_command_cannot_emit_selection_outputs(self):
        self.assert_rejected(self.invoke(status=23))

    def test_api_lookup_has_a_bounded_timeout(self):
        output = io.StringIO()
        with (
            patch.dict(os.environ, self.environment, clear=True),
            patch.object(
                sys,
                "argv",
                [select_catalog.__file__, "--modules-root", str(self.modules)],
            ),
            patch.object(subprocess, "run", wraps=subprocess.run) as calls,
            contextlib.redirect_stdout(output),
            contextlib.redirect_stderr(io.StringIO()),
        ):
            select_catalog.main()
        api_calls = [
            call
            for call in calls.call_args_list
            if call.args and call.args[0][0] == "gh"
        ]
        self.assertEqual(len(api_calls), 1)
        self.assertEqual(api_calls[0].kwargs["timeout"], 30)
        self.assertEqual(
            output.getvalue(),
            f"modules-tag=v10\ncatalog-commit-sha={self.current_commit}\n",
        )

    def test_selected_older_catalog_outputs_feed_the_complete_pair_matrix(self):
        selected = self.invoke("--modules-tag", "v9")
        self.assertEqual(selected.returncode, 0, selected.stderr)
        outputs = dict(line.split("=", 1) for line in selected.stdout.splitlines())
        self.assertEqual(outputs["catalog-commit-sha"], self.old_commit)
        result = subprocess.run(
            [
                sys.executable,
                "-B",
                plan_eval.__file__,
                "--modules-root",
                str(self.modules),
                "--modules-tag",
                outputs["modules-tag"],
                "--client-root",
                str(self.client),
                "--commit-sha",
                self.client_commit,
            ],
            cwd=self.client,
            env=self.environment,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=10,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        planned = dict(line.split("=", 1) for line in result.stdout.splitlines())
        self.assertEqual(planned["modules-tag"], "v9")
        jobs = json.loads(planned["matrix"])["include"]
        for arch, runner in plan_eval.ARCHITECTURES:
            selected = [job for job in jobs if job["arch"] == arch]
            self.assertTrue(all(job["runner"] == runner for job in selected))
            self.assertCountEqual(
                [case for job in selected for case in job["cases"].split()],
                ["empty", "third-party-capability", "go", "go-1.27"],
            )


if __name__ == "__main__":
    unittest.main()
