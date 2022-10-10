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
from rich.tree import Tree

import opentelemetry.trace
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


def walk_tree(root: Tree) -> int:
    # counts the amount of spans in a tree that contains a span
    return sum(walk_tree(child) for child in root.children) + int(
        "span" in root.label
    )


def test_multiple_traces(tracer_provider):
    exporter = RichConsoleSpanExporter()
    tracer = tracer_provider.get_tracer(__name__)
    with tracer.start_as_current_span("parent_1") as parent_1:
        with tracer.start_as_current_span("child_1") as child_1:
            pass

    with tracer.start_as_current_span("parent_2") as parent_2:
        pass

    trees = exporter.spans_to_tree((parent_2, parent_1, child_1))
    # asserts that we have all traces
    assert len(trees) == 2
    traceid_1 = opentelemetry.trace.format_trace_id(parent_1.context.trace_id)

    assert traceid_1 in trees

    assert (
        opentelemetry.trace.format_trace_id(parent_2.context.trace_id) in trees
    )

    # asserts that we have exactly the number of spans we exported
    assert sum(walk_tree(tree) for tree in trees.values()) == 3

    # assert that the relationship is correct
    assert parent_1.name in trees[traceid_1].children[0].label
    assert any(
        child_1.name in child.label
        for child in trees[traceid_1].children[0].children
    )
    assert not any(
        parent_1.name in child.label
        for child in trees[traceid_1].children[0].children
    )
    assert not any(
        parent_2.name in child.label
        for child in trees[traceid_1].children[0].children
    )
