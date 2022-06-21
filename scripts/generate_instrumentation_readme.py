#!/usr/bin/env python3

# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import logging
import os

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("instrumentation_readme_generator")

_prefix = "opentelemetry-instrumentation-"

header = """
| Instrumentation | Supported Packages | Metrics support |
| --------------- | ------------------ | --------------- |"""


def main():
    root_path = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    base_instrumentation_path = os.path.join(root_path, "instrumentation")

    table = [header]
    for instrumentation in sorted(os.listdir(base_instrumentation_path)):
        instrumentation_path = os.path.join(
            base_instrumentation_path, instrumentation
        )
        if not os.path.isdir(
            instrumentation_path
        ) or not instrumentation.startswith(_prefix):
            continue

        src_dir = os.path.join(
            instrumentation_path, "src", "opentelemetry", "instrumentation"
        )
        src_pkgs = [
            f
            for f in os.listdir(src_dir)
            if os.path.isdir(os.path.join(src_dir, f))
        ]
        assert len(src_pkgs) == 1
        name = src_pkgs[0]

        pkg_info = {}
        version_filename = os.path.join(
            src_dir,
            name,
            "package.py",
        )
        with open(version_filename, encoding="utf-8") as fh:
            exec(fh.read(), pkg_info)

        instruments = pkg_info["_instruments"]
        supports_metrics = pkg_info.get("_supports_metrics")
        if not instruments:
            instruments = (name,)

        metric_column = "Yes" if supports_metrics else "No"

        table.append(
            f"| [{instrumentation}](./{instrumentation}) | {','.join(instruments)} | {metric_column}"
        )

    with open(
        os.path.join(base_instrumentation_path, "README.md"),
        "w",
        encoding="utf-8",
    ) as fh:
        fh.write("\n".join(table))


if __name__ == "__main__":
    main()
