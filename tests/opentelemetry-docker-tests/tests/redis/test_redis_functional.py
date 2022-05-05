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

import asyncio

import redis
import redis.asyncio

from opentelemetry import trace
from opentelemetry.instrumentation.redis import RedisInstrumentor
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.test_base import TestBase


class TestRedisInstrument(TestBase):
    def setUp(self):
        super().setUp()
        self.redis_client = redis.Redis(port=6379)
        self.redis_client.flushall()
        RedisInstrumentor().instrument(tracer_provider=self.tracer_provider)

    def tearDown(self):
        super().tearDown()
        RedisInstrumentor().uninstrument()

    def _check_span(self, span, name):
        self.assertEqual(span.name, name)
        self.assertIs(span.status.status_code, trace.StatusCode.UNSET)
        self.assertEqual(span.attributes.get(SpanAttributes.DB_NAME), 0)
        self.assertEqual(
            span.attributes[SpanAttributes.NET_PEER_NAME], "localhost"
        )
        self.assertEqual(span.attributes[SpanAttributes.NET_PEER_PORT], 6379)

    def test_long_command(self):
        self.redis_client.mget(*range(1000))

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self._check_span(span, "MGET")
        self.assertTrue(
            span.attributes.get(SpanAttributes.DB_STATEMENT).startswith(
                "MGET 0 1 2 3"
            )
        )
        self.assertTrue(
            span.attributes.get(SpanAttributes.DB_STATEMENT).endswith("...")
        )

    def test_basics(self):
        self.assertIsNone(self.redis_client.get("cheese"))
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self._check_span(span, "GET")
        self.assertEqual(
            span.attributes.get(SpanAttributes.DB_STATEMENT), "GET cheese"
        )
        self.assertEqual(span.attributes.get("db.redis.args_length"), 2)

    def test_pipeline_traced(self):
        with self.redis_client.pipeline(transaction=False) as pipeline:
            pipeline.set("blah", 32)
            pipeline.rpush("foo", "éé")
            pipeline.hgetall("xxx")
            pipeline.execute()

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self._check_span(span, "SET RPUSH HGETALL")
        self.assertEqual(
            span.attributes.get(SpanAttributes.DB_STATEMENT),
            "SET blah 32\nRPUSH foo éé\nHGETALL xxx",
        )
        self.assertEqual(span.attributes.get("db.redis.pipeline_length"), 3)

    def test_pipeline_immediate(self):
        with self.redis_client.pipeline() as pipeline:
            pipeline.set("a", 1)
            pipeline.immediate_execute_command("SET", "b", 2)
            pipeline.execute()

        spans = self.memory_exporter.get_finished_spans()
        # expecting two separate spans here, rather than a
        # single span for the whole pipeline
        self.assertEqual(len(spans), 2)
        span = spans[0]
        self._check_span(span, "SET")
        self.assertEqual(
            span.attributes.get(SpanAttributes.DB_STATEMENT), "SET b 2"
        )

    def test_parent(self):
        """Ensure OpenTelemetry works with redis."""
        ot_tracer = trace.get_tracer("redis_svc")

        with ot_tracer.start_as_current_span("redis_get"):
            self.assertIsNone(self.redis_client.get("cheese"))

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 2)
        child_span, parent_span = spans[0], spans[1]

        # confirm the parenting
        self.assertIsNone(parent_span.parent)
        self.assertIs(child_span.parent, parent_span.get_span_context())

        self.assertEqual(parent_span.name, "redis_get")
        self.assertEqual(parent_span.instrumentation_info.name, "redis_svc")

        self.assertEqual(child_span.name, "GET")


def async_call(coro):
    loop = asyncio.get_event_loop()
    return loop.run_until_complete(coro)


class TestAsyncRedisInstrument(TestBase):
    def setUp(self):
        super().setUp()
        self.redis_client = redis.asyncio.Redis(port=6379)
        async_call(self.redis_client.flushall())
        RedisInstrumentor().instrument(tracer_provider=self.tracer_provider)

    def tearDown(self):
        super().tearDown()
        RedisInstrumentor().uninstrument()

    def _check_span(self, span, name):
        self.assertEqual(span.name, name)
        self.assertIs(span.status.status_code, trace.StatusCode.UNSET)
        self.assertEqual(span.attributes.get(SpanAttributes.DB_NAME), 0)
        self.assertEqual(
            span.attributes[SpanAttributes.NET_PEER_NAME], "localhost"
        )
        self.assertEqual(span.attributes[SpanAttributes.NET_PEER_PORT], 6379)

    def test_long_command(self):
        async_call(self.redis_client.mget(*range(1000)))

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self._check_span(span, "MGET")
        self.assertTrue(
            span.attributes.get(SpanAttributes.DB_STATEMENT).startswith(
                "MGET 0 1 2 3"
            )
        )
        self.assertTrue(
            span.attributes.get(SpanAttributes.DB_STATEMENT).endswith("...")
        )

    def test_basics(self):
        self.assertIsNone(async_call(self.redis_client.get("cheese")))
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self._check_span(span, "GET")
        self.assertEqual(
            span.attributes.get(SpanAttributes.DB_STATEMENT), "GET cheese"
        )
        self.assertEqual(span.attributes.get("db.redis.args_length"), 2)

    def test_pipeline_traced(self):
        async def pipeline_simple():
            async with self.redis_client.pipeline(
                transaction=False
            ) as pipeline:
                pipeline.set("blah", 32)
                pipeline.rpush("foo", "éé")
                pipeline.hgetall("xxx")
                await pipeline.execute()

        async_call(pipeline_simple())

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self._check_span(span, "SET RPUSH HGETALL")
        self.assertEqual(
            span.attributes.get(SpanAttributes.DB_STATEMENT),
            "SET blah 32\nRPUSH foo éé\nHGETALL xxx",
        )
        self.assertEqual(span.attributes.get("db.redis.pipeline_length"), 3)

    def test_pipeline_immediate(self):
        async def pipeline_immediate():
            async with self.redis_client.pipeline() as pipeline:
                pipeline.set("a", 1)
                await pipeline.immediate_execute_command("SET", "b", 2)
                await pipeline.execute()

        async_call(pipeline_immediate())

        spans = self.memory_exporter.get_finished_spans()
        # expecting two separate spans here, rather than a
        # single span for the whole pipeline
        self.assertEqual(len(spans), 2)
        span = spans[0]
        self._check_span(span, "SET")
        self.assertEqual(
            span.attributes.get(SpanAttributes.DB_STATEMENT), "SET b 2"
        )

    def test_parent(self):
        """Ensure OpenTelemetry works with redis."""
        ot_tracer = trace.get_tracer("redis_svc")

        with ot_tracer.start_as_current_span("redis_get"):
            self.assertIsNone(async_call(self.redis_client.get("cheese")))

        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 2)
        child_span, parent_span = spans[0], spans[1]

        # confirm the parenting
        self.assertIsNone(parent_span.parent)
        self.assertIs(child_span.parent, parent_span.get_span_context())

        self.assertEqual(parent_span.name, "redis_get")
        self.assertEqual(parent_span.instrumentation_info.name, "redis_svc")

        self.assertEqual(child_span.name, "GET")


class TestRedisDBIndexInstrument(TestBase):
    def setUp(self):
        super().setUp()
        self.redis_client = redis.Redis(port=6379, db=10)
        self.redis_client.flushall()
        RedisInstrumentor().instrument(tracer_provider=self.tracer_provider)

    def tearDown(self):
        super().tearDown()
        RedisInstrumentor().uninstrument()

    def _check_span(self, span, name):
        self.assertEqual(span.name, name)
        self.assertIs(span.status.status_code, trace.StatusCode.UNSET)
        self.assertEqual(
            span.attributes[SpanAttributes.NET_PEER_NAME], "localhost"
        )
        self.assertEqual(span.attributes[SpanAttributes.NET_PEER_PORT], 6379)
        self.assertEqual(
            span.attributes[SpanAttributes.DB_REDIS_DATABASE_INDEX], 10
        )

    def test_get(self):
        self.assertIsNone(self.redis_client.get("foo"))
        spans = self.memory_exporter.get_finished_spans()
        self.assertEqual(len(spans), 1)
        span = spans[0]
        self._check_span(span, "GET")
        self.assertEqual(
            span.attributes.get(SpanAttributes.DB_STATEMENT), "GET foo"
        )
