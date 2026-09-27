"""Prepare this repository's Markdown section for the Limanix documentation site."""

from __future__ import annotations

import argparse
import shutil
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
REFERENCES = ("cli.md", "configuration.md", "limanix.example.toml", "metadata.json")


def prepare(root: Path) -> Path:
    guides = root / "guides"
    if not (guides / "index.md").is_file():
        raise ValueError("Missing guide: guides/index.md")

    generated = root / "build" / "docs-generated"
    for name in REFERENCES:
        if not (generated / name).is_file():
            raise ValueError(f"Missing generated reference: build/docs-generated/{name}")

    output = root / "build" / "docs"
    if output.parent.is_symlink() or output.is_symlink():
        raise ValueError("build/ and build/docs/ must not be symlinks")
    if output.exists():
        shutil.rmtree(output)
    shutil.copytree(guides, output)

    references = output / "_generated"
    references.mkdir()
    for name in REFERENCES:
        shutil.copyfile(generated / name, references / name)

    return output


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.parse_args()

    try:
        output = prepare(ROOT)
    except (OSError, ValueError) as error:
        print(f"docs/prepare: {error}", file=sys.stderr)
        return 1
    print(f"Prepared documentation in {output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
