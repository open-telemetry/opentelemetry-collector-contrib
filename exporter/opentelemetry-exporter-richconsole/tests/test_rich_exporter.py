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

import pytest

from opentelemetry.exporter.richconsole import RichConsoleSpanExporter
from opentelemetry.sdk import trace
from opentelemetry.sdk.trace.export import BatchSpanProcessor


@pytest.fixture(name="span_processor")
def fixture_span_processor():
    exporter = RichConsoleSpanExporter()
    span_processor = BatchSpanProcessor(exporter)

    yield span_processor

    span_processor.shutdown()


@pytest.fixture(name="tracer_provider")
def fixture_tracer_provider(span_processor):
    tracer_provider = trace.TracerProvider()
    tracer_provider.add_span_processor(span_processor)

    yield tracer_provider


def test_span_exporter(tracer_provider, span_processor, capsys):
    tracer = tracer_provider.get_tracer(__name__)
    span = tracer.start_span("test_span")
    span.set_attribute("key", "V4LuE")
    span.end()
    span_processor.force_flush()
    captured = capsys.readouterr()
    assert "V4LuE" in captured.out
