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

from typing import Dict, Sequence

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


class PrometheusRemoteWriteMetricsExporter(MetricsExporter):
    """
    Prometheus remote write metric exporter for OpenTelemetry.

    Args:
        endpoint: url where data will be sent (Required)
        basic_auth: username and password for authentication (Optional)
        bearer_token: token used for authentication (Optional)
        bearer_token_file: filepath to file containing authentication token (Optional)
        headers: additional headers for remote write request (Optional
    """

    def __init__(
        self,
        endpoint: str,
        basic_auth: Dict = None,
        bearer_token: str = None,
        bearer_token_file: str = None,
        headers: Dict = None,
    ):
        raise NotImplementedError()

    def export(
        self, export_records: Sequence[ExportRecord]
    ) -> MetricsExportResult:
        raise NotImplementedError()

    def shutdown(self) -> None:
        raise NotImplementedError()

    def convert_to_timeseries(
        self, export_records: Sequence[ExportRecord]
    ) -> Sequence[TimeSeries]:
        raise NotImplementedError()

    def convert_from_sum(self, sum_record: ExportRecord) -> TimeSeries:
        raise NotImplementedError()

    def convert_from_min_max_sum_count(
        self, min_max_sum_count_record: ExportRecord
    ) -> TimeSeries:
        raise NotImplementedError()

    def convert_from_histogram(
        self, histogram_record: ExportRecord
    ) -> TimeSeries:
        raise NotImplementedError()

    def convert_from_last_value(
        self, last_value_record: ExportRecord
    ) -> TimeSeries:
        raise NotImplementedError()

    def convert_from_value_observer(
        self, value_observer_record: ExportRecord
    ) -> TimeSeries:
        raise NotImplementedError()

    def convert_from_quantile(
        self, summary_record: ExportRecord
    ) -> TimeSeries:
        raise NotImplementedError()

    # pylint: disable=no-member
    def create_timeseries(
        self, export_record: ExportRecord, name, value: float
    ) -> TimeSeries:
        raise NotImplementedError()

    def create_sample(self, timestamp: int, value: float) -> Sample:
        raise NotImplementedError()

    def create_label(self, name: str, value: str) -> Label:
        raise NotImplementedError()

    def build_message(self, timeseries: Sequence[TimeSeries]) -> bytes:
        raise NotImplementedError()

    def get_headers(self) -> Dict:
        raise NotImplementedError()

    def send_message(
        self, message: bytes, headers: Dict
    ) -> MetricsExportResult:
        raise NotImplementedError()
