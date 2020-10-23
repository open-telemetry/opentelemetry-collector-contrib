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

from opentelemetry import trace
from opentelemetry.context import attach, detach, set_value
from opentelemetry.exporter.opencensus.trace_exporter import (
    OpenCensusSpanExporter,
)
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import SimpleExportSpanProcessor
from opentelemetry.test.test_base import TestBase


class ExportStatusSpanProcessor(SimpleExportSpanProcessor):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.export_status = []

    def on_end(self, span):
        token = attach(set_value("suppress_instrumentation", True))
        self.export_status.append(self.span_exporter.export((span,)))
        detach(token)


class TestOpenCensusSpanExporter(TestBase):
    def setUp(self):
        super().setUp()

        trace.set_tracer_provider(TracerProvider())
        self.tracer = trace.get_tracer(__name__)
        self.span_processor = ExportStatusSpanProcessor(
            OpenCensusSpanExporter(
                service_name="basic-service", endpoint="localhost:55678"
            )
        )

        trace.get_tracer_provider().add_span_processor(self.span_processor)

    def test_export(self):
        with self.tracer.start_as_current_span("foo"):
            with self.tracer.start_as_current_span("bar"):
                with self.tracer.start_as_current_span("baz"):
                    pass

        self.assertTrue(len(self.span_processor.export_status), 3)

        for export_status in self.span_processor.export_status:
            self.assertEqual(export_status.name, "SUCCESS")
            self.assertEqual(export_status.value, 0)
