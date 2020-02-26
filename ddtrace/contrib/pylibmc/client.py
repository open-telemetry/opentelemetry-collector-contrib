from contextlib import contextmanager
import random

# 3p
from ddtrace.vendor.wrapt import ObjectProxy
import pylibmc

# project
import ddtrace
from ...constants import ANALYTICS_SAMPLE_RATE_KEY
from ...ext import SpanTypes, memcached, net
from ...internal.logger import get_logger
from ...settings import config
from .addrs import parse_addresses


# Original Client class
_Client = pylibmc.Client


log = get_logger(__name__)


class TracedClient(ObjectProxy):
    """ TracedClient is a proxy for a pylibmc.Client that times it's network operations. """

    def __init__(self, client=None, service=memcached.SERVICE, tracer=None, *args, **kwargs):
        """ Create a traced client that wraps the given memcached client.

        """
        # The client instance/service/tracer attributes are kept for compatibility
        # with the old interface: TracedClient(client=pylibmc.Client(['localhost:11211']))
        # TODO(Benjamin): Remove these in favor of patching.
        if not isinstance(client, _Client):
            # We are in the patched situation, just pass down all arguments to the pylibmc.Client
            # Note that, in that case, client isn't a real client (just the first argument)
            client = _Client(client, *args, **kwargs)
        else:
            log.warning('TracedClient instantiation is deprecated and will be remove '
                        'in future versions (0.6.0). Use patching instead (see the docs).')

        super(TracedClient, self).__init__(client)

        pin = ddtrace.Pin(service=service, tracer=tracer)
        pin.onto(self)

        # attempt to collect the pool of urls this client talks to
        try:
            self._addresses = parse_addresses(client.addresses)
        except Exception:
            log.debug('error setting addresses', exc_info=True)

    def clone(self, *args, **kwargs):
        # rewrap new connections.
        cloned = self.__wrapped__.clone(*args, **kwargs)
        traced_client = TracedClient(cloned)
        pin = ddtrace.Pin.get_from(self)
        if pin:
            pin.clone().onto(traced_client)
        return traced_client

    def get(self, *args, **kwargs):
        return self._trace_cmd('get', *args, **kwargs)

    def set(self, *args, **kwargs):
        return self._trace_cmd('set', *args, **kwargs)

    def delete(self, *args, **kwargs):
        return self._trace_cmd('delete', *args, **kwargs)

    def gets(self, *args, **kwargs):
        return self._trace_cmd('gets', *args, **kwargs)

    def touch(self, *args, **kwargs):
        return self._trace_cmd('touch', *args, **kwargs)

    def cas(self, *args, **kwargs):
        return self._trace_cmd('cas', *args, **kwargs)

    def incr(self, *args, **kwargs):
        return self._trace_cmd('incr', *args, **kwargs)

    def decr(self, *args, **kwargs):
        return self._trace_cmd('decr', *args, **kwargs)

    def append(self, *args, **kwargs):
        return self._trace_cmd('append', *args, **kwargs)

    def prepend(self, *args, **kwargs):
        return self._trace_cmd('prepend', *args, **kwargs)

    def get_multi(self, *args, **kwargs):
        return self._trace_multi_cmd('get_multi', *args, **kwargs)

    def set_multi(self, *args, **kwargs):
        return self._trace_multi_cmd('set_multi', *args, **kwargs)

    def delete_multi(self, *args, **kwargs):
        return self._trace_multi_cmd('delete_multi', *args, **kwargs)

    def _trace_cmd(self, method_name, *args, **kwargs):
        """ trace the execution of the method with the given name and will
            patch the first arg.
        """
        method = getattr(self.__wrapped__, method_name)
        with self._span(method_name) as span:

            if span and args:
                span.set_tag(memcached.QUERY, '%s %s' % (method_name, args[0]))

            return method(*args, **kwargs)

    def _trace_multi_cmd(self, method_name, *args, **kwargs):
        """ trace the execution of the multi command with the given name. """
        method = getattr(self.__wrapped__, method_name)
        with self._span(method_name) as span:

            pre = kwargs.get('key_prefix')
            if span and pre:
                span.set_tag(memcached.QUERY, '%s %s' % (method_name, pre))

            return method(*args, **kwargs)

    @contextmanager
    def _no_span(self):
        yield None

    def _span(self, cmd_name):
        """ Return a span timing the given command. """
        pin = ddtrace.Pin.get_from(self)
        if not pin or not pin.enabled():
            return self._no_span()

        span = pin.tracer.trace(
            'memcached.cmd',
            service=pin.service,
            resource=cmd_name,
            span_type=SpanTypes.CACHE)

        try:
            self._tag_span(span)
        except Exception:
            log.debug('error tagging span', exc_info=True)
        return span

    def _tag_span(self, span):
        # FIXME[matt] the host selection is buried in c code. we can't tell what it's actually
        # using, so fallback to randomly choosing one. can we do better?
        if self._addresses:
            _, host, port, _ = random.choice(self._addresses)
            span.set_meta(net.TARGET_HOST, host)
            span.set_meta(net.TARGET_PORT, port)

        # set analytics sample rate
        span.set_tag(
            ANALYTICS_SAMPLE_RATE_KEY,
            config.pylibmc.get_analytics_sample_rate()
        )
