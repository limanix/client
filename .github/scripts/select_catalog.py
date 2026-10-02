"""Resolve one published catalog tag from its own checkout."""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from pathlib import Path

CATALOG_TAG = re.compile(r"v[1-9][0-9]*")
CATALOG_REPOSITORY = "limanix/modules"


def published_tags(pages: list[list[dict]]) -> set[str]:
    if not isinstance(pages, list) or any(not isinstance(page, list) for page in pages):
        raise ValueError("Expected paginated catalog release records")
    tags = set()
    for page in pages:
        for release in page:
            if not isinstance(release, dict):
                raise ValueError("Invalid catalog release record")
            tag = release.get("tag_name")
            published_at = release.get("published_at")
            if (
                isinstance(tag, str)
                and CATALOG_TAG.fullmatch(tag)
                and release.get("draft") is False
                and isinstance(published_at, str)
                and published_at.strip()
            ):
                tags.add(tag)
    return tags


def catalog_git(root: Path, *arguments: str) -> str:
    try:
        return subprocess.check_output(
            ["git", "--no-optional-locks", "-C", str(root), *arguments],
            stderr=subprocess.PIPE,
            text=True,
            timeout=15,
        ).strip()
    except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as error:
        raise ValueError(
            "Cannot resolve the catalog checkout or requested tag"
        ) from error


def select_catalog(
    modules_root: Path, requested_tag: str, releases: list[list[dict]]
) -> tuple[str, str]:
    if requested_tag and not CATALOG_TAG.fullmatch(requested_tag):
        raise ValueError("An exact catalog tag such as v1 is required")
    tags = published_tags(releases)
    if requested_tag:
        if requested_tag not in tags:
            raise ValueError(f"Catalog {requested_tag} has no published release")
        tag = requested_tag
    else:
        if not tags:
            raise ValueError("No published catalog release is available")
        tag = max(tags, key=lambda value: int(value[1:]))
    actual_root = Path(catalog_git(modules_root, "rev-parse", "--show-toplevel"))
    if actual_root.resolve() != modules_root.resolve():
        raise ValueError("The catalog checkout must be a repository root")
    commit = catalog_git(
        modules_root, "rev-parse", "--verify", f"refs/tags/{tag}^{{commit}}"
    )
    if not re.fullmatch(r"[0-9a-f]{40,64}", commit):
        raise ValueError("The catalog tag must resolve to an exact commit")
    return tag, commit


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--modules-root", type=Path, required=True)
    parser.add_argument("--modules-tag", default="")
    args = parser.parse_args()
    if args.modules_tag and not CATALOG_TAG.fullmatch(args.modules_tag):
        raise ValueError("An exact catalog tag such as v1 is required")
    response = subprocess.check_output(
        [
            "gh",
            "api",
            "--paginate",
            "--slurp",
            f"repos/{CATALOG_REPOSITORY}/releases?per_page=100",
        ],
        text=True,
        timeout=30,
    )
    tag, commit = select_catalog(
        args.modules_root, args.modules_tag, json.loads(response)
    )
    print(f"modules-tag={tag}")
    print(f"catalog-commit-sha={commit}")
    print(f"Selected published catalog {tag} at {commit}", file=sys.stderr)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.SubprocessError) as error:
        print(f"Catalog selection failed: {error}", file=sys.stderr)
        sys.exit(1)
