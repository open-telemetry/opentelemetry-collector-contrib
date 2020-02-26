from opentracing import InvalidCarrierException, SpanContextCorruptedException
from ddtrace.propagation.http import HTTPPropagator as DDHTTPPropagator

from ...internal.logger import get_logger
from ..span_context import SpanContext
from .propagator import Propagator


log = get_logger(__name__)

HTTP_BAGGAGE_PREFIX = 'ot-baggage-'
HTTP_BAGGAGE_PREFIX_LEN = len(HTTP_BAGGAGE_PREFIX)


class HTTPPropagator(Propagator):
    """OpenTracing compatible HTTP_HEADER and TEXT_MAP format propagator.

    `HTTPPropagator` provides compatibility by using existing OpenTracing
    compatible methods from the ddtracer along with new logic supporting the
    outstanding OpenTracing-defined functionality.
    """

    __slots__ = ['_dd_propagator']

    def __init__(self):
        self._dd_propagator = DDHTTPPropagator()

    def inject(self, span_context, carrier):
        """Inject a span context into a carrier.

        *span_context* is injected into the carrier by first using an
        :class:`ddtrace.propagation.http.HTTPPropagator` to inject the ddtracer
        specific fields.

        Then the baggage is injected into *carrier*.

        :param span_context: span context to inject.

        :param carrier: carrier to inject into.
        """
        if not isinstance(carrier, dict):
            raise InvalidCarrierException('propagator expects carrier to be a dict')

        self._dd_propagator.inject(span_context._dd_context, carrier)

        # Add the baggage
        if span_context.baggage is not None:
            for key in span_context.baggage:
                carrier[HTTP_BAGGAGE_PREFIX + key] = span_context.baggage[key]

    def extract(self, carrier):
        """Extract a span context from a carrier.

        :class:`ddtrace.propagation.http.HTTPPropagator` is used to extract
        ddtracer supported fields into a `ddtrace.Context` context which is
        combined with new logic to extract the baggage which is returned in an
        OpenTracing compatible span context.

        :param carrier: carrier to extract from.

        :return: extracted span context.
        """
        if not isinstance(carrier, dict):
            raise InvalidCarrierException('propagator expects carrier to be a dict')

        ddspan_ctx = self._dd_propagator.extract(carrier)

        # if the dd propagator fails then it will return a new empty span
        # context (with trace_id=None), we however want to raise an exception
        # if this occurs.
        if not ddspan_ctx.trace_id:
            raise SpanContextCorruptedException('failed to extract span context')

        baggage = {}
        for key in carrier:
            if key.startswith(HTTP_BAGGAGE_PREFIX):
                baggage[key[HTTP_BAGGAGE_PREFIX_LEN:]] = carrier[key]

        return SpanContext(ddcontext=ddspan_ctx, baggage=baggage)
