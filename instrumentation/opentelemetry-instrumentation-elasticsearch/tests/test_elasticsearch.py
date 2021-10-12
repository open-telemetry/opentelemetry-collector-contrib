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
import json
import os
import threading
from ast import literal_eval
from unittest import mock

import elasticsearch
import elasticsearch.exceptions
from elasticsearch import Elasticsearch
from elasticsearch_dsl import Search

import opentelemetry.instrumentation.elasticsearch
from opentelemetry import trace
from opentelemetry.instrumentation.elasticsearch import (
    ElasticsearchInstrumentor,
)
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import StatusCode

major_version = elasticsearch.VERSION[0]

if major_version == 7:
    from . import helpers_es7 as helpers  # pylint: disable=no-name-in-module
elif major_version == 6:
    from . import helpers_es6 as helpers  # pylint: disable=no-name-in-module
elif major_version == 5:
    from . import helpers_es5 as helpers  # pylint: disable=no-name-in-module
else:
    from . import helpers_es2 as helpers  # pylint: disable=no-name-in-module


Article = helpers.Article


@mock.patch(
    "elasticsearch.connection.http_urllib3.Urllib3HttpConnection.perform_request"
)
class TestElasticsearchIntegration(TestBase):
    def setUp(self):
        super().setUp()
        self.tracer = self.tracer_provider.get_tracer(__name__)
        ElasticsearchInstrumentor().instrument()

    def tearDown(self):
        super().tearDown()
        with self.disable_logging():
            ElasticsearchInstrumentor().uninstrument()

    def test_instrumentor(self, request_mock):
        request_mock.return_value = (1, {}, {})

        es = Elasticsearch()
        es.index(index="sw", doc_type="_doc", id=1, body={"name": "adam"})

        spans_list = self.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]

        # Check version and name in span's instrumentation info
        # self.assertEqualSpanInstrumentationInfo(span, opentelemetry.instrumentation.elasticsearch)
        self.assertEqualSpanInstrumentationInfo(
            span, opentelemetry.instrumentation.elasticsearch
        )

        # check that no spans are generated after uninstrument
        ElasticsearchInstrumentor().uninstrument()

        es.index(index="sw", doc_type="_doc", id=1, body={"name": "adam"})

        spans_list = self.get_finished_spans()
        self.assertEqual(len(spans_list), 1)

    def test_span_not_recording(self, request_mock):
        request_mock.return_value = (1, {}, {})
        mock_tracer = mock.Mock()
        mock_span = mock.Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        with mock.patch("opentelemetry.trace.get_tracer") as tracer:
            tracer.return_value = mock_tracer
            Elasticsearch()
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)

        ElasticsearchInstrumentor().uninstrument()

    def test_prefix_arg(self, request_mock):
        prefix = "prefix-from-env"
        ElasticsearchInstrumentor().uninstrument()
        ElasticsearchInstrumentor(span_name_prefix=prefix).instrument()
        request_mock.return_value = (1, {}, {})
        self._test_prefix(prefix)

    def test_prefix_env(self, request_mock):
        prefix = "prefix-from-args"
        env_var = "OTEL_PYTHON_ELASTICSEARCH_NAME_PREFIX"
        os.environ[env_var] = prefix
        ElasticsearchInstrumentor().uninstrument()
        ElasticsearchInstrumentor().instrument()
        request_mock.return_value = (1, {}, {})
        del os.environ[env_var]
        self._test_prefix(prefix)

    def _test_prefix(self, prefix):
        es = Elasticsearch()
        es.index(index="sw", doc_type="_doc", id=1, body={"name": "adam"})

        spans_list = self.get_finished_spans()
        self.assertEqual(len(spans_list), 1)
        span = spans_list[0]
        self.assertTrue(span.name.startswith(prefix))

    def test_result_values(self, request_mock):
        request_mock.return_value = (
            1,
            {},
            '{"found": false, "timed_out": true, "took": 7}',
        )
        es = Elasticsearch()
        es.get(index="test-index", doc_type="_doc", id=1)

        spans = self.get_finished_spans()

        self.assertEqual(1, len(spans))
        self.assertEqual(spans[0].name, "Elasticsearch/test-index/_doc/:id")
        self.assertEqual("False", spans[0].attributes["elasticsearch.found"])
        self.assertEqual(
            "True", spans[0].attributes["elasticsearch.timed_out"]
        )
        self.assertEqual("7", spans[0].attributes["elasticsearch.took"])

    def test_trace_error_unknown(self, request_mock):
        exc = RuntimeError("custom error")
        request_mock.side_effect = exc
        self._test_trace_error(StatusCode.ERROR, exc)

    def test_trace_error_not_found(self, request_mock):
        msg = "record not found"
        exc = elasticsearch.exceptions.NotFoundError(404, msg)
        request_mock.return_value = (1, {}, {})
        request_mock.side_effect = exc
        self._test_trace_error(StatusCode.ERROR, exc)

    def _test_trace_error(self, code, exc):
        es = Elasticsearch()
        try:
            es.get(index="test-index", doc_type="_doc", id=1)
        except Exception:  # pylint: disable=broad-except
            pass

        spans = self.get_finished_spans()
        self.assertEqual(1, len(spans))
        span = spans[0]
        self.assertFalse(span.status.is_ok)
        self.assertEqual(span.status.status_code, code)
        self.assertEqual(
            span.status.description, f"{type(exc).__name__}: {exc}"
        )

    def test_parent(self, request_mock):
        request_mock.return_value = (1, {}, {})
        es = Elasticsearch()
        with self.tracer.start_as_current_span("parent"):
            es.index(index="sw", doc_type="_doc", id=1, body={"name": "adam"})

        spans = self.get_finished_spans()
        self.assertEqual(len(spans), 2)

        parent = spans.by_name("parent")
        child = spans.by_name("Elasticsearch/sw/_doc/:id")
        self.assertIsNotNone(child.parent)
        self.assertEqual(child.parent.span_id, parent.context.span_id)

    def test_multithread(self, request_mock):
        request_mock.return_value = (1, {}, {})
        es = Elasticsearch()
        ev = threading.Event()

        # 1. Start tracing from thread-1; make thread-2 wait
        # 2. Trace something from thread-2, make thread-1 join before finishing.
        # 3. Check the spans got different parents, and are in the expected order.
        def target1(parent_span):
            with trace.use_span(parent_span):
                es.get(index="test-index", doc_type="_doc", id=1)
                ev.set()
                ev.wait()

        def target2():
            ev.wait()
            es.get(index="test-index", doc_type="_doc", id=2)
            ev.set()

        with self.tracer.start_as_current_span("parent") as span:
            t1 = threading.Thread(target=target1, args=(span,))
            t1.start()

        t2 = threading.Thread(target=target2)
        t2.start()
        t1.join()
        t2.join()

        spans = self.get_finished_spans()
        self.assertEqual(3, len(spans))

        s1 = spans.by_name("parent")
        s2 = spans.by_attr("elasticsearch.id", "1")
        s3 = spans.by_attr("elasticsearch.id", "2")

        self.assertIsNotNone(s2.parent)
        self.assertEqual(s2.parent.span_id, s1.context.span_id)
        self.assertIsNone(s3.parent)

    def test_dsl_search(self, request_mock):
        request_mock.return_value = (1, {}, '{"hits": {"hits": []}}')

        client = Elasticsearch()
        search = Search(using=client, index="test-index").filter(
            "term", author="testing"
        )
        search.execute()
        spans = self.get_finished_spans()
        span = spans[0]
        self.assertEqual(1, len(spans))
        self.assertEqual(span.name, "Elasticsearch/test-index/_search")
        self.assertIsNotNone(span.end_time)
        self.assertEqual(
            span.attributes,
            {
                SpanAttributes.DB_SYSTEM: "elasticsearch",
                "elasticsearch.url": "/test-index/_search",
                "elasticsearch.method": helpers.dsl_search_method,
                SpanAttributes.DB_STATEMENT: str(
                    {
                        "query": {
                            "bool": {
                                "filter": [{"term": {"author": "testing"}}]
                            }
                        }
                    }
                ),
            },
        )

    def test_dsl_create(self, request_mock):
        request_mock.return_value = (1, {}, {})
        client = Elasticsearch()
        Article.init(using=client)

        spans = self.get_finished_spans()
        self.assertEqual(2, len(spans))
        span1 = spans.by_attr(key="elasticsearch.method", value="HEAD")
        span2 = spans.by_attr(key="elasticsearch.method", value="PUT")

        self.assertEqual(
            span1.attributes,
            {
                SpanAttributes.DB_SYSTEM: "elasticsearch",
                "elasticsearch.url": "/test-index",
                "elasticsearch.method": "HEAD",
            },
        )

        attributes = {
            SpanAttributes.DB_SYSTEM: "elasticsearch",
            "elasticsearch.url": "/test-index",
            "elasticsearch.method": "PUT",
        }
        self.assertSpanHasAttributes(span2, attributes)
        self.assertEqual(
            literal_eval(span2.attributes[SpanAttributes.DB_STATEMENT]),
            helpers.dsl_create_statement,
        )

    def test_dsl_index(self, request_mock):
        request_mock.return_value = helpers.dsl_index_result

        client = Elasticsearch()
        article = Article(
            meta={"id": 2},
            title="About searching",
            body="A few words here, a few words there",
        )
        res = article.save(using=client)
        self.assertTrue(res)
        spans = self.get_finished_spans()
        self.assertEqual(1, len(spans))
        span = spans[0]
        self.assertEqual(span.name, helpers.dsl_index_span_name)
        attributes = {
            SpanAttributes.DB_SYSTEM: "elasticsearch",
            "elasticsearch.url": helpers.dsl_index_url,
            "elasticsearch.method": "PUT",
        }
        self.assertSpanHasAttributes(span, attributes)
        self.assertEqual(
            literal_eval(span.attributes[SpanAttributes.DB_STATEMENT]),
            {
                "body": "A few words here, a few words there",
                "title": "About searching",
            },
        )

    def test_request_hook(self, request_mock):
        request_hook_method_attribute = "request_hook.method"
        request_hook_url_attribute = "request_hook.url"
        request_hook_kwargs_attribute = "request_hook.kwargs"

        def request_hook(span, method, url, kwargs):

            attributes = {
                request_hook_method_attribute: method,
                request_hook_url_attribute: url,
                request_hook_kwargs_attribute: json.dumps(kwargs),
            }

            if span and span.is_recording():
                span.set_attributes(attributes)

        ElasticsearchInstrumentor().uninstrument()
        ElasticsearchInstrumentor().instrument(request_hook=request_hook)

        request_mock.return_value = (
            1,
            {},
            '{"found": false, "timed_out": true, "took": 7}',
        )
        es = Elasticsearch()
        index = "test-index"
        doc_id = 1
        kwargs = {"params": {"test": True}}
        es.get(index=index, doc_type="_doc", id=doc_id, **kwargs)

        spans = self.get_finished_spans()

        self.assertEqual(1, len(spans))
        self.assertEqual(
            "GET", spans[0].attributes[request_hook_method_attribute]
        )
        self.assertEqual(
            f"/{index}/_doc/{doc_id}",
            spans[0].attributes[request_hook_url_attribute],
        )
        self.assertEqual(
            json.dumps(kwargs),
            spans[0].attributes[request_hook_kwargs_attribute],
        )

    def test_response_hook(self, request_mock):
        response_attribute_name = "db.query_result"

        def response_hook(span, response):
            if span and span.is_recording():
                span.set_attribute(
                    response_attribute_name, json.dumps(response)
                )

        ElasticsearchInstrumentor().uninstrument()
        ElasticsearchInstrumentor().instrument(response_hook=response_hook)

        response_payload = {
            "took": 9,
            "timed_out": False,
            "_shards": {
                "total": 1,
                "successful": 1,
                "skipped": 0,
                "failed": 0,
            },
            "hits": {
                "total": {"value": 1, "relation": "eq"},
                "max_score": 0.18232156,
                "hits": [
                    {
                        "_index": "test-index",
                        "_type": "_doc",
                        "_id": "1",
                        "_score": 0.18232156,
                        "_source": {"name": "tester"},
                    }
                ],
            },
        }

        request_mock.return_value = (
            1,
            {},
            json.dumps(response_payload),
        )
        es = Elasticsearch()
        es.get(index="test-index", doc_type="_doc", id=1)

        spans = self.get_finished_spans()

        self.assertEqual(1, len(spans))
        self.assertEqual(
            json.dumps(response_payload),
            spans[0].attributes[response_attribute_name],
        )
