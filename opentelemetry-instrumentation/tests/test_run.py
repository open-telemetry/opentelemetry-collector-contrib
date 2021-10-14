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

from os import environ, getcwd
from os.path import abspath, dirname, pathsep
from unittest import TestCase
from unittest.mock import patch

from opentelemetry.environment_variables import OTEL_TRACES_EXPORTER
from opentelemetry.instrumentation import auto_instrumentation


class TestRun(TestCase):
    auto_instrumentation_path = dirname(abspath(auto_instrumentation.__file__))

    @classmethod
    def setUpClass(cls):
        cls.execl_patcher = patch(
            "opentelemetry.instrumentation.auto_instrumentation.execl"
        )
        cls.which_patcher = patch(
            "opentelemetry.instrumentation.auto_instrumentation.which"
        )

        cls.execl_patcher.start()
        cls.which_patcher.start()

    @classmethod
    def tearDownClass(cls):
        cls.execl_patcher.stop()
        cls.which_patcher.stop()

    @patch("sys.argv", ["instrument", ""])
    @patch.dict("os.environ", {"PYTHONPATH": ""})
    def test_empty(self):
        auto_instrumentation.run()
        self.assertEqual(
            environ["PYTHONPATH"],
            pathsep.join([self.auto_instrumentation_path, getcwd()]),
        )

    @patch("sys.argv", ["instrument", ""])
    @patch.dict("os.environ", {"PYTHONPATH": "abc"})
    def test_non_empty(self):
        auto_instrumentation.run()
        self.assertEqual(
            environ["PYTHONPATH"],
            pathsep.join([self.auto_instrumentation_path, getcwd(), "abc"]),
        )

    @patch("sys.argv", ["instrument", ""])
    @patch.dict(
        "os.environ",
        {"PYTHONPATH": pathsep.join(["abc", auto_instrumentation_path])},
    )
    def test_after_path(self):
        auto_instrumentation.run()
        self.assertEqual(
            environ["PYTHONPATH"],
            pathsep.join([self.auto_instrumentation_path, getcwd(), "abc"]),
        )

    @patch("sys.argv", ["instrument", ""])
    @patch.dict(
        "os.environ",
        {
            "PYTHONPATH": pathsep.join(
                [auto_instrumentation_path, "abc", auto_instrumentation_path]
            )
        },
    )
    def test_single_path(self):
        auto_instrumentation.run()
        self.assertEqual(
            environ["PYTHONPATH"],
            pathsep.join([self.auto_instrumentation_path, getcwd(), "abc"]),
        )


class TestExecl(TestCase):
    @patch("sys.argv", ["1", "2", "3"])
    @patch("opentelemetry.instrumentation.auto_instrumentation.which")
    @patch("opentelemetry.instrumentation.auto_instrumentation.execl")
    def test_execl(
        self, mock_execl, mock_which
    ):  # pylint: disable=no-self-use
        mock_which.configure_mock(**{"return_value": "python"})

        auto_instrumentation.run()

        mock_execl.assert_called_with("python", "python", "3")


class TestArgs(TestCase):
    @patch("opentelemetry.instrumentation.auto_instrumentation.execl")
    def test_exporter(self, _):  # pylint: disable=no-self-use
        with patch("sys.argv", ["instrument", "2"]):
            auto_instrumentation.run()
            self.assertIsNone(environ.get(OTEL_TRACES_EXPORTER))

        with patch(
            "sys.argv",
            ["instrument", "--traces_exporter", "jaeger", "1", "2"],
        ):
            auto_instrumentation.run()
            self.assertEqual(environ.get(OTEL_TRACES_EXPORTER), "jaeger")
