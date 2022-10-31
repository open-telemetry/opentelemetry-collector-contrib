import random

import pytest

import opentelemetry.test.metrictestutil as metric_util
from opentelemetry.exporter.prometheus_remote_write import (
    PrometheusRemoteWriteMetricsExporter,
)
from opentelemetry.sdk.metrics.export import (
    AggregationTemporality,
    Histogram,
    HistogramDataPoint,
    Metric,
)


@pytest.fixture
def prom_rw():
    return PrometheusRemoteWriteMetricsExporter(
        "http://victoria:8428/api/v1/write"
    )


@pytest.fixture
def metric(request):
    if hasattr(request, "param"):
        type_ = request.param
    else:
        type_ = random.choice(["gauge", "sum"])

    if type_ == "gauge":
        return metric_util._generate_gauge(
            "test.gauge", random.randint(0, 100)
        )
    if type_ == "sum":
        return metric_util._generate_sum(
            "test.sum", random.randint(0, 9_999_999_999)
        )
    if type_ == "histogram":
        return _generate_histogram("test_histogram")

    raise ValueError(f"Unsupported metric type '{type_}'.")


def _generate_histogram(name):
    dp = HistogramDataPoint(
        attributes={"foo": "bar", "baz": 42},
        start_time_unix_nano=1641946016139533244,
        time_unix_nano=1641946016139533244,
        count=5,
        sum=420,
        bucket_counts=[1, 4],
        explicit_bounds=[10.0],
        min=8,
        max=80,
    )
    data = Histogram(
        [dp],
        AggregationTemporality.CUMULATIVE,
    )
    return Metric(
        name,
        "foo",
        "tu",
        data=data,
    )
