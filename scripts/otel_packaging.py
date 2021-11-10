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

import json
import os
import subprocess

scripts_path = os.path.dirname(os.path.abspath(__file__))
root_path = os.path.dirname(scripts_path)
instrumentations_path = os.path.join(root_path, "instrumentation")


def get_instrumentation_packages():
    for pkg in sorted(os.listdir(instrumentations_path)):
        pkg_path = os.path.join(instrumentations_path, pkg)
        if not os.path.isdir(pkg_path):
            continue

        out = subprocess.check_output(
            "python setup.py meta",
            shell=True,
            cwd=pkg_path,
            universal_newlines=True,
        )
        instrumentation = json.loads(out.splitlines()[1])
        instrumentation["requirement"] = "==".join(
            (
                instrumentation["name"],
                instrumentation["version"],
            )
        )
        yield instrumentation


if __name__ == "__main__":
    print(list(get_instrumentation_packages()))
