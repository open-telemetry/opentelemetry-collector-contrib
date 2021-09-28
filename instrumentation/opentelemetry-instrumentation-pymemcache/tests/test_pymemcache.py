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
from unittest import mock

import pymemcache
from pymemcache.exceptions import (
    MemcacheClientError,
    MemcacheIllegalInputError,
    MemcacheServerError,
    MemcacheUnknownCommandError,
    MemcacheUnknownError,
)

from opentelemetry import trace as trace_api
from opentelemetry.instrumentation.pymemcache import PymemcacheInstrumentor
from opentelemetry.semconv.trace import SpanAttributes
from opentelemetry.test.test_base import TestBase
from opentelemetry.trace import get_tracer

from .utils import MockSocket, _str

TEST_HOST = "localhost"
TEST_PORT = 117711


class PymemcacheClientTestCase(
    TestBase
):  # pylint: disable=too-many-public-methods
    """ Tests for a patched pymemcache.client.base.Client. """

    def setUp(self):
        super().setUp()
        PymemcacheInstrumentor().instrument()

        # pylint: disable=protected-access
        self.tracer = get_tracer(__name__)

    def tearDown(self):
        super().tearDown()
        PymemcacheInstrumentor().uninstrument()

    def make_client(self, mock_socket_values, **kwargs):
        # pylint: disable=attribute-defined-outside-init
        self.client = pymemcache.client.base.Client(
            (TEST_HOST, TEST_PORT), **kwargs
        )
        self.client.sock = MockSocket(list(mock_socket_values))
        return self.client

    def check_spans(self, spans, num_expected, queries_expected):
        """A helper for validating basic span information."""
        self.assertEqual(num_expected, len(spans))

        for span, query in zip(spans, queries_expected):
            command, *_ = query.split(" ")
            self.assertEqual(span.name, command)
            self.assertIs(span.kind, trace_api.SpanKind.CLIENT)
            self.assertEqual(
                span.attributes[SpanAttributes.NET_PEER_NAME], TEST_HOST
            )
            self.assertEqual(
                span.attributes[SpanAttributes.NET_PEER_PORT], TEST_PORT
            )
            self.assertEqual(
                span.attributes[SpanAttributes.DB_SYSTEM], "memcached"
            )
            self.assertEqual(
                span.attributes[SpanAttributes.DB_STATEMENT], query
            )

    def test_set_success(self):
        client = self.make_client([b"STORED\r\n"])
        result = client.set(b"key", b"value", noreply=False)
        self.assertTrue(result)

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["set key"])

    def test_set_not_recording(self):
        mock_tracer = mock.Mock()
        mock_span = mock.Mock()
        mock_span.is_recording.return_value = False
        mock_tracer.start_span.return_value = mock_span
        with mock.patch("opentelemetry.trace.get_tracer") as tracer:
            tracer.return_value = mock_tracer
            client = self.make_client([b"STORED\r\n"])
            result = client.set(b"key", b"value", noreply=False)
            self.assertTrue(result)
            self.assertFalse(mock_span.is_recording())
            self.assertTrue(mock_span.is_recording.called)
            self.assertFalse(mock_span.set_attribute.called)
            self.assertFalse(mock_span.set_status.called)

    def test_get_many_none_found(self):
        client = self.make_client([b"END\r\n"])
        result = client.get_many([b"key1", b"key2"])
        assert result == {}

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["get_many key1 key2"])

    def test_get_multi_none_found(self):
        client = self.make_client([b"END\r\n"])
        # alias for get_many
        result = client.get_multi([b"key1", b"key2"])
        assert result == {}

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["get_multi key1 key2"])

    def test_set_multi_success(self):
        client = self.make_client([b"STORED\r\n"])
        # Alias for set_many, a convienance function that calls set for every key
        result = client.set_multi({b"key": b"value"}, noreply=False)
        self.assertTrue(result)

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 2, ["set key", "set_multi key"])

    def test_delete_not_found(self):
        client = self.make_client([b"NOT_FOUND\r\n"])
        result = client.delete(b"key", noreply=False)
        assert result is False

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["delete key"])

    def test_incr_found(self):
        client = self.make_client([b"STORED\r\n", b"1\r\n"])
        client.set(b"key", 0, noreply=False)
        result = client.incr(b"key", 1, noreply=False)
        assert result == 1

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 2, ["set key", "incr key"])

    def test_get_found(self):
        client = self.make_client(
            [b"STORED\r\n", b"VALUE key 0 5\r\nvalue\r\nEND\r\n"]
        )
        result = client.set(b"key", b"value", noreply=False)
        result = client.get(b"key")
        assert result == b"value"

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 2, ["set key", "get key"])

    def test_decr_found(self):
        client = self.make_client([b"STORED\r\n", b"1\r\n"])
        client.set(b"key", 2, noreply=False)
        result = client.decr(b"key", 1, noreply=False)
        assert result == 1

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 2, ["set key", "decr key"])

    def test_add_stored(self):
        client = self.make_client([b"STORED\r", b"\n"])
        result = client.add(b"key", b"value", noreply=False)
        self.assertTrue(result)

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["add key"])

    def test_delete_many_found(self):
        client = self.make_client([b"STORED\r", b"\n", b"DELETED\r\n"])
        result = client.add(b"key", b"value", noreply=False)
        # a convienance function that calls delete for every key
        result = client.delete_many([b"key"], noreply=False)
        self.assertTrue(result)

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(
            spans, 3, ["add key", "delete key", "delete_many key"]
        )

    def test_set_many_success(self):
        client = self.make_client([b"STORED\r\n"])
        # a convienance function that calls set for every key
        result = client.set_many({b"key": b"value"}, noreply=False)
        self.assertTrue(result)

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 2, ["set key", "set_many key"])

    def test_set_get(self):
        client = self.make_client(
            [b"STORED\r\n", b"VALUE key 0 5\r\nvalue\r\nEND\r\n"]
        )
        client.set(b"key", b"value", noreply=False)
        result = client.get(b"key")
        assert _str(result) == "value"

        spans = self.memory_exporter.get_finished_spans()

        self.assertEqual(len(spans), 2)
        self.assertEqual(
            spans[0].attributes[SpanAttributes.NET_PEER_NAME], TEST_HOST
        )
        self.assertEqual(
            spans[0].attributes[SpanAttributes.NET_PEER_PORT], TEST_PORT
        )

    def test_append_stored(self):
        client = self.make_client([b"STORED\r\n"])
        result = client.append(b"key", b"value", noreply=False)
        self.assertTrue(result)

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["append key"])

    def test_prepend_stored(self):
        client = self.make_client([b"STORED\r\n"])
        result = client.prepend(b"key", b"value", noreply=False)
        self.assertTrue(result)

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["prepend key"])

    def test_cas_stored(self):
        client = self.make_client([b"STORED\r\n"])
        result = client.cas(b"key", b"value", b"cas", noreply=False)
        self.assertTrue(result)

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["cas key"])

    def test_cas_exists(self):
        client = self.make_client([b"EXISTS\r\n"])
        result = client.cas(b"key", b"value", b"cas", noreply=False)
        assert result is False

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["cas key"])

    def test_cas_not_found(self):
        client = self.make_client([b"NOT_FOUND\r\n"])
        result = client.cas(b"key", b"value", b"cas", noreply=False)
        assert result is None

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["cas key"])

    def test_delete_exception(self):
        client = self.make_client([Exception("fail")])

        def _delete():
            client.delete(b"key", noreply=False)

        with self.assertRaises(Exception):
            _delete()

        spans = self.memory_exporter.get_finished_spans()

        span = spans[0]

        self.assertFalse(span.status.is_ok)

        self.check_spans(spans, 1, ["delete key"])

    def test_flush_all(self):
        client = self.make_client([b"OK\r\n"])
        result = client.flush_all(noreply=False)
        self.assertTrue(result)

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["flush_all"])

    def test_incr_exception(self):
        client = self.make_client([Exception("fail")])

        def _incr():
            client.incr(b"key", 1)

        with self.assertRaises(Exception):
            _incr()

        spans = self.memory_exporter.get_finished_spans()

        span = spans[0]

        self.assertFalse(span.status.is_ok)

        self.check_spans(spans, 1, ["incr key"])

    def test_get_error(self):
        client = self.make_client([b"ERROR\r\n"])

        def _get():
            client.get(b"key")

        with self.assertRaises(MemcacheUnknownCommandError):
            _get()

        spans = self.memory_exporter.get_finished_spans()

        span = spans[0]

        self.assertFalse(span.status.is_ok)

        self.check_spans(spans, 1, ["get key"])

    def test_get_unknown_error(self):
        client = self.make_client([b"foobarbaz\r\n"])

        def _get():
            client.get(b"key")

        with self.assertRaises(MemcacheUnknownError):
            _get()

        spans = self.memory_exporter.get_finished_spans()

        span = spans[0]

        self.assertFalse(span.status.is_ok)

        self.check_spans(spans, 1, ["get key"])

    def test_gets_found(self):
        client = self.make_client([b"VALUE key 0 5 10\r\nvalue\r\nEND\r\n"])
        result = client.gets(b"key")
        assert result == (b"value", b"10")

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["gets key"])

    def test_touch_not_found(self):
        client = self.make_client([b"NOT_FOUND\r\n"])
        result = client.touch(b"key", noreply=False)
        assert result is False

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["touch key"])

    def test_set_client_error(self):
        client = self.make_client([b"CLIENT_ERROR some message\r\n"])

        def _set():
            client.set("key", "value", noreply=False)

        with self.assertRaises(MemcacheClientError):
            _set()

        spans = self.memory_exporter.get_finished_spans()

        span = spans[0]

        self.assertFalse(span.status.is_ok)

        self.check_spans(spans, 1, ["set key"])

    def test_set_server_error(self):
        client = self.make_client([b"SERVER_ERROR some message\r\n"])

        def _set():
            client.set(b"key", b"value", noreply=False)

        with self.assertRaises(MemcacheServerError):
            _set()

        spans = self.memory_exporter.get_finished_spans()

        span = spans[0]

        self.assertFalse(span.status.is_ok)

        self.check_spans(spans, 1, ["set key"])

    def test_set_key_with_space(self):
        client = self.make_client([b""])

        def _set():
            client.set(b"key has space", b"value", noreply=False)

        with self.assertRaises(MemcacheIllegalInputError):
            _set()

        spans = self.memory_exporter.get_finished_spans()

        span = spans[0]

        self.assertFalse(span.status.is_ok)

        self.check_spans(spans, 1, ["set key has space"])

    def test_quit(self):
        client = self.make_client([])
        assert client.quit() is None

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["quit"])

    def test_replace_not_stored(self):
        client = self.make_client([b"NOT_STORED\r\n"])
        result = client.replace(b"key", b"value", noreply=False)
        assert result is False

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["replace key"])

    def test_version_success(self):
        client = self.make_client(
            [b"VERSION 1.2.3\r\n"], default_noreply=False
        )
        result = client.version()
        assert result == b"1.2.3"

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["version"])

    def test_stats(self):
        client = self.make_client([b"STAT fake_stats 1\r\n", b"END\r\n"])
        result = client.stats()
        assert client.sock.send_bufs == [b"stats \r\n"]
        assert result == {b"fake_stats": 1}

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 1, ["stats"])

    def test_uninstrumented(self):
        PymemcacheInstrumentor().uninstrument()

        client = self.make_client(
            [b"STORED\r\n", b"VALUE key 0 5\r\nvalue\r\nEND\r\n"]
        )
        client.set(b"key", b"value", noreply=False)
        result = client.get(b"key")
        assert _str(result) == "value"

        spans = self.memory_exporter.get_finished_spans()

        self.assertEqual(len(spans), 0)

        PymemcacheInstrumentor().instrument()


class PymemcacheHashClientTestCase(TestBase):
    """ Tests for a patched pymemcache.client.hash.HashClient. """

    def setUp(self):
        super().setUp()
        PymemcacheInstrumentor().instrument()

        # pylint: disable=protected-access
        self.tracer = get_tracer(__name__)

    def tearDown(self):
        super().tearDown()
        PymemcacheInstrumentor().uninstrument()

    def make_client_pool(
        self, hostname, mock_socket_values, serializer=None, **kwargs
    ):  # pylint: disable=no-self-use
        mock_client = pymemcache.client.base.Client(
            hostname, serializer=serializer, **kwargs
        )
        mock_client.sock = MockSocket(mock_socket_values)
        client = pymemcache.client.base.PooledClient(
            hostname, serializer=serializer
        )
        client.client_pool = pymemcache.pool.ObjectPool(lambda: mock_client)
        return mock_client

    def make_client(self, *mock_socket_values, **kwargs):
        current_port = TEST_PORT

        # pylint: disable=import-outside-toplevel
        from pymemcache.client.hash import HashClient

        # pylint: disable=attribute-defined-outside-init
        self.client = HashClient([], **kwargs)
        ip = TEST_HOST

        for vals in mock_socket_values:
            url_string = f"{ip}:{current_port}"
            clnt_pool = self.make_client_pool(
                (ip, current_port), vals, **kwargs
            )
            self.client.clients[url_string] = clnt_pool
            self.client.hasher.add_node(url_string)
            current_port += 1
        return self.client

    def check_spans(self, spans, num_expected, queries_expected):
        """A helper for validating basic span information."""
        self.assertEqual(num_expected, len(spans))

        for span, query in zip(spans, queries_expected):
            command, *_ = query.split(" ")
            self.assertEqual(span.name, command)
            self.assertIs(span.kind, trace_api.SpanKind.CLIENT)
            self.assertEqual(
                span.attributes[SpanAttributes.NET_PEER_NAME], TEST_HOST
            )
            self.assertEqual(
                span.attributes[SpanAttributes.NET_PEER_PORT], TEST_PORT
            )
            self.assertEqual(
                span.attributes[SpanAttributes.DB_SYSTEM], "memcached"
            )
            self.assertEqual(
                span.attributes[SpanAttributes.DB_STATEMENT], query
            )

    def test_delete_many_found(self):
        client = self.make_client([b"STORED\r", b"\n", b"DELETED\r\n"])
        result = client.add(b"key", b"value", noreply=False)
        result = client.delete_many([b"key"], noreply=False)
        self.assertTrue(result)

        spans = self.memory_exporter.get_finished_spans()

        self.check_spans(spans, 2, ["add key", "delete key"])
