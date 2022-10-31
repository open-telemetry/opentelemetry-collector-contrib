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
import re
from collections import defaultdict
from itertools import chain
from typing import Dict, Sequence

import requests
import snappy

from opentelemetry.exporter.prometheus_remote_write.gen.remote_pb2 import (  # pylint: disable=no-name-in-module
    WriteRequest,
)
from opentelemetry.exporter.prometheus_remote_write.gen.types_pb2 import (  # pylint: disable=no-name-in-module
    Label,
    Sample,
    TimeSeries,
)
from opentelemetry.sdk.metrics import Counter
from opentelemetry.sdk.metrics import Histogram as ClientHistogram
from opentelemetry.sdk.metrics import (
    ObservableCounter,
    ObservableGauge,
    ObservableUpDownCounter,
    UpDownCounter,
)
from opentelemetry.sdk.metrics.export import (
    AggregationTemporality,
    Gauge,
    Histogram,
    Metric,
    MetricExporter,
    MetricExportResult,
    MetricsData,
    Sum,
)

logger = logging.getLogger(__name__)

PROMETHEUS_NAME_REGEX = re.compile(r"^\d|[^\w:]")
PROMETHEUS_LABEL_REGEX = re.compile(r"^\d|[^\w]")
UNDERSCORE_REGEX = re.compile(r"_+")


class PrometheusRemoteWriteMetricsExporter(MetricExporter):
    """
    Prometheus remote write metric exporter for OpenTelemetry.

    Args:
        endpoint: url where data will be sent (Required)
        basic_auth: username and password for authentication (Optional)
        headers: additional headers for remote write request (Optional)
        timeout: timeout for remote write requests in seconds, defaults to 30 (Optional)
        proxies: dict mapping request proxy protocols to proxy urls (Optional)
        tls_config: configuration for remote write TLS settings (Optional)
    """

    def __init__(
        self,
        endpoint: str,
        basic_auth: Dict = None,
        headers: Dict = None,
        timeout: int = 30,
        tls_config: Dict = None,
        proxies: Dict = None,
        resources_as_labels: bool = True,
        preferred_temporality: Dict[type, AggregationTemporality] = None,
        preferred_aggregation: Dict = None,
    ):
        self.endpoint = endpoint
        self.basic_auth = basic_auth
        self.headers = headers
        self.timeout = timeout
        self.tls_config = tls_config
        self.proxies = proxies
        self.resources_as_labels = resources_as_labels

        if not preferred_temporality:
            preferred_temporality = {
                Counter: AggregationTemporality.CUMULATIVE,
                UpDownCounter: AggregationTemporality.CUMULATIVE,
                ClientHistogram: AggregationTemporality.CUMULATIVE,
                ObservableCounter: AggregationTemporality.CUMULATIVE,
                ObservableUpDownCounter: AggregationTemporality.CUMULATIVE,
                ObservableGauge: AggregationTemporality.CUMULATIVE,
            }

        super().__init__(preferred_temporality, preferred_aggregation)

    @property
    def endpoint(self):
        return self._endpoint

    @endpoint.setter
    def endpoint(self, endpoint: str):
        if endpoint == "":
            raise ValueError("endpoint required")
        self._endpoint = endpoint

    @property
    def basic_auth(self):
        return self._basic_auth

    @basic_auth.setter
    def basic_auth(self, basic_auth: Dict):
        if basic_auth:
            if "username" not in basic_auth:
                raise ValueError("username required in basic_auth")
            if "password_file" in basic_auth:
                if "password" in basic_auth:
                    raise ValueError(
                        "basic_auth cannot contain password and password_file"
                    )
                with open(  # pylint: disable=unspecified-encoding
                    basic_auth["password_file"]
                ) as file:
                    basic_auth["password"] = file.readline().strip()
            elif "password" not in basic_auth:
                raise ValueError("password required in basic_auth")
        self._basic_auth = basic_auth

    @property
    def timeout(self):
        return self._timeout

    @timeout.setter
    def timeout(self, timeout: int):
        if timeout <= 0:
            raise ValueError("timeout must be greater than 0")
        self._timeout = timeout

    @property
    def tls_config(self):
        return self._tls_config

    @tls_config.setter
    def tls_config(self, tls_config: Dict):
        if tls_config:
            new_config = {}
            if "ca_file" in tls_config:
                new_config["ca_file"] = tls_config["ca_file"]
            if "cert_file" in tls_config and "key_file" in tls_config:
                new_config["cert_file"] = tls_config["cert_file"]
                new_config["key_file"] = tls_config["key_file"]
            elif "cert_file" in tls_config or "key_file" in tls_config:
                raise ValueError(
                    "tls_config requires both cert_file and key_file"
                )
            if "insecure_skip_verify" in tls_config:
                new_config["insecure_skip_verify"] = tls_config[
                    "insecure_skip_verify"
                ]
        self._tls_config = tls_config

    @property
    def proxies(self):
        return self._proxies

    @proxies.setter
    def proxies(self, proxies: Dict):
        self._proxies = proxies

    @property
    def headers(self):
        return self._headers

    @headers.setter
    def headers(self, headers: Dict):
        self._headers = headers

    def export(
        self,
        metrics_data: MetricsData,
        timeout_millis: float = 10_000,
        **kwargs,
    ) -> MetricExportResult:
        if not metrics_data:
            return MetricExportResult.SUCCESS
        timeseries = self._translate_data(metrics_data)
        if not timeseries:
            logger.error(
                "All records contain unsupported aggregators, export aborted"
            )
            return MetricExportResult.FAILURE
        message = self._build_message(timeseries)
        headers = self._build_headers()
        return self._send_message(message, headers)

    def _translate_data(self, data: MetricsData) -> Sequence[TimeSeries]:
        rw_timeseries = []

        for resource_metrics in data.resource_metrics:
            resource = resource_metrics.resource
            # OTLP Data model suggests combining some attrs into  job/instance
            # Should we do that here?
            if self.resources_as_labels:
                resource_labels = [
                    (n, str(v)) for n, v in resource.attributes.items()
                ]
            else:
                resource_labels = []
            # Scope name/version probably not too useful from a labeling perspective
            for scope_metrics in resource_metrics.scope_metrics:
                for metric in scope_metrics.metrics:
                    rw_timeseries.extend(
                        self._parse_metric(metric, resource_labels)
                    )
        return rw_timeseries

    def _parse_metric(
        self, metric: Metric, resource_labels: Sequence
    ) -> Sequence[TimeSeries]:
        """
        Parses the Metric & lower objects, then converts the output into
        OM TimeSeries. Returns a List of TimeSeries objects based on one Metric
        """

        # Create the metric name, will be a label later
        if metric.unit:
            # Prom. naming guidelines add unit to the name
            name = f"{metric.name}_{metric.unit}"
        else:
            name = metric.name

        # datapoints have attributes associated with them. these would be sent
        # to RW as different metrics: name & labels is a unique time series
        sample_sets = defaultdict(list)
        if isinstance(metric.data, (Gauge, Sum)):
            for dp in metric.data.data_points:
                attrs, sample = self._parse_data_point(dp, name)
                sample_sets[attrs].append(sample)
        elif isinstance(metric.data, Histogram):
            for dp in metric.data.data_points:
                dp_result = self._parse_histogram_data_point(dp, name)
                for attrs, sample in dp_result:
                    sample_sets[attrs].append(sample)
        else:
            logger.warning("Unsupported Metric Type: %s", type(metric.data))
            return []
        return self._convert_to_timeseries(sample_sets, resource_labels)

    def _convert_to_timeseries(
        self, sample_sets: Sequence[tuple], resource_labels: Sequence
    ) -> Sequence[TimeSeries]:
        timeseries = []
        for labels, samples in sample_sets.items():
            ts = TimeSeries()
            for label_name, label_value in chain(resource_labels, labels):
                # Previous implementation did not str() the names...
                ts.labels.append(self._label(label_name, str(label_value)))
            for value, timestamp in samples:
                ts.samples.append(self._sample(value, timestamp))
            timeseries.append(ts)
        return timeseries

    @staticmethod
    def _sample(value: int, timestamp: int) -> Sample:
        sample = Sample()
        sample.value = value
        sample.timestamp = timestamp
        return sample

    def _label(self, name: str, value: str) -> Label:
        label = Label()
        label.name = self._sanitize_string(name, "label")
        label.value = value
        return label

    @staticmethod
    def _sanitize_string(string: str, type_: str) -> str:
        # I Think Prometheus requires names to NOT start with a number this
        # would not catch that, but do cover the other cases. The naming rules
        # don't explicit say this, but the supplied regex implies it.
        # Got a little weird trying to do substitution with it, but can be
        # fixed if we allow numeric beginnings to metric names
        if type_ == "name":
            sanitized = PROMETHEUS_NAME_REGEX.sub("_", string)
        elif type_ == "label":
            sanitized = PROMETHEUS_LABEL_REGEX.sub("_", string)
        else:
            raise TypeError(f"Unsupported string type: {type_}")

        # Remove consecutive underscores
        # TODO: Unfortunately this clobbbers __name__
        # sanitized = UNDERSCORE_REGEX.sub("_",sanitized)

        return sanitized

    def _parse_histogram_data_point(self, data_point, name):

        sample_attr_pairs = []

        base_attrs = list(data_point.attributes.items())
        timestamp = data_point.time_unix_nano // 1_000_000

        def handle_bucket(value, bound=None, name_override=None):
            # Metric Level attributes + the bucket boundary attribute + name
            ts_attrs = base_attrs.copy()
            ts_attrs.append(
                (
                    "__name__",
                    self._sanitize_string(name_override or name, "name"),
                )
            )
            if bound:
                ts_attrs.append(("le", str(bound)))
            # Value is count of values in each bucket
            ts_sample = (value, timestamp)
            return tuple(ts_attrs), ts_sample

        for bound_pos, bound in enumerate(data_point.explicit_bounds):
            sample_attr_pairs.append(
                handle_bucket(data_point.bucket_counts[bound_pos], bound)
            )

        # Add the last label for implicit +inf bucket
        sample_attr_pairs.append(
            handle_bucket(data_point.bucket_counts[-1], bound="+Inf")
        )

        # Lastly, add series for count & sum
        sample_attr_pairs.append(
            handle_bucket(data_point.sum, name_override=f"{name}_sum")
        )
        sample_attr_pairs.append(
            handle_bucket(data_point.count, name_override=f"{name}_count")
        )
        return sample_attr_pairs

    def _parse_data_point(self, data_point, name=None):

        attrs = tuple(data_point.attributes.items()) + (
            ("__name__", self._sanitize_string(name, "name")),
        )
        sample = (data_point.value, (data_point.time_unix_nano // 1_000_000))
        return attrs, sample

    @staticmethod
    def _build_message(timeseries: Sequence[TimeSeries]) -> bytes:
        write_request = WriteRequest()
        write_request.timeseries.extend(timeseries)
        serialized_message = write_request.SerializeToString()
        return snappy.compress(serialized_message)

    def _build_headers(self) -> Dict:
        headers = {
            "Content-Encoding": "snappy",
            "Content-Type": "application/x-protobuf",
            "X-Prometheus-Remote-Write-Version": "0.1.0",
        }
        if self.headers:
            for header_name, header_value in self.headers.items():
                headers[header_name] = header_value
        return headers

    def _send_message(
        self, message: bytes, headers: Dict
    ) -> MetricExportResult:
        auth = None
        if self.basic_auth:
            auth = (self.basic_auth["username"], self.basic_auth["password"])

        cert = None
        verify = True
        if self.tls_config:
            if "ca_file" in self.tls_config:
                verify = self.tls_config["ca_file"]
            elif "insecure_skip_verify" in self.tls_config:
                verify = self.tls_config["insecure_skip_verify"]

            if (
                "cert_file" in self.tls_config
                and "key_file" in self.tls_config
            ):
                cert = (
                    self.tls_config["cert_file"],
                    self.tls_config["key_file"],
                )
        try:
            response = requests.post(
                self.endpoint,
                data=message,
                headers=headers,
                auth=auth,
                timeout=self.timeout,
                proxies=self.proxies,
                cert=cert,
                verify=verify,
            )
            if not response.ok:
                response.raise_for_status()
        except requests.exceptions.RequestException as err:
            logger.error("Export POST request failed with reason: %s", err)
            return MetricExportResult.FAILURE
        return MetricExportResult.SUCCESS

    def force_flush(self, timeout_millis: float = 10_000) -> bool:
        return True

    def shutdown(self, timeout_millis: float = 30_000, **kwargs) -> None:
        pass
