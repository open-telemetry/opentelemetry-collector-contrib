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
from typing import Dict, Sequence

import requests
import snappy

from opentelemetry.exporter.prometheus_remote_write.gen.remote_pb2 import (
    WriteRequest,
)
from opentelemetry.exporter.prometheus_remote_write.gen.types_pb2 import (
    Label,
    Sample,
    TimeSeries,
)
from opentelemetry.sdk.metrics.export import (
    ExportRecord,
    MetricsExporter,
    MetricsExportResult,
)
from opentelemetry.sdk.metrics.export.aggregate import (
    HistogramAggregator,
    LastValueAggregator,
    MinMaxSumCountAggregator,
    SumAggregator,
    ValueObserverAggregator,
)

logger = logging.getLogger(__name__)


class PrometheusRemoteWriteMetricsExporter(MetricsExporter):
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
    ):
        self.endpoint = endpoint
        self.basic_auth = basic_auth
        self.headers = headers
        self.timeout = timeout
        self.tls_config = tls_config
        self.proxies = proxies

        self.converter_map = {
            MinMaxSumCountAggregator: self._convert_from_min_max_sum_count,
            SumAggregator: self._convert_from_sum,
            HistogramAggregator: self._convert_from_histogram,
            LastValueAggregator: self._convert_from_last_value,
            ValueObserverAggregator: self._convert_from_value_observer,
        }

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
                with open(basic_auth["password_file"]) as file:
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
        self, export_records: Sequence[ExportRecord]
    ) -> MetricsExportResult:
        if not export_records:
            return MetricsExportResult.SUCCESS
        timeseries = self._convert_to_timeseries(export_records)
        if not timeseries:
            logger.error(
                "All records contain unsupported aggregators, export aborted"
            )
            return MetricsExportResult.FAILURE
        message = self._build_message(timeseries)
        headers = self._build_headers()
        return self._send_message(message, headers)

    def shutdown(self) -> None:
        pass

    def _convert_to_timeseries(
        self, export_records: Sequence[ExportRecord]
    ) -> Sequence[TimeSeries]:
        timeseries = []
        for export_record in export_records:
            aggregator_type = type(export_record.aggregator)
            converter = self.converter_map.get(aggregator_type)
            if converter:
                timeseries.extend(converter(export_record))
            else:
                logger.warning(
                    "%s aggregator is not supported, record dropped",
                    aggregator_type,
                )
        return timeseries

    def _convert_from_sum(
        self, sum_record: ExportRecord
    ) -> Sequence[TimeSeries]:
        return [
            self._create_timeseries(
                sum_record,
                sum_record.instrument.name + "_sum",
                sum_record.aggregator.checkpoint,
            )
        ]

    def _convert_from_min_max_sum_count(
        self, min_max_sum_count_record: ExportRecord
    ) -> Sequence[TimeSeries]:
        timeseries = []
        for agg_type in ["min", "max", "sum", "count"]:
            name = min_max_sum_count_record.instrument.name + "_" + agg_type
            value = getattr(
                min_max_sum_count_record.aggregator.checkpoint, agg_type
            )
            timeseries.append(
                self._create_timeseries(min_max_sum_count_record, name, value)
            )
        return timeseries

    def _convert_from_histogram(
        self, histogram_record: ExportRecord
    ) -> Sequence[TimeSeries]:
        timeseries = []
        for bound in histogram_record.aggregator.checkpoint.keys():
            bound_str = "+Inf" if bound == float("inf") else str(bound)
            value = histogram_record.aggregator.checkpoint[bound]
            timeseries.append(
                self._create_timeseries(
                    histogram_record,
                    histogram_record.instrument.name + "_histogram",
                    value,
                    extra_label=("le", bound_str),
                )
            )
        return timeseries

    def _convert_from_last_value(
        self, last_value_record: ExportRecord
    ) -> Sequence[TimeSeries]:
        return [
            self._create_timeseries(
                last_value_record,
                last_value_record.instrument.name + "_last",
                last_value_record.aggregator.checkpoint,
            )
        ]

    def _convert_from_value_observer(
        self, value_observer_record: ExportRecord
    ) -> Sequence[TimeSeries]:
        timeseries = []
        for agg_type in ["min", "max", "sum", "count", "last"]:
            timeseries.append(
                self._create_timeseries(
                    value_observer_record,
                    value_observer_record.instrument.name + "_" + agg_type,
                    getattr(
                        value_observer_record.aggregator.checkpoint, agg_type
                    ),
                )
            )
        return timeseries

    # TODO: Implement convert from quantile once supported by SDK for Prometheus Summaries
    def _convert_from_quantile(
        self, summary_record: ExportRecord
    ) -> Sequence[TimeSeries]:
        raise NotImplementedError()

    # pylint: disable=no-member,no-self-use
    def _create_timeseries(
        self,
        export_record: ExportRecord,
        name: str,
        value: float,
        extra_label: (str, str) = None,
    ) -> TimeSeries:
        timeseries = TimeSeries()
        seen = set()

        def add_label(label_name: str, label_value: str):
            # Label name must contain only alphanumeric characters and underscores
            label_name = re.sub("[^\\w_]", "_", label_name)
            if label_name not in seen:
                label = Label()
                label.name = label_name
                label.value = label_value
                timeseries.labels.append(label)
                seen.add(label_name)
            else:
                logger.warning(
                    "Duplicate label with name %s and value %s",
                    label_name,
                    label_value,
                )

        # The __name__ label is required by PromQL as its value appears as the metric_name
        add_label("__name__", name)
        if extra_label:
            add_label(extra_label[0], extra_label[1])
        if export_record.resource.attributes:
            for (
                label_name,
                label_value,
            ) in export_record.resource.attributes.items():
                add_label(label_name, str(label_value))
        if export_record.labels:
            for [label_name, label_value] in export_record.labels:
                add_label(label_name, label_value)

        sample = Sample()
        sample.timestamp = int(
            export_record.aggregator.last_update_timestamp / 1000000
        )
        sample.value = value
        timeseries.samples.append(sample)
        return timeseries

    # pylint: disable=no-member,no-self-use
    def _build_message(self, timeseries: Sequence[TimeSeries]) -> bytes:
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
    ) -> MetricsExportResult:
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
            return MetricsExportResult.FAILURE
        return MetricsExportResult.SUCCESS
