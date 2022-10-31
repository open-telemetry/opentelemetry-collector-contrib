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
import random
import sys
import time
from logging import INFO

import psutil

from opentelemetry import metrics
from opentelemetry.exporter.prometheus_remote_write import (
    PrometheusRemoteWriteMetricsExporter,
)
from opentelemetry.metrics import Observation
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader

logging.basicConfig(stream=sys.stdout, level=logging.INFO)
logger = logging.getLogger(__name__)


testing_labels = {"environment": "testing"}

exporter = PrometheusRemoteWriteMetricsExporter(
    endpoint="http://cortex:9009/api/prom/push",
    headers={"X-Scope-Org-ID": "5"},
)
reader = PeriodicExportingMetricReader(exporter, 1000)
provider = MeterProvider(metric_readers=[reader])
metrics.set_meter_provider(provider)
meter = metrics.get_meter(__name__)


# Callback to gather cpu usage
def get_cpu_usage_callback(observer):
    for (number, percent) in enumerate(psutil.cpu_percent(percpu=True)):
        labels = {"cpu_number": str(number)}
        yield Observation(percent, labels)


# Callback to gather RAM usage
def get_ram_usage_callback(observer):
    ram_percent = psutil.virtual_memory().percent
    yield Observation(ram_percent, {})


requests_counter = meter.create_counter(
    name="requests",
    description="number of requests",
    unit="1",
)

request_min_max = meter.create_counter(
    name="requests_min_max",
    description="min max sum count of requests",
    unit="1",
)

request_last_value = meter.create_counter(
    name="requests_last_value",
    description="last value number of requests",
    unit="1",
)

requests_active = meter.create_up_down_counter(
    name="requests_active",
    description="number of active requests",
    unit="1",
)

meter.create_observable_counter(
    callbacks=[get_ram_usage_callback],
    name="ram_usage",
    description="ram usage",
    unit="1",
)

meter.create_observable_up_down_counter(
    callbacks=[get_cpu_usage_callback],
    name="cpu_percent",
    description="per-cpu usage",
    unit="1",
)

request_latency = meter.create_histogram("request_latency")

# Load generator
num = random.randint(0, 1000)
while True:
    # counters
    requests_counter.add(num % 131 + 200, testing_labels)
    request_min_max.add(num % 181 + 200, testing_labels)
    request_last_value.add(num % 101 + 200, testing_labels)

    # updown counter
    requests_active.add(num % 7231 + 200, testing_labels)

    request_latency.record(num % 92, testing_labels)
    logger.log(level=INFO, msg="completed metrics collection cycle")
    time.sleep(1)
    num += 9791
