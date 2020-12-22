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
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.metrics.export.aggregate import (
    HistogramAggregator,
    LastValueAggregator,
    MinMaxSumCountAggregator,
    SumAggregator,
)
from opentelemetry.sdk.metrics.view import View, ViewConfig

logging.basicConfig(stream=sys.stdout, level=logging.INFO)
logger = logging.getLogger(__name__)

metrics.set_meter_provider(MeterProvider())
meter = metrics.get_meter(__name__)
exporter = PrometheusRemoteWriteMetricsExporter(
    endpoint="http://cortex:9009/api/prom/push",
    headers={"X-Scope-Org-ID": "5"},
)
metrics.get_meter_provider().start_pipeline(meter, exporter, 1)
testing_labels = {"environment": "testing"}


# Callback to gather cpu usage
def get_cpu_usage_callback(observer):
    for (number, percent) in enumerate(psutil.cpu_percent(percpu=True)):
        labels = {"cpu_number": str(number)}
        observer.observe(percent, labels)


# Callback to gather RAM usage
def get_ram_usage_callback(observer):
    ram_percent = psutil.virtual_memory().percent
    observer.observe(ram_percent, {})


requests_counter = meter.create_counter(
    name="requests",
    description="number of requests",
    unit="1",
    value_type=int,
)

request_min_max = meter.create_counter(
    name="requests_min_max",
    description="min max sum count of requests",
    unit="1",
    value_type=int,
)

request_last_value = meter.create_counter(
    name="requests_last_value",
    description="last value number of requests",
    unit="1",
    value_type=int,
)

requests_size = meter.create_valuerecorder(
    name="requests_size",
    description="size of requests",
    unit="1",
    value_type=int,
)

requests_size_histogram = meter.create_valuerecorder(
    name="requests_size_histogram",
    description="histogram of request_size",
    unit="1",
    value_type=int,
)
requests_active = meter.create_updowncounter(
    name="requests_active",
    description="number of active requests",
    unit="1",
    value_type=int,
)

meter.register_sumobserver(
    callback=get_ram_usage_callback,
    name="ram_usage",
    description="ram usage",
    unit="1",
    value_type=float,
)

meter.register_valueobserver(
    callback=get_cpu_usage_callback,
    name="cpu_percent",
    description="per-cpu usage",
    unit="1",
    value_type=float,
)


counter_view1 = View(
    requests_counter,
    SumAggregator,
    label_keys=["environment"],
    view_config=ViewConfig.LABEL_KEYS,
)
counter_view2 = View(
    request_min_max,
    MinMaxSumCountAggregator,
    label_keys=["os_type"],
    view_config=ViewConfig.LABEL_KEYS,
)

counter_view3 = View(
    request_last_value,
    LastValueAggregator,
    label_keys=["environment"],
    view_config=ViewConfig.UNGROUPED,
)
size_view = View(
    requests_size_histogram,
    HistogramAggregator,
    label_keys=["environment"],
    aggregator_config={"bounds": [20, 40, 60, 80, 100]},
    view_config=ViewConfig.UNGROUPED,
)
meter.register_view(counter_view1)
meter.register_view(counter_view2)
meter.register_view(counter_view3)
meter.register_view(size_view)

# Load generator
num = random.randint(0, 1000)
while True:
    # counters
    requests_counter.add(num % 131 + 200, testing_labels)
    request_min_max.add(num % 181 + 200, testing_labels)
    request_last_value.add(num % 101 + 200, testing_labels)

    # updown counter
    requests_active.add(num % 7231 + 200, testing_labels)

    # value observers
    requests_size.record(num % 6101 + 100, testing_labels)
    requests_size_histogram.record(num % 113, testing_labels)
    logger.log(level=INFO, msg="completed metrics collection cycle")
    time.sleep(1)
    num += 9791
