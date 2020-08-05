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

from collections import namedtuple
from unittest import mock

from opentelemetry import metrics
from opentelemetry.instrumentation.system_metrics import SystemMetrics
from opentelemetry.test.test_base import TestBase


class TestSystemMetrics(TestBase):
    def setUp(self):
        super().setUp()
        self.memory_metrics_exporter.clear()

    def test_system_metrics_constructor(self):
        # ensure the observers have been registered
        meter = metrics.get_meter(__name__)
        with mock.patch("opentelemetry.metrics.get_meter") as mock_get_meter:
            mock_get_meter.return_value = meter
            SystemMetrics(self.memory_metrics_exporter)
        self.assertEqual(len(meter.observers), 6)
        observer_names = [
            "system.mem",
            "system.cpu",
            "system.net.bytes",
            "runtime.python.mem",
            "runtime.python.cpu",
            "runtime.python.gc.count",
        ]
        for observer in meter.observers:
            self.assertIn(observer.name, observer_names)
            observer_names.remove(observer.name)

    def _assert_metrics(self, observer_name, system_metrics, expected):
        system_metrics.controller.tick()
        assertions = 0
        for (
            metric
        ) in (
            self.memory_metrics_exporter._exported_metrics  # pylint: disable=protected-access
        ):
            if (
                metric.labels in expected
                and metric.instrument.name == observer_name
            ):
                self.assertEqual(
                    metric.aggregator.checkpoint.last, expected[metric.labels],
                )
                assertions += 1
        self.assertEqual(len(expected), assertions)

    def _test_metrics(self, observer_name, expected):
        meter = self.meter_provider.get_meter(__name__)

        with mock.patch("opentelemetry.metrics.get_meter") as mock_get_meter:
            mock_get_meter.return_value = meter
            system_metrics = SystemMetrics(self.memory_metrics_exporter)
            self._assert_metrics(observer_name, system_metrics, expected)

    @mock.patch("psutil.cpu_times")
    def test_system_cpu(self, mock_cpu_times):
        CPUTimes = namedtuple("CPUTimes", ["user", "nice", "system", "idle"])
        mock_cpu_times.return_value = CPUTimes(
            user=332277.48, nice=0.0, system=309836.43, idle=6724698.94
        )

        expected = {
            (("type", "user"),): 332277.48,
            (("type", "system"),): 309836.43,
            (("type", "idle"),): 6724698.94,
        }
        self._test_metrics("system.cpu", expected)

    @mock.patch("psutil.virtual_memory")
    def test_system_memory(self, mock_virtual_memory):
        VirtualMemory = namedtuple(
            "VirtualMemory",
            [
                "total",
                "available",
                "percent",
                "used",
                "free",
                "active",
                "inactive",
                "wired",
            ],
        )
        mock_virtual_memory.return_value = VirtualMemory(
            total=17179869184,
            available=5520928768,
            percent=67.9,
            used=10263990272,
            free=266964992,
            active=5282459648,
            inactive=5148700672,
            wired=4981530624,
        )

        expected = {
            (("type", "total"),): 17179869184,
            (("type", "used"),): 10263990272,
            (("type", "available"),): 5520928768,
            (("type", "free"),): 266964992,
        }
        self._test_metrics("system.mem", expected)

    @mock.patch("psutil.net_io_counters")
    def test_network_bytes(self, mock_net_io_counters):
        NetworkIO = namedtuple(
            "NetworkIO",
            ["bytes_sent", "bytes_recv", "packets_recv", "packets_sent"],
        )
        mock_net_io_counters.return_value = NetworkIO(
            bytes_sent=23920188416,
            bytes_recv=46798894080,
            packets_sent=53127118,
            packets_recv=53205738,
        )

        expected = {
            (("type", "bytes_recv"),): 46798894080,
            (("type", "bytes_sent"),): 23920188416,
        }
        self._test_metrics("system.net.bytes", expected)

    def test_runtime_memory(self):
        meter = self.meter_provider.get_meter(__name__)
        with mock.patch("opentelemetry.metrics.get_meter") as mock_get_meter:
            mock_get_meter.return_value = meter
            system_metrics = SystemMetrics(self.memory_metrics_exporter)

            with mock.patch.object(
                system_metrics._proc,  # pylint: disable=protected-access
                "memory_info",
            ) as mock_runtime_memory:
                RuntimeMemory = namedtuple(
                    "RuntimeMemory", ["rss", "vms", "pfaults", "pageins"],
                )
                mock_runtime_memory.return_value = RuntimeMemory(
                    rss=9777152, vms=4385665024, pfaults=2631, pageins=49
                )
                expected = {
                    (("type", "rss"),): 9777152,
                    (("type", "vms"),): 4385665024,
                }
                self._assert_metrics(
                    "runtime.python.mem", system_metrics, expected
                )

    def test_runtime_cpu(self):
        meter = self.meter_provider.get_meter(__name__)
        with mock.patch("opentelemetry.metrics.get_meter") as mock_get_meter:
            mock_get_meter.return_value = meter
            system_metrics = SystemMetrics(self.memory_metrics_exporter)

            with mock.patch.object(
                system_metrics._proc,  # pylint: disable=protected-access
                "cpu_times",
            ) as mock_runtime_cpu_times:
                RuntimeCPU = namedtuple(
                    "RuntimeCPU", ["user", "nice", "system"]
                )
                mock_runtime_cpu_times.return_value = RuntimeCPU(
                    user=100.48, nice=0.0, system=200.43
                )

                expected = {
                    (("type", "user"),): 100.48,
                    (("type", "system"),): 200.43,
                }

                self._assert_metrics(
                    "runtime.python.cpu", system_metrics, expected
                )

    @mock.patch("gc.get_count")
    def test_runtime_gc_count(self, mock_gc):
        mock_gc.return_value = [
            100,  # gen0
            50,  # gen1
            10,  # gen2
        ]

        expected = {
            (("count", "0"),): 100,
            (("count", "1"),): 50,
            (("count", "2"),): 10,
        }
        self._test_metrics("runtime.python.gc.count", expected)
