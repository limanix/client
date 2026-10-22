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
# Protocol 2 names cases after the kinds of the catalog; 1 after its flat module directories.
SHARD_PROTOCOLS = (
    "// limanix-eval-shard-protocol: 1",
    "// limanix-eval-shard-protocol: 2",
)
SHARD_PROTOCOL = SHARD_PROTOCOLS[-1]
KINDS = ("shell", "toolchains", "modules")
SHELL_CASES = [
    "shell-terminal",
    "shell-terminal-starship",
    "shell-terminal-bare",
    "shell-minimal-tools",
]
WORKBENCH_MODULES = {"docker", "gcloud", "claude", "posting"}
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
    paths = [path for _, path in tree(root, revision, "catalog/")]
    # Catalog v5 keeps each module below its kind: catalog/<kind>/<name>/.
    kinded = any(
        len(parts) >= 4 and parts[1] in KINDS
        for parts in (path.split("/") for path in paths)
    )
    modules = []
    directories = set()
    metadata_names = set()
    for path in paths:
        parts = path.split("/")
        if parts[1] == "_shared":
            continue
        if kinded:
            if len(parts) < 4:
                continue
            if parts[1] not in KINDS:
                raise ValueError(f"Invalid catalog kind: {parts[1]}")
            kind, name, rest = parts[1], parts[2], parts[3:]
        else:
            if len(parts) < 3:
                continue
            kind, name, rest = None, parts[1], parts[2:]
        if not re.fullmatch(r"[a-z][a-z0-9]*(-[a-z0-9]+)*", name) or len(name) > 63:
            raise ValueError(f"Invalid catalog directory: {name}")
        directories.add((kind, name))
        if rest != ["module.toml"]:
            continue
        metadata = tomllib.loads(git(root, "show", f"{revision}:{path}").decode())
        metadata_names.add((kind, name))
        versions = metadata.get("versions", [])
        if not isinstance(versions, list) or any(
            not isinstance(version, str)
            or not re.fullmatch(r"[0-9]+(\.[0-9]+)*", version)
            or len(version) > 63
            for version in versions
        ):
            raise ValueError(f"Invalid versions in {path}")
        modules.append((kind, name, versions))
    if directories != metadata_names:
        raise ValueError(
            "Catalog directories require module.toml: "
            + ", ".join(sorted(name for _, name in directories - metadata_names))
        )
    names = [name for _, name, _ in modules]
    if not names or len(names) != len(set(names)):
        raise ValueError("Catalog selectors must be nonempty and unique")
    cases = kinded_cases(modules) if kinded else flat_cases(modules)
    if len(cases) != len(set(cases)):
        raise ValueError("Catalog selectors overlap reserved evaluator cases")
    return cases


def flat_cases(modules: list[tuple[None, str, list[str]]]) -> list[str]:
    selectors = []
    for _, name, versions in modules:
        selectors.append(name)
        selectors.extend(f"{name}-{version}" for version in versions)
    cases = ["empty", "third-party-capability", *sorted(selectors)]
    if {"console", "astronvim-6", "go"} <= set(selectors):
        cases.append("console-version")
    return cases


def kinded_cases(modules: list[tuple[str, str, list[str]]]) -> list[str]:
    # Mirrors evaluationCases in internal/nixos/evaluation_cases_test.go.
    selectors = []
    for kind, name, versions in modules:
        if kind == "modules":
            selectors.append(name)
            selectors.extend(f"{name}-{version}" for version in versions)
        elif kind == "toolchains":
            # The client names a line when the toolchain has lines.
            selectors.extend(
                [f"{name}-{version}" for version in versions] if versions else [name]
            )
    cases = ["empty", "third-party-capability", *SHELL_CASES, *sorted(selectors)]
    if WORKBENCH_MODULES <= {name for kind, name, _ in modules if kind == "modules"}:
        cases.append("workbench")
    return cases


def supports_sharding(root: Path, commit: str) -> bool:
    if not isinstance(commit, str) or not re.fullmatch(r"[0-9a-f]{40,64}", commit):
        raise ValueError("An exact client commit SHA is required")
    git(root, "cat-file", "-e", f"{commit}^{{commit}}")
    if not tree(root, commit, EVALUATOR):
        # The evaluator task reports a missing test explicitly.
        return False
    source = git(root, "show", f"{commit}:{EVALUATOR}").decode()
    return any(protocol in source for protocol in SHARD_PROTOCOLS)


def version_case(case: str) -> bool:
    return re.search(r"-[0-9]+(?:\.[0-9]+)*$", case) is not None


def version_bearing_case(case: str) -> bool:
    # The integration cases also select versioned lines.
    return version_case(case) or case in ("console-version", "workbench")


def select_cases(cases: list[str], profile: str) -> list[str]:
    if profile == "full":
        return cases
    if profile != "pr":
        raise ValueError("Evaluation profile must be full or pr")
    selected = ["empty", "third-party-capability"]
    if not set(selected) <= set(cases):
        raise ValueError("PR coverage requires empty and third-party cases")
    if "shell-terminal" in cases:
        critical = ["shell-terminal", "workbench", "docker", "rust"]
    else:
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
