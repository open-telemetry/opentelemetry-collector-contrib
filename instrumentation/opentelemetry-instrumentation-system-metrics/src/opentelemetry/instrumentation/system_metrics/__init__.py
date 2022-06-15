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
"""
Instrument to report system (CPU, memory, network) and
process (CPU, memory, garbage collection) metrics. By default, the
following metrics are configured:

.. code:: python

    {
        "system.cpu.time": ["idle", "user", "system", "irq"],
        "system.cpu.utilization": ["idle", "user", "system", "irq"],
        "system.memory.usage": ["used", "free", "cached"],
        "system.memory.utilization": ["used", "free", "cached"],
        "system.swap.usage": ["used", "free"],
        "system.swap.utilization": ["used", "free"],
        "system.disk.io": ["read", "write"],
        "system.disk.operations": ["read", "write"],
        "system.disk.time": ["read", "write"],
        "system.network.dropped.packets": ["transmit", "receive"],
        "system.network.packets": ["transmit", "receive"],
        "system.network.errors": ["transmit", "receive"],
        "system.network.io": ["transmit", "receive"],
        "system.network.connections": ["family", "type"],
        "runtime.memory": ["rss", "vms"],
        "runtime.cpu.time": ["user", "system"],
    }

Usage
-----

.. code:: python

    from opentelemetry.metrics import set_meter_provider
    from opentelemetry.instrumentation.system_metrics import SystemMetricsInstrumentor
    from opentelemetry.sdk.metrics import MeterProvider
    from opentelemetry.sdk.metrics.export import ConsoleMetricExporter, PeriodicExportingMetricReader

    exporter = ConsoleMetricExporter()

    set_meter_provider(MeterProvider([PeriodicExportingMetricReader(exporter)]))
    SystemMetricsInstrumentor().instrument()

    # metrics are collected asynchronously
    input("...")

    # to configure custom metrics
    configuration = {
        "system.memory.usage": ["used", "free", "cached"],
        "system.cpu.time": ["idle", "user", "system", "irq"],
        "system.network.io": ["transmit", "receive"],
        "runtime.memory": ["rss", "vms"],
        "runtime.cpu.time": ["user", "system"],
    }
    SystemMetricsInstrumentor(config=configuration).instrument()

API
---
"""

import gc
import os
from platform import python_implementation
from typing import Collection, Dict, Iterable, List, Optional

import psutil

# FIXME Remove this pyling disabling line when Github issue is cleared
# pylint: disable=no-name-in-module
from opentelemetry.instrumentation.instrumentor import BaseInstrumentor
from opentelemetry.instrumentation.system_metrics.package import _instruments
from opentelemetry.instrumentation.system_metrics.version import __version__
from opentelemetry.metrics import CallbackOptions, Observation, get_meter
from opentelemetry.sdk.util import get_dict_as_key

_DEFAULT_CONFIG = {
    "system.cpu.time": ["idle", "user", "system", "irq"],
    "system.cpu.utilization": ["idle", "user", "system", "irq"],
    "system.memory.usage": ["used", "free", "cached"],
    "system.memory.utilization": ["used", "free", "cached"],
    "system.swap.usage": ["used", "free"],
    "system.swap.utilization": ["used", "free"],
    "system.disk.io": ["read", "write"],
    "system.disk.operations": ["read", "write"],
    "system.disk.time": ["read", "write"],
    "system.network.dropped.packets": ["transmit", "receive"],
    "system.network.packets": ["transmit", "receive"],
    "system.network.errors": ["transmit", "receive"],
    "system.network.io": ["transmit", "receive"],
    "system.network.connections": ["family", "type"],
    "runtime.memory": ["rss", "vms"],
    "runtime.cpu.time": ["user", "system"],
    "runtime.gc_count": None,
}


class SystemMetricsInstrumentor(BaseInstrumentor):
    def __init__(
        self,
        labels: Optional[Dict[str, str]] = None,
        config: Optional[Dict[str, List[str]]] = None,
    ):
        super().__init__()
        if config is None:
            self._config = _DEFAULT_CONFIG
        else:
            self._config = config
        self._labels = {} if labels is None else labels
        self._meter = None
        self._python_implementation = python_implementation().lower()

        self._proc = psutil.Process(os.getpid())

        self._system_cpu_time_labels = self._labels.copy()
        self._system_cpu_utilization_labels = self._labels.copy()

        self._system_memory_usage_labels = self._labels.copy()
        self._system_memory_utilization_labels = self._labels.copy()

        self._system_swap_usage_labels = self._labels.copy()
        self._system_swap_utilization_labels = self._labels.copy()

        self._system_disk_io_labels = self._labels.copy()
        self._system_disk_operations_labels = self._labels.copy()
        self._system_disk_time_labels = self._labels.copy()
        self._system_disk_merged_labels = self._labels.copy()

        self._system_network_dropped_packets_labels = self._labels.copy()
        self._system_network_packets_labels = self._labels.copy()
        self._system_network_errors_labels = self._labels.copy()
        self._system_network_io_labels = self._labels.copy()
        self._system_network_connections_labels = self._labels.copy()

        self._runtime_memory_labels = self._labels.copy()
        self._runtime_cpu_time_labels = self._labels.copy()
        self._runtime_gc_count_labels = self._labels.copy()

    def instrumentation_dependencies(self) -> Collection[str]:
        return _instruments

    def _instrument(self, **kwargs):
        # pylint: disable=too-many-branches
        meter_provider = kwargs.get("meter_provider")
        self._meter = get_meter(
            __name__,
            __version__,
            meter_provider,
        )

        if "system.cpu.time" in self._config:
            self._meter.create_observable_counter(
                name="system.cpu.time",
                callbacks=[self._get_system_cpu_time],
                description="System CPU time",
                unit="seconds",
            )

        if "system.cpu.utilization" in self._config:
            self._meter.create_observable_gauge(
                name="system.cpu.utilization",
                callbacks=[self._get_system_cpu_utilization],
                description="System CPU utilization",
                unit="1",
            )

        if "system.memory.usage" in self._config:
            self._meter.create_observable_gauge(
                name="system.memory.usage",
                callbacks=[self._get_system_memory_usage],
                description="System memory usage",
                unit="bytes",
            )

        if "system.memory.utilization" in self._config:
            self._meter.create_observable_gauge(
                name="system.memory.utilization",
                callbacks=[self._get_system_memory_utilization],
                description="System memory utilization",
                unit="1",
            )

        if "system.swap.usage" in self._config:
            self._meter.create_observable_gauge(
                name="system.swap.usage",
                callbacks=[self._get_system_swap_usage],
                description="System swap usage",
                unit="pages",
            )

        if "system.swap.utilization" in self._config:
            self._meter.create_observable_gauge(
                name="system.swap.utilization",
                callbacks=[self._get_system_swap_utilization],
                description="System swap utilization",
                unit="1",
            )

        # TODO Add _get_system_swap_page_faults

        # self._meter.create_observable_counter(
        #     name="system.swap.page_faults",
        #     callbacks=[self._get_system_swap_page_faults],
        #     description="System swap page faults",
        #     unit="faults",
        #     value_type=int,
        # )

        # TODO Add _get_system_swap_page_operations
        # self._meter.create_observable_counter(
        #     name="system.swap.page_operations",
        #     callbacks=self._get_system_swap_page_operations,
        #     description="System swap page operations",
        #     unit="operations",
        #     value_type=int,
        # )

        if "system.disk.io" in self._config:
            self._meter.create_observable_counter(
                name="system.disk.io",
                callbacks=[self._get_system_disk_io],
                description="System disk IO",
                unit="bytes",
            )

        if "system.disk.operations" in self._config:
            self._meter.create_observable_counter(
                name="system.disk.operations",
                callbacks=[self._get_system_disk_operations],
                description="System disk operations",
                unit="operations",
            )

        if "system.disk.time" in self._config:
            self._meter.create_observable_counter(
                name="system.disk.time",
                callbacks=[self._get_system_disk_time],
                description="System disk time",
                unit="seconds",
            )

        # TODO Add _get_system_filesystem_usage

        # self.accumulator.register_valueobserver(
        #     callback=self._get_system_filesystem_usage,
        #     name="system.filesystem.usage",
        #     description="System filesystem usage",
        #     unit="bytes",
        #     value_type=int,
        # )

        # TODO Add _get_system_filesystem_utilization
        # self._meter.create_observable_gauge(
        #     callback=self._get_system_filesystem_utilization,
        #     name="system.filesystem.utilization",
        #     description="System filesystem utilization",
        #     unit="1",
        #     value_type=float,
        # )

        # TODO Filesystem information can be obtained with os.statvfs in Unix-like
        # OSs, how to do the same in Windows?

        if "system.network.dropped.packets" in self._config:
            self._meter.create_observable_counter(
                name="system.network.dropped_packets",
                callbacks=[self._get_system_network_dropped_packets],
                description="System network dropped_packets",
                unit="packets",
            )

        if "system.network.packets" in self._config:
            self._meter.create_observable_counter(
                name="system.network.packets",
                callbacks=[self._get_system_network_packets],
                description="System network packets",
                unit="packets",
            )

        if "system.network.errors" in self._config:
            self._meter.create_observable_counter(
                name="system.network.errors",
                callbacks=[self._get_system_network_errors],
                description="System network errors",
                unit="errors",
            )

        if "system.network.io" in self._config:
            self._meter.create_observable_counter(
                name="system.network.io",
                callbacks=[self._get_system_network_io],
                description="System network io",
                unit="bytes",
            )

        if "system.network.connections" in self._config:
            self._meter.create_observable_up_down_counter(
                name="system.network.connections",
                callbacks=[self._get_system_network_connections],
                description="System network connections",
                unit="connections",
            )

        if "runtime.memory" in self._config:
            self._meter.create_observable_counter(
                name=f"runtime.{self._python_implementation}.memory",
                callbacks=[self._get_runtime_memory],
                description=f"Runtime {self._python_implementation} memory",
                unit="bytes",
            )

        if "runtime.cpu.time" in self._config:
            self._meter.create_observable_counter(
                name=f"runtime.{self._python_implementation}.cpu_time",
                callbacks=[self._get_runtime_cpu_time],
                description=f"Runtime {self._python_implementation} CPU time",
                unit="seconds",
            )

        if "runtime.gc_count" in self._config:
            self._meter.create_observable_counter(
                name=f"runtime.{self._python_implementation}.gc_count",
                callbacks=[self._get_runtime_gc_count],
                description=f"Runtime {self._python_implementation} GC count",
                unit="bytes",
            )

    def _uninstrument(self, **__):
        pass

    def _get_system_cpu_time(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for system CPU time"""
        for cpu, times in enumerate(psutil.cpu_times(percpu=True)):
            for metric in self._config["system.cpu.time"]:
                if hasattr(times, metric):
                    self._system_cpu_time_labels["state"] = metric
                    self._system_cpu_time_labels["cpu"] = cpu + 1
                    yield Observation(
                        getattr(times, metric),
                        self._system_cpu_time_labels.copy(),
                    )

    def _get_system_cpu_utilization(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for system CPU utilization"""

        for cpu, times_percent in enumerate(
            psutil.cpu_times_percent(percpu=True)
        ):
            for metric in self._config["system.cpu.utilization"]:
                if hasattr(times_percent, metric):
                    self._system_cpu_utilization_labels["state"] = metric
                    self._system_cpu_utilization_labels["cpu"] = cpu + 1
                    yield Observation(
                        getattr(times_percent, metric) / 100,
                        self._system_cpu_utilization_labels.copy(),
                    )

    def _get_system_memory_usage(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for memory usage"""
        virtual_memory = psutil.virtual_memory()
        for metric in self._config["system.memory.usage"]:
            self._system_memory_usage_labels["state"] = metric
            if hasattr(virtual_memory, metric):
                yield Observation(
                    getattr(virtual_memory, metric),
                    self._system_memory_usage_labels.copy(),
                )

    def _get_system_memory_utilization(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for memory utilization"""
        system_memory = psutil.virtual_memory()

        for metric in self._config["system.memory.utilization"]:
            self._system_memory_utilization_labels["state"] = metric
            if hasattr(system_memory, metric):
                yield Observation(
                    getattr(system_memory, metric) / system_memory.total,
                    self._system_memory_utilization_labels.copy(),
                )

    def _get_system_swap_usage(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for swap usage"""
        system_swap = psutil.swap_memory()

        for metric in self._config["system.swap.usage"]:
            self._system_swap_usage_labels["state"] = metric
            if hasattr(system_swap, metric):
                yield Observation(
                    getattr(system_swap, metric),
                    self._system_swap_usage_labels.copy(),
                )

    def _get_system_swap_utilization(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for swap utilization"""
        system_swap = psutil.swap_memory()

        for metric in self._config["system.swap.utilization"]:
            if hasattr(system_swap, metric):
                self._system_swap_utilization_labels["state"] = metric
                yield Observation(
                    getattr(system_swap, metric) / system_swap.total,
                    self._system_swap_utilization_labels.copy(),
                )

    def _get_system_disk_io(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for disk IO"""
        for device, counters in psutil.disk_io_counters(perdisk=True).items():
            for metric in self._config["system.disk.io"]:
                if hasattr(counters, f"{metric}_bytes"):
                    self._system_disk_io_labels["device"] = device
                    self._system_disk_io_labels["direction"] = metric
                    yield Observation(
                        getattr(counters, f"{metric}_bytes"),
                        self._system_disk_io_labels.copy(),
                    )

    def _get_system_disk_operations(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for disk operations"""
        for device, counters in psutil.disk_io_counters(perdisk=True).items():
            for metric in self._config["system.disk.operations"]:
                if hasattr(counters, f"{metric}_count"):
                    self._system_disk_operations_labels["device"] = device
                    self._system_disk_operations_labels["direction"] = metric
                    yield Observation(
                        getattr(counters, f"{metric}_count"),
                        self._system_disk_operations_labels.copy(),
                    )

    def _get_system_disk_time(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for disk time"""
        for device, counters in psutil.disk_io_counters(perdisk=True).items():
            for metric in self._config["system.disk.time"]:
                if hasattr(counters, f"{metric}_time"):
                    self._system_disk_time_labels["device"] = device
                    self._system_disk_time_labels["direction"] = metric
                    yield Observation(
                        getattr(counters, f"{metric}_time") / 1000,
                        self._system_disk_time_labels.copy(),
                    )

    def _get_system_disk_merged(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for disk merged operations"""

        # FIXME The units in the spec is 1, it seems like it should be
        # operations or the value type should be Double

        for device, counters in psutil.disk_io_counters(perdisk=True).items():
            for metric in self._config["system.disk.time"]:
                if hasattr(counters, f"{metric}_merged_count"):
                    self._system_disk_merged_labels["device"] = device
                    self._system_disk_merged_labels["direction"] = metric
                    yield Observation(
                        getattr(counters, f"{metric}_merged_count"),
                        self._system_disk_merged_labels.copy(),
                    )

    def _get_system_network_dropped_packets(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for network dropped packets"""

        for device, counters in psutil.net_io_counters(pernic=True).items():
            for metric in self._config["system.network.dropped.packets"]:
                in_out = {"receive": "in", "transmit": "out"}[metric]
                if hasattr(counters, f"drop{in_out}"):
                    self._system_network_dropped_packets_labels[
                        "device"
                    ] = device
                    self._system_network_dropped_packets_labels[
                        "direction"
                    ] = metric
                    yield Observation(
                        getattr(counters, f"drop{in_out}"),
                        self._system_network_dropped_packets_labels.copy(),
                    )

    def _get_system_network_packets(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for network packets"""

        for device, counters in psutil.net_io_counters(pernic=True).items():
            for metric in self._config["system.network.dropped.packets"]:
                recv_sent = {"receive": "recv", "transmit": "sent"}[metric]
                if hasattr(counters, f"packets_{recv_sent}"):
                    self._system_network_packets_labels["device"] = device
                    self._system_network_packets_labels["direction"] = metric
                    yield Observation(
                        getattr(counters, f"packets_{recv_sent}"),
                        self._system_network_packets_labels.copy(),
                    )

    def _get_system_network_errors(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for network errors"""
        for device, counters in psutil.net_io_counters(pernic=True).items():
            for metric in self._config["system.network.errors"]:
                in_out = {"receive": "in", "transmit": "out"}[metric]
                if hasattr(counters, f"err{in_out}"):
                    self._system_network_errors_labels["device"] = device
                    self._system_network_errors_labels["direction"] = metric
                    yield Observation(
                        getattr(counters, f"err{in_out}"),
                        self._system_network_errors_labels.copy(),
                    )

    def _get_system_network_io(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for network IO"""

        for device, counters in psutil.net_io_counters(pernic=True).items():
            for metric in self._config["system.network.dropped.packets"]:
                recv_sent = {"receive": "recv", "transmit": "sent"}[metric]
                if hasattr(counters, f"bytes_{recv_sent}"):
                    self._system_network_io_labels["device"] = device
                    self._system_network_io_labels["direction"] = metric
                    yield Observation(
                        getattr(counters, f"bytes_{recv_sent}"),
                        self._system_network_io_labels.copy(),
                    )

    def _get_system_network_connections(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for network connections"""
        # TODO How to find the device identifier for a particular
        # connection?

        connection_counters = {}

        for net_connection in psutil.net_connections():
            for metric in self._config["system.network.connections"]:
                self._system_network_connections_labels["protocol"] = {
                    1: "tcp",
                    2: "udp",
                }[net_connection.type.value]
                self._system_network_connections_labels[
                    "state"
                ] = net_connection.status
                self._system_network_connections_labels[metric] = getattr(
                    net_connection, metric
                )

            connection_counters_key = get_dict_as_key(
                self._system_network_connections_labels
            )

            if connection_counters_key in connection_counters:
                connection_counters[connection_counters_key]["counter"] += 1
            else:
                connection_counters[connection_counters_key] = {
                    "counter": 1,
                    "labels": self._system_network_connections_labels.copy(),
                }

        for connection_counter in connection_counters.values():
            yield Observation(
                connection_counter["counter"],
                connection_counter["labels"],
            )

    def _get_runtime_memory(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for runtime memory"""
        proc_memory = self._proc.memory_info()
        for metric in self._config["runtime.memory"]:
            if hasattr(proc_memory, metric):
                self._runtime_memory_labels["type"] = metric
                yield Observation(
                    getattr(proc_memory, metric),
                    self._runtime_memory_labels.copy(),
                )

    def _get_runtime_cpu_time(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for runtime CPU time"""
        proc_cpu = self._proc.cpu_times()
        for metric in self._config["runtime.cpu.time"]:
            if hasattr(proc_cpu, metric):
                self._runtime_cpu_time_labels["type"] = metric
                yield Observation(
                    getattr(proc_cpu, metric),
                    self._runtime_cpu_time_labels.copy(),
                )

    def _get_runtime_gc_count(
        self, options: CallbackOptions
    ) -> Iterable[Observation]:
        """Observer callback for garbage collection"""
        for index, count in enumerate(gc.get_count()):
            self._runtime_gc_count_labels["count"] = str(index)
            yield Observation(count, self._runtime_gc_count_labels.copy())
