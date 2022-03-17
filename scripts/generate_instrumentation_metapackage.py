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
from configparser import ConfigParser

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("instrumentation_metapackage_generator")

_prefix = "opentelemetry-instrumentation-"


root_path = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
base_instrumentation_path = os.path.join(root_path, "instrumentation")


def get_instrumentation_packages():
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
            "version.py",
        )
        with open(version_filename, encoding="utf-8") as fh:
            exec(fh.read(), pkg_info)

        version = pkg_info["__version__"]
        yield (instrumentation, version)


def main():
    dependencies = get_instrumentation_packages()

    setup_cfg_path = os.path.join(
        root_path, "opentelemetry-contrib-instrumentations", "setup.cfg"
    )
    config = ConfigParser()
    config.read(setup_cfg_path)

    deps = "\n".join(f"{pkg}=={version}" for pkg, version in dependencies)

    config["options"]["install_requires"] = "\n" + deps
    with open(setup_cfg_path, "w", encoding="utf-8") as fh:
        config.write(fh)


if __name__ == "__main__":
    main()
