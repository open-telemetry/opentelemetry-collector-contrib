# Copyright The OpenTelemetry Authors
# SPDX-License-Identifier: Apache-2.0

"""Build a small Collector using the current checkout's SNMP receiver."""

import argparse
import json
from pathlib import Path
import re
import subprocess


def main():
    example = Path(__file__).resolve().parent
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-root", type=Path, default=example.parents[3])
    parser.add_argument("--output", type=Path, default=example / ".work/collector")
    args = parser.parse_args()
    root = args.source_root.resolve()
    manifest = (root / "cmd/otelcontribcol/builder-config.yaml").read_text()

    def component(module):
        match = re.search(r"gomod: " + re.escape(module) + r" (\S+)", manifest)
        if match is None:
            raise RuntimeError(f"Cannot find {module} in the repository builder manifest")
        return {"gomod": f"{module} {match[1]}"}

    prefix = "github.com/open-telemetry/opentelemetry-collector-contrib/"
    paths = subprocess.check_output(
        ["git", "ls-files", "go.mod", "**/go.mod"], cwd=root, text=True
    ).splitlines()
    replaces = []
    for path in paths:
        module = re.search(r"^module (\S+)", (root / path).read_text(), re.MULTILINE)[1]
        # The builder inserts replacements verbatim into go.mod. JSON quoting
        # also produces a Go string literal, preserving spaces and non-ASCII paths.
        directory = json.dumps(str((root / path).parent), ensure_ascii=False)
        replaces.append(f"{module} => {directory}")

    config = {
        "dist": {
            "module": prefix + "cmd/snmp-simulation",
            "name": "snmp-simulation",
            "description": "Local SNMP simulation Collector",
            "output_path": str(args.output.resolve()),
        },
        "receivers": [component(prefix + "receiver/snmpreceiver")],
        "exporters": [component(prefix + "exporter/fileexporter")],
        "providers": [
            component("go.opentelemetry.io/collector/confmap/provider/fileprovider"),
            component("go.opentelemetry.io/collector/confmap/provider/envprovider"),
        ],
        "replaces": replaces,
    }
    output = example / ".work/builder.yaml"
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(config, indent=2) + "\n")
    subprocess.run(
        ["go", "tool", "-modfile=internal/tools/go.mod",
         "go.opentelemetry.io/collector/cmd/builder", "--config", str(output)],
        cwd=root, check=True,
    )
    revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip()
    (args.output.resolve() / "build-info.json").write_text(
        json.dumps({"source_root": str(root), "source_revision": revision}, indent=2) + "\n"
    )


if __name__ == "__main__":
    main()
