"""Test script to check given paths for valid README.rst files."""
import argparse
import sys
from pathlib import Path

import readme_renderer.rst


def is_valid_rst(path):
    """Checks if RST can be rendered on PyPI."""
    with open(path, encoding="utf-8") as readme_file:
        markup = readme_file.read()
    return readme_renderer.rst.render(markup, stream=sys.stderr) is not None


def parse_args():
    parser = argparse.ArgumentParser(
        description="Checks README.rst file in path for syntax errors."
    )
    parser.add_argument(
        "paths", nargs="+", help="paths containing a README.rst to test"
    )
    parser.add_argument("-v", "--verbose", action="store_true")
    return parser.parse_args()


def main():
    args = parse_args()
    error = False

    for path in map(Path, args.paths):
        readme = path / "README.rst"
        try:
            if not is_valid_rst(readme):
                error = True
                print("FAILED: RST syntax errors in", readme)
                continue
        except FileNotFoundError:
            print("FAILED: README.rst not found in", path)
            continue
        if args.verbose:
            print("PASSED:", readme)

    if error:
        sys.exit(1)
    print("All clear.")


if __name__ == "__main__":
    main()
