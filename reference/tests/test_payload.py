import math

from ddtrace.encoding import get_encoder, JSONEncoder
from ddtrace.payload import Payload, PayloadFull
from ddtrace.span import Span

from .base import BaseTracerTestCase

import pytest


class PayloadTestCase(BaseTracerTestCase):
    def test_init(self):
        """
        When calling `Payload.init`
            With an encoder
                We use that encoder
            With no encoder
                We use the default encoder
        """
        default_encoder_type = type(get_encoder())

        payload = Payload()
        self.assertIsInstance(payload.encoder, default_encoder_type)

        json_encoder = JSONEncoder()
        payload = Payload(encoder=json_encoder)
        self.assertEqual(payload.encoder, json_encoder)

    def test_add_trace(self):
        """
        When calling `Payload.add_trace`
            With a falsey value
                Nothing is added to the payload
            With a trace
                We encode and add the trace to the payload
                We increment the payload size by the expected amount
        """
        payload = Payload()

        # Add falsey traces
        for val in (False, None, 0, '', [], dict()):
            payload.add_trace(val)
        self.assertEqual(payload.length, 0)
        self.assertTrue(payload.empty)

        # Add a single trace to the payload
        trace = [Span(self.tracer, name='root.span'), Span(self.tracer, name='child.span')]
        payload.add_trace(trace)

        self.assertEqual(payload.length, 1)
        self.assertFalse(payload.empty)

    def test_get_payload(self):
        """
        When calling `Payload.get_payload`
            With no traces
                We return the appropriate data
            With traces
                We return the appropriate data
        """
        payload = Payload()

        # No traces
        self.assertTrue(payload.empty)
        encoded_data = payload.get_payload()
        decoded_data = payload.encoder.decode(encoded_data)
        self.assertEqual(decoded_data, [])

        # Add traces to the payload
        for _ in range(5):
            trace = [Span(self.tracer, name='root.span'), Span(self.tracer, name='child.span')]
            payload.add_trace(trace)

        self.assertEqual(payload.length, 5)
        self.assertFalse(payload.empty)

        # Assert the payload generated from Payload
        encoded_data = payload.get_payload()
        decoded_data = payload.encoder.decode(encoded_data)
        self.assertEqual(len(decoded_data), 5)
        for trace in decoded_data:
            self.assertEqual(len(trace), 2)
            self.assertEqual(trace[0][b'name'], b'root.span')
            self.assertEqual(trace[1][b'name'], b'child.span')

    def test_full(self):
        payload = Payload()

        # Empty
        self.assertTrue(payload.empty)

        # Trace and it's size in bytes
        trace = [Span(self.tracer, 'root.span'), Span(self.tracer, 'child.span')]
        trace_size = len(payload.encoder.encode_trace(trace))

        # Number of traces before we hit the max size limit and are considered full
        num_traces = int(math.floor(payload.max_payload_size / trace_size))

        # Add the traces
        for _ in range(num_traces):
            payload.add_trace(trace)

        # Just confirm
        self.assertEqual(payload.length, num_traces)

        with pytest.raises(PayloadFull):
            payload.add_trace(trace)

        # Just confirm again
        self.assertEqual(payload.length, num_traces)
