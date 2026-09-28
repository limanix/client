"""Prepare client release inputs and documentation publication records."""

from __future__ import annotations

import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path


CLIENT_TAG = re.compile(
    r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(\+[1-9][0-9]*)?"
)
MODULES_TAG = re.compile(r"v[1-9][0-9]*")
MAX_REBUILD = 2**63 - 1


def required(name: str) -> str:
    value = os.environ.get(name, "")
    if not value:
        raise ValueError(f"Missing {name}")
    return value


def output(name: str, value: str) -> None:
    line = f"{name}={value}\n"
    if path := os.environ.get("GITHUB_OUTPUT"):
        with Path(path).open("a", encoding="utf-8") as stream:
            stream.write(line)
    else:
        print(line, end="")


def command(*args: str) -> str:
    return subprocess.run(args, check=True, capture_output=True, text=True).stdout.strip()


def modules() -> None:
    tags = selected_tags()
    if not tags:
        raise ValueError("No published module catalog tag was found")
    tag = tags[0]["tag"]
    if not MODULES_TAG.fullmatch(tag):
        raise ValueError(f"Invalid module catalog tag: {tag!r}")
    output("tag", tag)


def verify_modules() -> None:
    tag = required("MODULES_TAG")
    release = json.loads(command("gh", "api", f"repos/limanix/modules/releases/tags/{tag}"))
    if not (
        isinstance(release, dict)
        and release.get("tag_name") == tag
        and release.get("draft") is False
        and release.get("prerelease") is False
        and release.get("published_at") is not None
    ):
        raise ValueError("Expected a published stable module release matching MODULES_TAG")


def selected_tags() -> list[dict[str, str]]:
    tags = json.loads(required("TAGS"))
    if not isinstance(tags, list) or any(
        not isinstance(item, dict)
        or not isinstance(item.get("tag"), str)
        or not isinstance(item.get("commit_sha"), str)
        for item in tags
    ):
        raise ValueError("Expected TAGS to be an array of tag/commit_sha objects")
    return tags


def next_release(tag: str) -> str:
    if not CLIENT_TAG.fullmatch(tag):
        raise ValueError("Invalid client source tag")
    base, separator, counter = tag.partition("+")
    if separator:
        # Keep the previous shell implementation's signed 64-bit counter limit.
        maximum = str(MAX_REBUILD)
        if len(counter) > len(maximum) or (
            len(counter) == len(maximum) and counter >= maximum
        ):
            raise ValueError("Rebuild counter is too large to increment")
        number = int(counter)
    else:
        number = 0
    return f"{base}+{number + 1}"


def plan() -> None:
    required("MODULES_TAG")
    client_tag = os.environ.get("CLIENT_TAG", "")
    tags = (
        [{"tag": client_tag, "commit_sha": command("git", "rev-parse", "HEAD^{commit}")}]
        if client_tag
        else selected_tags()
    )
    releases = []
    for item in tags:
        tag, error = item["tag"], ""
        try:
            release_tag = tag if client_tag else next_release(tag)
        except ValueError as failure:
            release_tag, error = "", str(failure)
        releases.append(
            {
                "source_tag": tag,
                "commit_sha": item["commit_sha"],
                "release_tag": release_tag,
                "error": error,
            }
        )
    output("releases", json.dumps(releases, separators=(",", ":")))


def verify() -> None:
    if error := os.environ.get("PREPARATION_ERROR", ""):
        raise ValueError(error)
    source_tag = required("SOURCE_TAG")
    source_sha = required("SOURCE_SHA")
    release_tag = required("RELEASE_TAG")
    tag_sha = command("git", "rev-parse", f"refs/tags/{source_tag}^{{commit}}")
    if tag_sha != source_sha:
        raise ValueError("Source tag no longer points to the selected commit")
    target_exists = subprocess.run(
        ["git", "show-ref", "--verify", "--quiet", f"refs/tags/{release_tag}"],
        check=False,
    ).returncode == 0
    if target_exists:
        target_sha = command("git", "rev-parse", f"refs/tags/{release_tag}^{{commit}}")
        if target_sha != source_sha:
            raise ValueError("Target tag already points to another commit")


def release_pair(value: object) -> dict[str, str]:
    if not isinstance(value, dict) or set(value) != {"client_tag", "modules_tag"}:
        raise ValueError("Expected an exact client_tag/modules_tag release pair")
    if not isinstance(value["client_tag"], str) or not CLIENT_TAG.fullmatch(value["client_tag"]):
        raise ValueError("Invalid client tag in publication record")
    if not isinstance(value["modules_tag"], str) or not MODULES_TAG.fullmatch(value["modules_tag"]):
        raise ValueError("Invalid modules tag in publication record")
    return value


def receipt() -> None:
    pair = release_pair(
        {"client_tag": required("CLIENT_TAG"), "modules_tag": required("MODULES_TAG")}
    )
    path = Path(required("RUNNER_TEMP")) / f"docs-release-{pair['client_tag']}.json"
    path.write_text(json.dumps(pair, separators=(",", ":")) + "\n", encoding="utf-8")


def docs_event() -> None:
    temporary = Path(required("RUNNER_TEMP"))
    receipts = sorted((temporary / "docs-releases").glob("*.json"))
    if not receipts:
        print("No published docs versions in this run; skipping docs event.")
        return
    releases: dict[str, str] = {}
    for path in receipts:
        pair = release_pair(json.loads(path.read_text(encoding="utf-8")))
        tag, modules_tag = pair["client_tag"], pair["modules_tag"]
        if tag in releases and releases[tag] != modules_tag:
            raise ValueError("Conflicting module versions for a client release")
        releases[tag] = modules_tag
    payload = {
        "event_type": "limanix-client-release",
        "client_payload": {
            "releases": [
                {"client_tag": tag, "modules_tag": modules_tag}
                for tag, modules_tag in sorted(releases.items())
            ]
        },
    }
    (temporary / "docs-event.json").write_text(
        json.dumps(payload, separators=(",", ":")) + "\n", encoding="utf-8"
    )
    output("ready", "true")


def latest() -> None:
    tags = selected_tags()
    if tags:
        command("gh", "release", "edit", tags[0]["tag"], "--latest")


COMMANDS = {
    "modules": modules,
    "verify-modules": verify_modules,
    "plan": plan,
    "verify": verify,
    "receipt": receipt,
    "docs-event": docs_event,
    "latest": latest,
}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=COMMANDS)
    args = parser.parse_args()
    try:
        COMMANDS[args.command]()
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"release/{args.command}: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
