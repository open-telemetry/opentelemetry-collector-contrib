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

"system_memory": ["total", "available", "used", "free"],
"system_cpu": ["user", "system", "idle"],
"network_bytes": ["bytes_recv", "bytes_sent"],
"runtime_memory": ["rss", "vms"],
"runtime_cpu": ["user", "system"],


Usage
-----

.. code:: python

    from opentelemetry import metrics
    from opentelemetry.instrumentation.system_metrics import SystemMetrics
    from opentelemetry.sdk.metrics import MeterProvider,
    from opentelemetry.sdk.metrics.export import ConsoleMetricsExporter

    metrics.set_meter_provider(MeterProvider())
    exporter = ConsoleMetricsExporter()
    SystemMetrics(exporter)

    # metrics are collected asynchronously
    input("...")

    # to configure custom metrics
    configuration = {
        "system_memory": ["total", "available", "used", "free", "active", "inactive", "wired"],
        "system_cpu": ["user", "nice", "system", "idle"],
        "network_bytes": ["bytes_recv", "bytes_sent"],
        "runtime_memory": ["rss", "vms"],
        "runtime_cpu": ["user", "system"],
    }
    SystemMetrics(exporter, config=configuration)

API
---
"""

import gc
import os
import typing

import psutil

from opentelemetry import metrics
from opentelemetry.sdk.metrics import ValueObserver
from opentelemetry.sdk.metrics.export import MetricsExporter
from opentelemetry.sdk.metrics.export.controller import PushController


class SystemMetrics:
    def __init__(
        self,
        exporter: MetricsExporter,
        interval: int = 30,
        labels: typing.Optional[typing.Dict[str, str]] = None,
        config: typing.Optional[typing.Dict[str, typing.List[str]]] = None,
    ):
        self._labels = {} if labels is None else labels
        self.meter = metrics.get_meter(__name__)
        self.controller = PushController(
            meter=self.meter, exporter=exporter, interval=interval
        )
        if config is None:
            self._config = {
                "system_memory": ["total", "available", "used", "free"],
                "system_cpu": ["user", "system", "idle"],
                "network_bytes": ["bytes_recv", "bytes_sent"],
                "runtime_memory": ["rss", "vms"],
                "runtime_cpu": ["user", "system"],
            }
        else:
            self._config = config
        self._proc = psutil.Process(os.getpid())
        self._system_memory_labels = {}
        self._system_cpu_labels = {}
        self._network_bytes_labels = {}
        self._runtime_memory_labels = {}
        self._runtime_cpu_labels = {}
        self._runtime_gc_labels = {}
        # create the label set for each observer once
        for key, value in self._labels.items():
            self._system_memory_labels[key] = value
            self._system_cpu_labels[key] = value
            self._network_bytes_labels[key] = value
            self._runtime_memory_labels[key] = value
            self._runtime_gc_labels[key] = value

        self.meter.register_observer(
            callback=self._get_system_memory,
            name="system.mem",
            description="System memory",
            unit="bytes",
            value_type=int,
            observer_type=ValueObserver,
        )

        self.meter.register_observer(
            callback=self._get_system_cpu,
            name="system.cpu",
            description="System CPU",
            unit="seconds",
            value_type=float,
            observer_type=ValueObserver,
        )

        self.meter.register_observer(
            callback=self._get_network_bytes,
            name="system.net.bytes",
            description="System network bytes",
            unit="bytes",
            value_type=int,
            observer_type=ValueObserver,
        )

        self.meter.register_observer(
            callback=self._get_runtime_memory,
            name="runtime.python.mem",
            description="Runtime memory",
            unit="bytes",
            value_type=int,
            observer_type=ValueObserver,
        )

        self.meter.register_observer(
            callback=self._get_runtime_cpu,
            name="runtime.python.cpu",
            description="Runtime CPU",
            unit="seconds",
            value_type=float,
            observer_type=ValueObserver,
        )

        self.meter.register_observer(
            callback=self._get_runtime_gc_count,
            name="runtime.python.gc.count",
            description="Runtime: gc objects",
            unit="objects",
            value_type=int,
            observer_type=ValueObserver,
        )

    def _get_system_memory(self, observer: metrics.ValueObserver) -> None:
        """Observer callback for memory available

        Args:
            observer: the observer to update
        """
        system_memory = psutil.virtual_memory()
        for metric in self._config["system_memory"]:
            self._system_memory_labels["type"] = metric
            observer.observe(
                getattr(system_memory, metric), self._system_memory_labels
            )

    def _get_system_cpu(self, observer: metrics.ValueObserver) -> None:
        """Observer callback for system cpu

        Args:
            observer: the observer to update
        """
        cpu_times = psutil.cpu_times()
        for _type in self._config["system_cpu"]:
            self._system_cpu_labels["type"] = _type
            observer.observe(
                getattr(cpu_times, _type), self._system_cpu_labels
            )

    def _get_network_bytes(self, observer: metrics.ValueObserver) -> None:
        """Observer callback for network bytes

        Args:
            observer: the observer to update
        """
        net_io = psutil.net_io_counters()
        for _type in self._config["network_bytes"]:
            self._network_bytes_labels["type"] = _type
            observer.observe(
                getattr(net_io, _type), self._network_bytes_labels
            )

    def _get_runtime_memory(self, observer: metrics.ValueObserver) -> None:
        """Observer callback for runtime memory

        Args:
            observer: the observer to update
        """
        proc_memory = self._proc.memory_info()
        for _type in self._config["runtime_memory"]:
            self._runtime_memory_labels["type"] = _type
            observer.observe(
                getattr(proc_memory, _type), self._runtime_memory_labels
            )

    def _get_runtime_cpu(self, observer: metrics.ValueObserver) -> None:
        """Observer callback for runtime CPU

        Args:
            observer: the observer to update
        """
        proc_cpu = self._proc.cpu_times()
        for _type in self._config["runtime_cpu"]:
            self._runtime_cpu_labels["type"] = _type
            observer.observe(
                getattr(proc_cpu, _type), self._runtime_cpu_labels
            )

    def _get_runtime_gc_count(self, observer: metrics.ValueObserver) -> None:
        """Observer callback for garbage collection

        Args:
            observer: the observer to update
        """
        gc_count = gc.get_count()
        for index, count in enumerate(gc_count):
            self._runtime_gc_labels["count"] = str(index)
            observer.observe(count, self._runtime_gc_labels)
