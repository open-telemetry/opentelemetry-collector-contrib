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
# type: ignore

from io import StringIO
from random import sample
from unittest import TestCase
from unittest.mock import call, patch

from opentelemetry.instrumentation import bootstrap
from opentelemetry.instrumentation.bootstrap_gen import libraries


def sample_packages(packages, rate):
    return sample(
        list(packages),
        int(len(packages) * rate),
    )


class TestBootstrap(TestCase):

    installed_libraries = {}
    installed_instrumentations = {}

    @classmethod
    def setUpClass(cls):
        cls.installed_libraries = sample_packages(
            [lib["instrumentation"] for lib in libraries.values()], 0.6
        )

        # treat 50% of sampled packages as pre-installed
        cls.installed_instrumentations = sample_packages(
            cls.installed_libraries, 0.5
        )

        cls.pkg_patcher = patch(
            "opentelemetry.instrumentation.bootstrap._find_installed_libraries",
            return_value=cls.installed_libraries,
        )

        cls.pip_install_patcher = patch(
            "opentelemetry.instrumentation.bootstrap._sys_pip_install",
        )
        cls.pip_check_patcher = patch(
            "opentelemetry.instrumentation.bootstrap._pip_check",
        )

        cls.pkg_patcher.start()
        cls.mock_pip_install = cls.pip_install_patcher.start()
        cls.mock_pip_check = cls.pip_check_patcher.start()

    @classmethod
    def tearDownClass(cls):
        cls.pip_check_patcher.start()
        cls.pip_install_patcher.start()
        cls.pkg_patcher.stop()

    @patch("sys.argv", ["bootstrap", "-a", "pipenv"])
    def test_run_unknown_cmd(self):
        with self.assertRaises(SystemExit):
            bootstrap.run()

    @patch("sys.argv", ["bootstrap", "-a", "requirements"])
    def test_run_cmd_print(self):
        with patch("sys.stdout", new=StringIO()) as fake_out:
            bootstrap.run()
            self.assertEqual(
                fake_out.getvalue(),
                "\n".join(self.installed_libraries),
            )

    @patch("sys.argv", ["bootstrap", "-a", "install"])
    def test_run_cmd_install(self):
        bootstrap.run()
        self.mock_pip_install.assert_has_calls(
            [call(i) for i in self.installed_libraries],
            any_order=True,
        )
        self.assertEqual(self.mock_pip_check.call_count, 1)
