"""Verify pair coverage and exact-history selection using disposable Git fixtures."""

import contextlib
import io
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import plan_eval


class PairPlannerTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.modules = self.repository("modules")
        self.client = self.repository("client")
        self.catalog({"astronvim": ["6"], "console": [], "go": ["1.26", "1.27"]})
        self.legacy = self.commit(
            self.client, plan_eval.EVALUATOR, "// previous evaluator\n"
        )
        self.current = self.commit(
            self.client, plan_eval.EVALUATOR, plan_eval.SHARD_PROTOCOL + "\n"
        )

    def repository(self, name):
        root = self.root / name
        root.mkdir()
        self.command(root, "init", "--quiet")
        return root

    def command(self, root, *arguments):
        environment = {
            key: value
            for key, value in os.environ.items()
            if not key.startswith("GIT_")
        }
        environment.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull)
        return (
            subprocess.check_output(
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
                stderr=subprocess.PIPE,
                env=environment,
                timeout=10,
            )
            .decode()
            .strip()
        )

    def commit(self, root, path, contents):
        target = root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(contents)
        self.command(root, "add", "--", path)
        self.command(root, "commit", "--quiet", "-m", "fixture")
        return self.command(root, "rev-parse", "HEAD")

    def catalog(self, modules):
        shutil.rmtree(self.modules / "catalog", ignore_errors=True)
        for name, versions in modules.items():
            target = self.modules / "catalog" / name / "module.toml"
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(
                'description = "fixture"\nversions = ' + json.dumps(versions) + "\n"
            )
        self.command(self.modules, "add", "-A", "--", "catalog")
        self.command(self.modules, "commit", "--quiet", "-m", "catalog")
        self.command(self.modules, "tag", "-f", "v9")

    def test_current_matrix_covers_each_case_once_on_both_architectures(self):
        cases = plan_eval.catalog_cases(self.modules, "v9")
        self.assertEqual(
            cases,
            [
                "empty",
                "third-party-capability",
                "astronvim",
                "astronvim-6",
                "console",
                "go",
                "go-1.26",
                "go-1.27",
                "console-version",
            ],
        )
        jobs = plan_eval.matrix(cases, True)["include"]
        self.assertEqual(len(jobs), 6)
        for arch, runner in plan_eval.ARCHITECTURES:
            selected = [job for job in jobs if job["arch"] == arch]
            self.assertCountEqual(
                [case for job in selected for case in job["cases"].split()], cases
            )
            self.assertTrue(
                all(
                    job["runner"] == runner
                    and len(job["cases"].split()) <= 4
                    and sum(map(plan_eval.version_bearing_case, job["cases"].split()))
                    <= 2
                    for job in selected
                )
            )

    def test_legacy_source_runs_one_complete_unfiltered_pair(self):
        self.assertFalse(plan_eval.supports_sharding(self.client, self.legacy))
        jobs = plan_eval.matrix(["empty"], False)["include"]
        self.assertEqual(len(jobs), 1)
        self.assertEqual(jobs[0]["id"], "legacy-full-pair")
        self.assertEqual(jobs[0]["arch"], "")
        self.assertEqual(jobs[0]["cases"], "")
        self.assertTrue(plan_eval.supports_sharding(self.client, self.current))

    def test_dispatch_plans_each_historical_source_without_checkout(self):
        clients = [
            {"tag": "v1.0.0+2", "commit_sha": self.legacy},
            {"tag": "v2.0.0+1", "commit_sha": self.current},
        ]
        arguments = [
            "plan_eval.py",
            "--modules-root",
            str(self.modules),
            "--modules-tag",
            "v9",
            "--client-root",
            str(self.client),
            "--clients-json",
            json.dumps(clients),
        ]
        output = io.StringIO()
        with (
            patch.object(sys, "argv", arguments),
            patch.dict(os.environ, {"GITHUB_STEP_SUMMARY": ""}),
            contextlib.redirect_stdout(output),
        ):
            plan_eval.main()
        planned = json.loads(output.getvalue().removeprefix("tags="))
        self.assertEqual(len(planned[0]["evaluation_matrix"]["include"]), 1)
        self.assertEqual(len(planned[1]["evaluation_matrix"]["include"]), 6)
        self.assertEqual(self.command(self.client, "rev-parse", "HEAD"), self.current)

    def test_invalid_names_versions_and_duplicate_selectors_are_rejected(self):
        for name, versions in [
            ("bad;name", []),
            ("go", ["1.27", "1.27"]),
            ("go", ["a"]),
            ("go", [27]),
        ]:
            with self.subTest(name=name, versions=versions):
                self.catalog({name: versions})
                with self.assertRaises(ValueError):
                    plan_eval.catalog_cases(self.modules, "v9")

    def test_catalog_symlink_is_rejected_before_metadata_is_followed(self):
        symlink = self.modules / "catalog" / "linked"
        symlink.symlink_to("go", target_is_directory=True)
        self.command(self.modules, "add", "--", "catalog/linked")
        self.command(self.modules, "commit", "--quiet", "-m", "symlink")
        self.command(self.modules, "tag", "-f", "v9")
        with self.assertRaisesRegex(ValueError, "Unsupported Git entry"):
            plan_eval.catalog_cases(self.modules, "v9")

    def test_missing_metadata_and_reserved_case_names_are_rejected(self):
        self.commit(self.modules, "catalog/undocumented/default.nix", "{}\n")
        self.command(self.modules, "tag", "-f", "v9")
        with self.assertRaisesRegex(ValueError, "require module.toml"):
            plan_eval.catalog_cases(self.modules, "v9")
        self.catalog({"empty": []})
        with self.assertRaisesRegex(ValueError, "reserved evaluator cases"):
            plan_eval.catalog_cases(self.modules, "v9")

    def test_invalid_revisions_and_oversized_matrix_are_rejected(self):
        for tag in ["main", "v0", "v9;touch marker"]:
            with self.assertRaises(ValueError):
                plan_eval.catalog_cases(self.modules, tag)
        with self.assertRaises(ValueError):
            plan_eval.supports_sharding(self.client, "HEAD")
        self.assertEqual(
            len(
                plan_eval.matrix([f"module-{index}" for index in range(256)], True)[
                    "include"
                ]
            ),
            256,
        )
        with self.assertRaisesRegex(ValueError, "matrix limit"):
            plan_eval.matrix([f"module-{index}" for index in range(257)], True)
        self.assertEqual(
            len(
                plan_eval.matrix([f"module{index}" for index in range(512)], True)[
                    "include"
                ]
            ),
            256,
        )
        with self.assertRaisesRegex(ValueError, "matrix limit"):
            plan_eval.matrix([f"module{index}" for index in range(513)], True)

    def test_balanced_release_preserves_all_122_evaluations_in_34_jobs(self):
        cases = [
            "empty",
            "third-party-capability",
            "console-version",
            *[f"module{index}" for index in range(25)],
            *[f"module{index % 25}-{index // 25 + 1}.0" for index in range(33)],
        ]
        jobs = plan_eval.matrix(cases, True)["include"]
        self.assertEqual(len(jobs), 34)
        self.assertEqual(sum(len(job["cases"].split()) for job in jobs), 122)
        self.assertEqual(plan_eval.matrix(cases, True)["include"], jobs)
        for job in jobs:
            self.assertLessEqual(len(job["cases"].split()), 4)
            self.assertLessEqual(
                sum(map(plan_eval.version_bearing_case, job["cases"].split())), 2
            )
        for arch, _ in plan_eval.ARCHITECTURES:
            self.assertCountEqual(
                [
                    case
                    for job in jobs
                    if job["arch"] == arch
                    for case in job["cases"].split()
                ],
                cases,
            )

    def test_pr_critical_cases_fit_four_native_workers(self):
        critical = [
            "cozy",
            "console",
            "console-version",
            "docker",
            "minikube",
            "astronvim",
        ]
        cases = ["empty", "third-party-capability", *critical, "go", "go-1.26"]
        selected = plan_eval.select_cases(cases, "pr")
        self.assertEqual(selected, ["empty", "third-party-capability", *critical])
        jobs = plan_eval.matrix(selected, True)["include"]
        self.assertEqual(len(jobs), 4)
        self.assertEqual(sum(len(job["cases"].split()) for job in jobs), 16)
        self.assertEqual(plan_eval.select_cases(cases, "full"), cases)

    def test_pr_older_catalog_uses_available_critical_and_default_fallback(self):
        cases = [
            "empty",
            "third-party-capability",
            "go",
            "go-1.26",
            "console",
            "yazi",
            "zsh",
            "tmux",
            "fzf",
            "jq",
        ]
        selected = plan_eval.select_cases(cases, "pr")
        self.assertEqual(len(selected), 8)
        self.assertEqual(
            selected[:4], ["empty", "third-party-capability", "console", "go"]
        )
        self.assertNotIn("go-1.26", selected)
        self.assertEqual(len(selected), len(set(selected)))

    def test_pr_missing_base_or_unknown_profile_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "empty and third-party"):
            plan_eval.select_cases(["empty", "go"], "pr")
        with self.assertRaisesRegex(ValueError, "full or pr"):
            plan_eval.select_cases(["empty"], "unknown")

    def test_cli_defaults_to_full_and_pr_does_not_weaken_historical_release(self):
        arguments = [
            "plan_eval.py",
            "--modules-root",
            str(self.modules),
            "--modules-tag",
            "v9",
            "--client-root",
            str(self.client),
            "--commit-sha",
            self.current,
        ]
        output = io.StringIO()
        with (
            patch.object(sys, "argv", arguments),
            patch.dict(os.environ, {"GITHUB_STEP_SUMMARY": ""}),
            contextlib.redirect_stdout(output),
        ):
            plan_eval.main()
        coverage = json.loads(output.getvalue().split("matrix=", 1)[1])
        self.assertEqual(
            sum(len(job["cases"].split()) for job in coverage["include"]), 18
        )
        historical = arguments[:-2] + ["--clients-json", "[]", "--profile", "pr"]
        with (
            patch.object(sys, "argv", historical),
            self.assertRaisesRegex(ValueError, "full coverage"),
        ):
            plan_eval.main()

    def test_legacy_pr_retains_complete_unfiltered_pair(self):
        cases = plan_eval.catalog_cases(self.modules, "v9")
        jobs = plan_eval.matrix(plan_eval.select_cases(cases, "pr"), False)["include"]
        self.assertEqual(len(jobs), 1)
        self.assertEqual(jobs[0]["id"], "legacy-full-pair")
        self.assertEqual(jobs[0]["cases"], "")
        self.assertEqual(jobs[0]["arch"], "")


if __name__ == "__main__":
    unittest.main()
