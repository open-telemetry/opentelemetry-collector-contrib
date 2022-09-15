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

import os
import subprocess

import tomli

scripts_path = os.path.dirname(os.path.abspath(__file__))
root_path = os.path.dirname(scripts_path)
instrumentations_path = os.path.join(root_path, "instrumentation")


def get_instrumentation_packages():
    for pkg in sorted(os.listdir(instrumentations_path)):
        pkg_path = os.path.join(instrumentations_path, pkg)
        if not os.path.isdir(pkg_path):
            continue

        version = subprocess.check_output(
            "hatch version",
            shell=True,
            cwd=pkg_path,
            universal_newlines=True,
        )
        pyproject_toml_path = os.path.join(pkg_path, "pyproject.toml")

        with open(pyproject_toml_path, "rb") as file:
            pyproject_toml = tomli.load(file)

        instrumentation = {
            "name": pyproject_toml["project"]["name"],
            "version": version.strip(),
            "instruments": pyproject_toml["project"]["optional-dependencies"][
                "instruments"
            ],
        }
        instrumentation["requirement"] = "==".join(
            (
                instrumentation["name"],
                instrumentation["version"],
            )
        )
        yield instrumentation


if __name__ == "__main__":
    print(list(get_instrumentation_packages()))
