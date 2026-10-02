"""Plan stage-specific client/catalog coverage from exact Git revisions."""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import tomllib
from pathlib import Path

SHARD_SIZE = 4
VERSION_CASES_PER_SHARD = 2
PR_CASE_LIMIT = 8
SHARD_PROTOCOL = "// limanix-eval-shard-protocol: 1"
EVALUATOR = "internal/nixos/generated_eval_test.go"
ARCHITECTURES = [("amd64", "ubuntu-24.04"), ("arm64", "ubuntu-24.04-arm")]


def git(root: Path, *arguments: str) -> bytes:
    return subprocess.check_output(
        ["git", "--no-optional-locks", "-C", str(root), *arguments],
        stderr=subprocess.PIPE,
        timeout=15,
    )


def tree(root: Path, revision: str, path: str) -> list[tuple[str, str]]:
    records = []
    for record in git(root, "ls-tree", "-r", "-z", revision, "--", path).split(b"\0"):
        if not record:
            continue
        header, name = record.split(b"\t", 1)
        mode, kind, _ = header.decode().split()
        if kind != "blob" or mode not in ("100644", "100755"):
            raise ValueError(f"Unsupported Git entry: {name.decode()}")
        records.append((mode, name.decode()))
    return records


def catalog_cases(root: Path, tag: str) -> list[str]:
    if not re.fullmatch(r"v[1-9][0-9]*", tag):
        raise ValueError("A published catalog tag is required")
    revision = f"refs/tags/{tag}"
    selectors = []
    directories = set()
    metadata_names = set()
    for _, path in tree(root, revision, "catalog/"):
        parts = path.split("/")
        if len(parts) < 3 or parts[1] == "_shared":
            continue
        name = parts[1]
        if not re.fullmatch(r"[a-z][a-z0-9]*(-[a-z0-9]+)*", name) or len(name) > 63:
            raise ValueError(f"Invalid catalog directory: {name}")
        directories.add(name)
        if len(parts) != 3 or parts[2] != "module.toml":
            continue
        metadata = tomllib.loads(git(root, "show", f"{revision}:{path}").decode())
        metadata_names.add(name)
        versions = metadata.get("versions", [])
        if not isinstance(versions, list) or any(
            not isinstance(version, str)
            or not re.fullmatch(r"[0-9]+(\.[0-9]+)*", version)
            or len(version) > 63
            for version in versions
        ):
            raise ValueError(f"Invalid versions in {path}")
        selectors.append(name)
        selectors.extend(f"{name}-{version}" for version in versions)
    if directories != metadata_names:
        raise ValueError(
            "Catalog directories require module.toml: "
            + ", ".join(sorted(directories - metadata_names))
        )
    if not selectors or len(selectors) != len(set(selectors)):
        raise ValueError("Catalog selectors must be nonempty and unique")
    cases = ["empty", "third-party-capability", *sorted(selectors)]
    if {"console", "astronvim-6", "go"} <= set(selectors):
        cases.append("console-version")
    if len(cases) != len(set(cases)):
        raise ValueError("Catalog selectors overlap reserved evaluator cases")
    return cases


def supports_sharding(root: Path, commit: str) -> bool:
    if not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{40,64}", commit):
        raise ValueError("An exact client commit SHA is required")
    git(root, "cat-file", "-e", f"{commit}^{{commit}}")
    if not tree(root, commit, EVALUATOR):
        # The evaluator task reports a missing test explicitly.
        return False
    return SHARD_PROTOCOL in git(root, "show", f"{commit}:{EVALUATOR}").decode()


def version_case(case: str) -> bool:
    return re.search(r"-[0-9]+(?:\.[0-9]+)*$", case) is not None


def version_bearing_case(case: str) -> bool:
    # The integration case also selects an older editor line.
    return version_case(case) or case == "console-version"


def select_cases(cases: list[str], profile: str) -> list[str]:
    if profile == "full":
        return cases
    if profile != "pr":
        raise ValueError("Evaluation profile must be full or pr")
    selected = ["empty", "third-party-capability"]
    if not set(selected) <= set(cases):
        raise ValueError("PR coverage requires empty and third-party cases")
    critical = ["cozy", "console", "console-version", "docker", "minikube", "astronvim"]
    selected.extend(case for case in critical if case in cases)
    fallback = ["go", *sorted(case for case in cases if not version_case(case))]
    for case in fallback:
        if len(selected) == PR_CASE_LIMIT:
            break
        if case in cases and case not in selected:
            selected.append(case)
    if (
        not set(critical).intersection(cases) <= set(selected)
        or len(selected) > PR_CASE_LIMIT
    ):
        raise ValueError("Incomplete PR representative coverage")
    return selected


def matrix(cases: list[str], sharded: bool) -> dict:
    if not sharded:
        # Older sources evaluate the whole pair once, including both architectures.
        return {
            "include": [
                {
                    "id": "legacy-full-pair",
                    "cases": "",
                    "arch": "",
                    "runner": "ubuntu-24.04",
                }
            ]
        }
    # Balance historical source downloads without parsing private Nix code.
    # Every case remains selected; ordinary cases fill unused worker capacity.
    historical = [case for case in cases if version_bearing_case(case)]
    ordinary = [case for case in cases if not version_bearing_case(case)]
    chunks = []
    while historical or ordinary:
        selected = historical[:VERSION_CASES_PER_SHARD]
        del historical[:VERSION_CASES_PER_SHARD]
        available = SHARD_SIZE - len(selected)
        selected.extend(ordinary[:available])
        del ordinary[:available]
        chunks.append({"id": f"{len(chunks):02}", "cases": " ".join(selected)})
    jobs = [
        dict(chunk, arch=arch, runner=runner)
        for arch, runner in ARCHITECTURES
        for chunk in chunks
    ]
    for arch, _ in ARCHITECTURES:
        covered = [
            case for job in jobs if job["arch"] == arch for case in job["cases"].split()
        ]
        if sorted(covered) != sorted(cases) or len(covered) != len(set(covered)):
            raise ValueError(f"Incomplete evaluation matrix for {arch}")
    for job in jobs:
        selected = job["cases"].split()
        if (
            len(selected) > SHARD_SIZE
            or sum(map(version_bearing_case, selected)) > VERSION_CASES_PER_SHARD
        ):
            raise ValueError("Evaluation worker exceeds its case budget")
    if len(jobs) > 256:
        raise ValueError(
            "Evaluation plan exceeds the GitHub matrix limit; increase shard size"
        )
    return {"include": jobs}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--modules-root", type=Path, required=True)
    parser.add_argument("--modules-tag", required=True)
    parser.add_argument("--client-root", type=Path, default=Path("."))
    parser.add_argument("--profile", choices=["full", "pr"], default="full")
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--commit-sha")
    source.add_argument("--clients-json")
    args = parser.parse_args()
    cases = catalog_cases(args.modules_root, args.modules_tag)
    if args.clients_json is None:
        coverage = matrix(
            select_cases(cases, args.profile),
            supports_sharding(args.client_root, args.commit_sha),
        )
        print("modules-tag=" + args.modules_tag)
        print("matrix=" + json.dumps(coverage, separators=(",", ":")))
    else:
        if args.profile != "full":
            raise ValueError("Historical release planning requires full coverage")
        clients = json.loads(args.clients_json)
        if not isinstance(clients, list) or any(
            not isinstance(client, dict) for client in clients
        ):
            raise ValueError("Client selection must be an array of release records")
        if len(clients) > 256:
            raise ValueError("Client selection exceeds the GitHub matrix limit")
        tags = set()
        for client in clients:
            tag = client.get("tag", "")
            if (
                not isinstance(tag, str)
                or not re.fullmatch(
                    r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(\+[1-9][0-9]*)?",
                    tag,
                )
                or tag in tags
            ):
                raise ValueError("Client release tags must be valid and unique")
            tags.add(tag)
            client["evaluation_matrix"] = matrix(
                cases, supports_sharding(args.client_root, client.get("commit_sha", ""))
            )
        print("tags=" + json.dumps(clients, separators=(",", ":")))
    summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary:
        with open(summary, "a") as output:
            count = len(select_cases(cases, args.profile))
            output.write(
                f"Catalog: `{args.modules_tag}`. Profile: `{args.profile}`. "
                f"Catalog cases: {len(cases)}; selected shard cases: {count}. "
                "Legacy sources retain one complete unfiltered evaluator.\n"
            )


if __name__ == "__main__":
    main()
