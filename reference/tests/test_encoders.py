import json

from unittest import TestCase

from ddtrace.span import Span
from ddtrace.compat import msgpack_type, string_type
from ddtrace.encoding import JSONEncoder, MsgpackEncoder
from ddtrace.vendor import msgpack


class TestEncoders(TestCase):
    """
    Ensures that Encoders serialize the payload as expected.
    """
    def test_encode_traces_json(self):
        # test encoding for JSON format
        traces = []
        traces.append([
            Span(name='client.testing', tracer=None),
            Span(name='client.testing', tracer=None),
        ])
        traces.append([
            Span(name='client.testing', tracer=None),
            Span(name='client.testing', tracer=None),
        ])

        encoder = JSONEncoder()
        spans = encoder.encode_traces(traces)
        items = json.loads(spans)

        # test the encoded output that should be a string
        # and the output must be flatten
        assert isinstance(spans, string_type)
        assert len(items) == 2
        assert len(items[0]) == 2
        assert len(items[1]) == 2
        for i in range(2):
            for j in range(2):
                assert 'client.testing' == items[i][j]['name']

    def test_join_encoded_json(self):
        # test encoding for JSON format
        traces = []
        traces.append([
            Span(name='client.testing', tracer=None),
            Span(name='client.testing', tracer=None),
        ])
        traces.append([
            Span(name='client.testing', tracer=None),
            Span(name='client.testing', tracer=None),
        ])

        encoder = JSONEncoder()

        # Encode each trace on it's own
        encoded_traces = [
            encoder.encode_trace(trace)
            for trace in traces
        ]

        # Join the encoded traces together
        data = encoder.join_encoded(encoded_traces)

        # Parse the resulting data
        items = json.loads(data)

        # test the encoded output that should be a string
        # and the output must be flatten
        assert isinstance(data, string_type)
        assert len(items) == 2
        assert len(items[0]) == 2
        assert len(items[1]) == 2
        for i in range(2):
            for j in range(2):
                assert 'client.testing' == items[i][j]['name']

    def test_encode_traces_msgpack(self):
        # test encoding for MsgPack format
        traces = []
        traces.append([
            Span(name='client.testing', tracer=None),
            Span(name='client.testing', tracer=None),
        ])
        traces.append([
            Span(name='client.testing', tracer=None),
            Span(name='client.testing', tracer=None),
        ])

        encoder = MsgpackEncoder()
        spans = encoder.encode_traces(traces)
        items = msgpack.unpackb(spans)

        # test the encoded output that should be a string
        # and the output must be flatten
        assert isinstance(spans, msgpack_type)
        assert len(items) == 2
        assert len(items[0]) == 2
        assert len(items[1]) == 2
        for i in range(2):
            for j in range(2):
                assert b'client.testing' == items[i][j][b'name']

    def test_join_encoded_msgpack(self):
        # test encoding for MsgPack format
        traces = []
        traces.append([
            Span(name='client.testing', tracer=None),
            Span(name='client.testing', tracer=None),
        ])
        traces.append([
            Span(name='client.testing', tracer=None),
            Span(name='client.testing', tracer=None),
        ])

        encoder = MsgpackEncoder()

        # Encode each individual trace on it's own
        encoded_traces = [
            encoder.encode_trace(trace)
            for trace in traces
        ]
        # Join the encoded traces together
        data = encoder.join_encoded(encoded_traces)

        # Parse the encoded data
        items = msgpack.unpackb(data)

        # test the encoded output that should be a string
        # and the output must be flatten
        assert isinstance(data, msgpack_type)
        assert len(items) == 2
        assert len(items[0]) == 2
        assert len(items[1]) == 2
        for i in range(2):
            for j in range(2):
                assert b'client.testing' == items[i][j][b'name']
