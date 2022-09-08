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

# pylint: disable=protected-access

from collections import namedtuple
from platform import python_implementation
from unittest import mock

from opentelemetry.instrumentation.system_metrics import (
    SystemMetricsInstrumentor,
)
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import InMemoryMetricReader
from opentelemetry.test.test_base import TestBase


def _mock_netconnection():
    NetConnection = namedtuple(
        "NetworkConnection", ["family", "type", "status"]
    )
    Type = namedtuple("Type", ["value"])
    return [
        NetConnection(
            family=1,
            status="ESTABLISHED",
            type=Type(value=2),
        ),
        NetConnection(
            family=1,
            status="ESTABLISHED",
            type=Type(value=1),
        ),
    ]


class _SystemMetricsResult:
    def __init__(self, attributes, value) -> None:
        self.attributes = attributes
        self.value = value


# pylint: disable=too-many-public-methods
class TestSystemMetrics(TestBase):
    def setUp(self):
        super().setUp()
        self.implementation = python_implementation().lower()
        self._patch_net_connections = mock.patch(
            "psutil.net_connections", _mock_netconnection
        )
        self._patch_net_connections.start()

    def tearDown(self):
        super().tearDown()
        self._patch_net_connections.stop()
        SystemMetricsInstrumentor().uninstrument()

    def test_system_metrics_instrument(self):
        reader = InMemoryMetricReader()
        meter_provider = MeterProvider(metric_readers=[reader])
        system_metrics = SystemMetricsInstrumentor()
        system_metrics.instrument(meter_provider=meter_provider)
        metric_names = []
        for resource_metrics in reader.get_metrics_data().resource_metrics:
            for scope_metrics in resource_metrics.scope_metrics:
                for metric in scope_metrics.metrics:
                    metric_names.append(metric.name)
        self.assertEqual(len(metric_names), 24)

        observer_names = [
            "system.cpu.time",
            "system.cpu.utilization",
            "system.memory.usage",
            "system.memory.utilization",
            "system.swap.usage",
            "system.swap.utilization",
            "system.disk.io.read",
            "system.disk.io.write",
            "system.disk.operations.read",
            "system.disk.operations.write",
            "system.disk.operation_time.read",
            "system.disk.operation_time.write",
            "system.network.dropped.transmit",
            "system.network.dropped.receive",
            "system.network.packets.transmit",
            "system.network.packets.receive",
            "system.network.errors.transmit",
            "system.network.errors.receive",
            "system.network.io.transmit",
            "system.network.io.receive",
            "system.network.connections",
            f"runtime.{self.implementation}.memory",
            f"runtime.{self.implementation}.cpu_time",
            f"runtime.{self.implementation}.gc_count",
        ]

        for observer in metric_names:
            self.assertIn(observer, observer_names)
            observer_names.remove(observer)

    def _assert_metrics(self, observer_name, reader, expected):
        assertions = 0
        # pylint: disable=too-many-nested-blocks
        for resource_metrics in reader.get_metrics_data().resource_metrics:
            for scope_metrics in resource_metrics.scope_metrics:
                for metric in scope_metrics.metrics:
                    for data_point in metric.data.data_points:
                        for expect in expected:
                            if (
                                dict(data_point.attributes)
                                == expect.attributes
                                and metric.name == observer_name
                            ):
                                self.assertEqual(
                                    data_point.value,
                                    expect.value,
                                )
                                assertions += 1
        self.assertEqual(len(expected), assertions)

    def _test_metrics(self, observer_name, expected):
        reader = InMemoryMetricReader()
        meter_provider = MeterProvider(metric_readers=[reader])

        system_metrics = SystemMetricsInstrumentor()
        system_metrics.instrument(meter_provider=meter_provider)
        self._assert_metrics(observer_name, reader, expected)

    # This patch is added here to stop psutil from raising an exception
    # because we're patching cpu_times
    # pylint: disable=unused-argument
    @mock.patch("psutil.cpu_times_percent")
    @mock.patch("psutil.cpu_times")
    def test_system_cpu_time(self, mock_cpu_times, mock_cpu_times_percent):
        CPUTimes = namedtuple("CPUTimes", ["idle", "user", "system", "irq"])
        mock_cpu_times.return_value = [
            CPUTimes(idle=1.2, user=3.4, system=5.6, irq=7.8),
            CPUTimes(idle=1.2, user=3.4, system=5.6, irq=7.8),
        ]

        expected = [
            _SystemMetricsResult(
                {
                    "cpu": 1,
                    "state": "idle",
                },
                1.2,
            ),
            _SystemMetricsResult(
                {
                    "cpu": 1,
                    "state": "user",
                },
                3.4,
            ),
            _SystemMetricsResult(
                {
                    "cpu": 1,
                    "state": "system",
                },
                5.6,
            ),
            _SystemMetricsResult(
                {
                    "cpu": 1,
                    "state": "irq",
                },
                7.8,
            ),
            _SystemMetricsResult(
                {
                    "cpu": 2,
                    "state": "idle",
                },
                1.2,
            ),
            _SystemMetricsResult(
                {
                    "cpu": 2,
                    "state": "user",
                },
                3.4,
            ),
            _SystemMetricsResult(
                {
                    "cpu": 2,
                    "state": "system",
                },
                5.6,
            ),
            _SystemMetricsResult(
                {
                    "cpu": 2,
                    "state": "irq",
                },
                7.8,
            ),
        ]
        self._test_metrics("system.cpu.time", expected)

    @mock.patch("psutil.cpu_times_percent")
    def test_system_cpu_utilization(self, mock_cpu_times_percent):
        CPUTimesPercent = namedtuple(
            "CPUTimesPercent", ["idle", "user", "system", "irq"]
        )
        mock_cpu_times_percent.return_value = [
            CPUTimesPercent(idle=1.2, user=3.4, system=5.6, irq=7.8),
            CPUTimesPercent(idle=1.2, user=3.4, system=5.6, irq=7.8),
        ]

        expected = [
            _SystemMetricsResult({"cpu": 1, "state": "idle"}, 1.2 / 100),
            _SystemMetricsResult({"cpu": 1, "state": "user"}, 3.4 / 100),
            _SystemMetricsResult({"cpu": 1, "state": "system"}, 5.6 / 100),
            _SystemMetricsResult({"cpu": 1, "state": "irq"}, 7.8 / 100),
            _SystemMetricsResult({"cpu": 2, "state": "idle"}, 1.2 / 100),
            _SystemMetricsResult({"cpu": 2, "state": "user"}, 3.4 / 100),
            _SystemMetricsResult({"cpu": 2, "state": "system"}, 5.6 / 100),
            _SystemMetricsResult({"cpu": 2, "state": "irq"}, 7.8 / 100),
        ]
        self._test_metrics("system.cpu.utilization", expected)

    @mock.patch("psutil.virtual_memory")
    def test_system_memory_usage(self, mock_virtual_memory):
        VirtualMemory = namedtuple(
            "VirtualMemory", ["used", "free", "cached", "total"]
        )
        mock_virtual_memory.return_value = VirtualMemory(
            used=1, free=2, cached=3, total=4
        )

        expected = [
            _SystemMetricsResult({"state": "used"}, 1),
            _SystemMetricsResult({"state": "free"}, 2),
            _SystemMetricsResult({"state": "cached"}, 3),
        ]
        self._test_metrics("system.memory.usage", expected)

    @mock.patch("psutil.virtual_memory")
    def test_system_memory_utilization(self, mock_virtual_memory):
        VirtualMemory = namedtuple(
            "VirtualMemory", ["used", "free", "cached", "total"]
        )
        mock_virtual_memory.return_value = VirtualMemory(
            used=1, free=2, cached=3, total=4
        )

        expected = [
            _SystemMetricsResult({"state": "used"}, 1 / 4),
            _SystemMetricsResult({"state": "free"}, 2 / 4),
            _SystemMetricsResult({"state": "cached"}, 3 / 4),
        ]
        self._test_metrics("system.memory.utilization", expected)

    @mock.patch("psutil.swap_memory")
    def test_system_swap_usage(self, mock_swap_memory):
        SwapMemory = namedtuple("SwapMemory", ["used", "free", "total"])
        mock_swap_memory.return_value = SwapMemory(used=1, free=2, total=3)

        expected = [
            _SystemMetricsResult({"state": "used"}, 1),
            _SystemMetricsResult({"state": "free"}, 2),
        ]
        self._test_metrics("system.swap.usage", expected)

    @mock.patch("psutil.swap_memory")
    def test_system_swap_utilization(self, mock_swap_memory):
        SwapMemory = namedtuple("SwapMemory", ["used", "free", "total"])
        mock_swap_memory.return_value = SwapMemory(used=1, free=2, total=3)

        expected = [
            _SystemMetricsResult({"state": "used"}, 1 / 3),
            _SystemMetricsResult({"state": "free"}, 2 / 3),
        ]
        self._test_metrics("system.swap.utilization", expected)

    @mock.patch("psutil.disk_io_counters")
    def test_system_disk_io_read(self, mock_disk_io_counters):
        DiskIO = namedtuple(
            "DiskIO",
            [
                "read_count",
                "write_count",
                "read_bytes",
                "write_bytes",
                "read_time",
                "write_time",
                "read_merged_count",
                "write_merged_count",
            ],
        )
        mock_disk_io_counters.return_value = {
            "sda": DiskIO(
                read_count=1,
                write_count=2,
                read_bytes=3,
                write_bytes=4,
                read_time=5,
                write_time=6,
                read_merged_count=7,
                write_merged_count=8,
            ),
            "sdb": DiskIO(
                read_count=9,
                write_count=10,
                read_bytes=11,
                write_bytes=12,
                read_time=13,
                write_time=14,
                read_merged_count=15,
                write_merged_count=16,
            ),
        }

        expected = [
            _SystemMetricsResult({"device": "sda"}, 3),
            _SystemMetricsResult({"device": "sdb"}, 11),
        ]
        self._test_metrics("system.disk.io.read", expected)

    @mock.patch("psutil.disk_io_counters")
    def test_system_disk_io_write(self, mock_disk_io_counters):
        DiskIO = namedtuple(
            "DiskIO",
            [
                "read_count",
                "write_count",
                "read_bytes",
                "write_bytes",
                "read_time",
                "write_time",
                "read_merged_count",
                "write_merged_count",
            ],
        )
        mock_disk_io_counters.return_value = {
            "sda": DiskIO(
                read_count=1,
                write_count=2,
                read_bytes=3,
                write_bytes=4,
                read_time=5,
                write_time=6,
                read_merged_count=7,
                write_merged_count=8,
            ),
            "sdb": DiskIO(
                read_count=9,
                write_count=10,
                read_bytes=11,
                write_bytes=12,
                read_time=13,
                write_time=14,
                read_merged_count=15,
                write_merged_count=16,
            ),
        }

        expected = [
            _SystemMetricsResult({"device": "sda"}, 4),
            _SystemMetricsResult({"device": "sdb"}, 12),
        ]
        self._test_metrics("system.disk.io.write", expected)

    @mock.patch("psutil.disk_io_counters")
    def test_system_disk_operations_read(self, mock_disk_io_counters):
        DiskIO = namedtuple(
            "DiskIO",
            [
                "read_count",
                "write_count",
                "read_bytes",
                "write_bytes",
                "read_time",
                "write_time",
                "read_merged_count",
                "write_merged_count",
            ],
        )
        mock_disk_io_counters.return_value = {
            "sda": DiskIO(
                read_count=1,
                write_count=2,
                read_bytes=3,
                write_bytes=4,
                read_time=5,
                write_time=6,
                read_merged_count=7,
                write_merged_count=8,
            ),
            "sdb": DiskIO(
                read_count=9,
                write_count=10,
                read_bytes=11,
                write_bytes=12,
                read_time=13,
                write_time=14,
                read_merged_count=15,
                write_merged_count=16,
            ),
        }

        expected = [
            _SystemMetricsResult({"device": "sda"}, 1),
            _SystemMetricsResult({"device": "sdb"}, 9),
        ]
        self._test_metrics("system.disk.operations.read", expected)

    @mock.patch("psutil.disk_io_counters")
    def test_system_disk_operations_write(self, mock_disk_io_counters):
        DiskIO = namedtuple(
            "DiskIO",
            [
                "read_count",
                "write_count",
                "read_bytes",
                "write_bytes",
                "read_time",
                "write_time",
                "read_merged_count",
                "write_merged_count",
            ],
        )
        mock_disk_io_counters.return_value = {
            "sda": DiskIO(
                read_count=1,
                write_count=2,
                read_bytes=3,
                write_bytes=4,
                read_time=5,
                write_time=6,
                read_merged_count=7,
                write_merged_count=8,
            ),
            "sdb": DiskIO(
                read_count=9,
                write_count=10,
                read_bytes=11,
                write_bytes=12,
                read_time=13,
                write_time=14,
                read_merged_count=15,
                write_merged_count=16,
            ),
        }

        expected = [
            _SystemMetricsResult({"device": "sda"}, 2),
            _SystemMetricsResult({"device": "sdb"}, 10),
        ]
        self._test_metrics("system.disk.operations.write", expected)

    @mock.patch("psutil.disk_io_counters")
    def test_system_disk_operation_time_read(self, mock_disk_io_counters):
        DiskIO = namedtuple(
            "DiskIO",
            [
                "read_count",
                "write_count",
                "read_bytes",
                "write_bytes",
                "read_time",
                "write_time",
                "read_merged_count",
                "write_merged_count",
            ],
        )
        mock_disk_io_counters.return_value = {
            "sda": DiskIO(
                read_count=1,
                write_count=2,
                read_bytes=3,
                write_bytes=4,
                read_time=5,
                write_time=6,
                read_merged_count=7,
                write_merged_count=8,
            ),
            "sdb": DiskIO(
                read_count=9,
                write_count=10,
                read_bytes=11,
                write_bytes=12,
                read_time=13,
                write_time=14,
                read_merged_count=15,
                write_merged_count=16,
            ),
        }

        expected = [
            _SystemMetricsResult({"device": "sda"}, 5 / 1000),
            _SystemMetricsResult({"device": "sdb"}, 13 / 1000),
        ]
        self._test_metrics("system.disk.operation_time.read", expected)

    @mock.patch("psutil.disk_io_counters")
    def test_system_disk_operation_time_write(self, mock_disk_io_counters):
        DiskIO = namedtuple(
            "DiskIO",
            [
                "read_count",
                "write_count",
                "read_bytes",
                "write_bytes",
                "read_time",
                "write_time",
                "read_merged_count",
                "write_merged_count",
            ],
        )
        mock_disk_io_counters.return_value = {
            "sda": DiskIO(
                read_count=1,
                write_count=2,
                read_bytes=3,
                write_bytes=4,
                read_time=5,
                write_time=6,
                read_merged_count=7,
                write_merged_count=8,
            ),
            "sdb": DiskIO(
                read_count=9,
                write_count=10,
                read_bytes=11,
                write_bytes=12,
                read_time=13,
                write_time=14,
                read_merged_count=15,
                write_merged_count=16,
            ),
        }

        expected = [
            _SystemMetricsResult({"device": "sda"}, 6 / 1000),
            _SystemMetricsResult({"device": "sdb"}, 14 / 1000),
        ]
        self._test_metrics("system.disk.operation_time.write", expected)

    @mock.patch("psutil.net_io_counters")
    def test_system_network_dropped_transmit(self, mock_net_io_counters):
        NetIO = namedtuple(
            "NetIO",
            [
                "dropin",
                "dropout",
                "packets_sent",
                "packets_recv",
                "errin",
                "errout",
                "bytes_sent",
                "bytes_recv",
            ],
        )
        mock_net_io_counters.return_value = {
            "eth0": NetIO(
                dropin=1,
                dropout=2,
                packets_sent=3,
                packets_recv=4,
                errin=5,
                errout=6,
                bytes_sent=7,
                bytes_recv=8,
            ),
            "eth1": NetIO(
                dropin=9,
                dropout=10,
                packets_sent=11,
                packets_recv=12,
                errin=13,
                errout=14,
                bytes_sent=15,
                bytes_recv=16,
            ),
        }

        expected = [
            _SystemMetricsResult({"device": "eth0"}, 2),
            _SystemMetricsResult({"device": "eth1"}, 10),
        ]
        self._test_metrics("system.network.dropped.transmit", expected)

    @mock.patch("psutil.net_io_counters")
    def test_system_network_dropped_receive(self, mock_net_io_counters):
        NetIO = namedtuple(
            "NetIO",
            [
                "dropin",
                "dropout",
                "packets_sent",
                "packets_recv",
                "errin",
                "errout",
                "bytes_sent",
                "bytes_recv",
            ],
        )
        mock_net_io_counters.return_value = {
            "eth0": NetIO(
                dropin=1,
                dropout=2,
                packets_sent=3,
                packets_recv=4,
                errin=5,
                errout=6,
                bytes_sent=7,
                bytes_recv=8,
            ),
            "eth1": NetIO(
                dropin=9,
                dropout=10,
                packets_sent=11,
                packets_recv=12,
                errin=13,
                errout=14,
                bytes_sent=15,
                bytes_recv=16,
            ),
        }

        expected = [
            _SystemMetricsResult({"device": "eth0"}, 1),
            _SystemMetricsResult({"device": "eth1"}, 9),
        ]
        self._test_metrics("system.network.dropped.receive", expected)

    @mock.patch("psutil.net_io_counters")
    def test_system_network_packets_transmit(self, mock_net_io_counters):
        NetIO = namedtuple(
            "NetIO",
            [
                "dropin",
                "dropout",
                "packets_sent",
                "packets_recv",
                "errin",
                "errout",
                "bytes_sent",
                "bytes_recv",
            ],
        )
        mock_net_io_counters.return_value = {
            "eth0": NetIO(
                dropin=1,
                dropout=2,
                packets_sent=3,
                packets_recv=4,
                errin=5,
                errout=6,
                bytes_sent=7,
                bytes_recv=8,
            ),
            "eth1": NetIO(
                dropin=9,
                dropout=10,
                packets_sent=11,
                packets_recv=12,
                errin=13,
                errout=14,
                bytes_sent=15,
                bytes_recv=16,
            ),
        }

        expected = [
            _SystemMetricsResult({"device": "eth0"}, 3),
            _SystemMetricsResult({"device": "eth1"}, 11),
        ]
        self._test_metrics("system.network.packets.transmit", expected)

    @mock.patch("psutil.net_io_counters")
    def test_system_network_packets_receive(self, mock_net_io_counters):
        NetIO = namedtuple(
            "NetIO",
            [
                "dropin",
                "dropout",
                "packets_sent",
                "packets_recv",
                "errin",
                "errout",
                "bytes_sent",
                "bytes_recv",
            ],
        )
        mock_net_io_counters.return_value = {
            "eth0": NetIO(
                dropin=1,
                dropout=2,
                packets_sent=3,
                packets_recv=4,
                errin=5,
                errout=6,
                bytes_sent=7,
                bytes_recv=8,
            ),
            "eth1": NetIO(
                dropin=9,
                dropout=10,
                packets_sent=11,
                packets_recv=12,
                errin=13,
                errout=14,
                bytes_sent=15,
                bytes_recv=16,
            ),
        }

        expected = [
            _SystemMetricsResult({"device": "eth0"}, 4),
            _SystemMetricsResult({"device": "eth1"}, 12),
        ]
        self._test_metrics("system.network.packets.receive", expected)

    @mock.patch("psutil.net_io_counters")
    def test_system_network_errors_transmit(self, mock_net_io_counters):
        NetIO = namedtuple(
            "NetIO",
            [
                "dropin",
                "dropout",
                "packets_sent",
                "packets_recv",
                "errin",
                "errout",
                "bytes_sent",
                "bytes_recv",
            ],
        )
        mock_net_io_counters.return_value = {
            "eth0": NetIO(
                dropin=1,
                dropout=2,
                packets_sent=3,
                packets_recv=4,
                errin=5,
                errout=6,
                bytes_sent=7,
                bytes_recv=8,
            ),
            "eth1": NetIO(
                dropin=9,
                dropout=10,
                packets_sent=11,
                packets_recv=12,
                errin=13,
                errout=14,
                bytes_sent=15,
                bytes_recv=16,
            ),
        }

        expected = [
            _SystemMetricsResult({"device": "eth0"}, 6),
            _SystemMetricsResult({"device": "eth1"}, 14),
        ]
        self._test_metrics("system.network.errors.transmit", expected)

    @mock.patch("psutil.net_io_counters")
    def test_system_network_errors_receive(self, mock_net_io_counters):
        NetIO = namedtuple(
            "NetIO",
            [
                "dropin",
                "dropout",
                "packets_sent",
                "packets_recv",
                "errin",
                "errout",
                "bytes_sent",
                "bytes_recv",
            ],
        )
        mock_net_io_counters.return_value = {
            "eth0": NetIO(
                dropin=1,
                dropout=2,
                packets_sent=3,
                packets_recv=4,
                errin=5,
                errout=6,
                bytes_sent=7,
                bytes_recv=8,
            ),
            "eth1": NetIO(
                dropin=9,
                dropout=10,
                packets_sent=11,
                packets_recv=12,
                errin=13,
                errout=14,
                bytes_sent=15,
                bytes_recv=16,
            ),
        }

        expected = [
            _SystemMetricsResult({"device": "eth0"}, 5),
            _SystemMetricsResult({"device": "eth1"}, 13),
        ]
        self._test_metrics("system.network.errors.receive", expected)

    @mock.patch("psutil.net_io_counters")
    def test_system_network_io_transmit(self, mock_net_io_counters):
        NetIO = namedtuple(
            "NetIO",
            [
                "dropin",
                "dropout",
                "packets_sent",
                "packets_recv",
                "errin",
                "errout",
                "bytes_sent",
                "bytes_recv",
            ],
        )
        mock_net_io_counters.return_value = {
            "eth0": NetIO(
                dropin=1,
                dropout=2,
                packets_sent=3,
                packets_recv=4,
                errin=5,
                errout=6,
                bytes_sent=7,
                bytes_recv=8,
            ),
            "eth1": NetIO(
                dropin=9,
                dropout=10,
                packets_sent=11,
                packets_recv=12,
                errin=13,
                errout=14,
                bytes_sent=15,
                bytes_recv=16,
            ),
        }

        expected = [
            _SystemMetricsResult({"device": "eth0"}, 7),
            _SystemMetricsResult({"device": "eth1"}, 15),
        ]
        self._test_metrics("system.network.io.transmit", expected)

    @mock.patch("psutil.net_io_counters")
    def test_system_network_io_receive(self, mock_net_io_counters):
        NetIO = namedtuple(
            "NetIO",
            [
                "dropin",
                "dropout",
                "packets_sent",
                "packets_recv",
                "errin",
                "errout",
                "bytes_sent",
                "bytes_recv",
            ],
        )
        mock_net_io_counters.return_value = {
            "eth0": NetIO(
                dropin=1,
                dropout=2,
                packets_sent=3,
                packets_recv=4,
                errin=5,
                errout=6,
                bytes_sent=7,
                bytes_recv=8,
            ),
            "eth1": NetIO(
                dropin=9,
                dropout=10,
                packets_sent=11,
                packets_recv=12,
                errin=13,
                errout=14,
                bytes_sent=15,
                bytes_recv=16,
            ),
        }

        expected = [
            _SystemMetricsResult({"device": "eth0"}, 8),
            _SystemMetricsResult({"device": "eth1"}, 16),
        ]
        self._test_metrics("system.network.io.receive", expected)

    @mock.patch("psutil.net_connections")
    def test_system_network_connections(self, mock_net_connections):
        NetConnection = namedtuple(
            "NetworkConnection", ["family", "type", "status"]
        )
        Type = namedtuple("Type", ["value"])
        mock_net_connections.return_value = [
            NetConnection(
                family=1,
                status="ESTABLISHED",
                type=Type(value=2),
            ),
            NetConnection(
                family=1,
                status="ESTABLISHED",
                type=Type(value=1),
            ),
        ]

        expected = [
            _SystemMetricsResult(
                {
                    "family": 1,
                    "protocol": "udp",
                    "state": "ESTABLISHED",
                    "type": Type(value=2),
                },
                1,
            ),
            _SystemMetricsResult(
                {
                    "family": 1,
                    "protocol": "tcp",
                    "state": "ESTABLISHED",
                    "type": Type(value=1),
                },
                1,
            ),
        ]
        self._test_metrics("system.network.connections", expected)

    @mock.patch("psutil.Process.memory_info")
    def test_runtime_memory(self, mock_process_memory_info):

        PMem = namedtuple("PMem", ["rss", "vms"])

        mock_process_memory_info.configure_mock(
            **{"return_value": PMem(rss=1, vms=2)}
        )

        expected = [
            _SystemMetricsResult({"type": "rss"}, 1),
            _SystemMetricsResult({"type": "vms"}, 2),
        ]
        self._test_metrics(f"runtime.{self.implementation}.memory", expected)

    @mock.patch("psutil.Process.cpu_times")
    def test_runtime_cpu_time(self, mock_process_cpu_times):

        PCPUTimes = namedtuple("PCPUTimes", ["user", "system"])

        mock_process_cpu_times.configure_mock(
            **{"return_value": PCPUTimes(user=1.1, system=2.2)}
        )

        expected = [
            _SystemMetricsResult({"type": "user"}, 1.1),
            _SystemMetricsResult({"type": "system"}, 2.2),
        ]
        self._test_metrics(f"runtime.{self.implementation}.cpu_time", expected)

    @mock.patch("gc.get_count")
    def test_runtime_get_count(self, mock_gc_get_count):

        mock_gc_get_count.configure_mock(**{"return_value": (1, 2, 3)})

        expected = [
            _SystemMetricsResult({"count": "0"}, 1),
            _SystemMetricsResult({"count": "1"}, 2),
            _SystemMetricsResult({"count": "2"}, 3),
        ]
        self._test_metrics(f"runtime.{self.implementation}.gc_count", expected)
