import opentracing
from opentracing import Format
from opentracing.scope_managers import ThreadLocalScopeManager

import ddtrace
from ddtrace import Tracer as DatadogTracer
from ddtrace.constants import FILTERS_KEY
from ddtrace.settings import ConfigException
from ddtrace.utils import merge_dicts
from ddtrace.utils.config import get_application_name

from ..internal.logger import get_logger
from .propagation import HTTPPropagator
from .span import Span
from .span_context import SpanContext
from .settings import ConfigKeys as keys, config_invalid_keys
from .utils import get_context_provider_for_scope_manager

log = get_logger(__name__)

DEFAULT_CONFIG = {
    keys.AGENT_HOSTNAME: 'localhost',
    keys.AGENT_HTTPS: False,
    keys.AGENT_PORT: 8126,
    keys.DEBUG: False,
    keys.ENABLED: True,
    keys.GLOBAL_TAGS: {},
    keys.SAMPLER: None,
    keys.PRIORITY_SAMPLING: None,
    keys.SETTINGS: {
        FILTERS_KEY: [],
    },
}


class Tracer(opentracing.Tracer):
    """A wrapper providing an OpenTracing API for the Datadog tracer."""

    def __init__(self, service_name=None, config=None, scope_manager=None, dd_tracer=None):
        """Initialize a new Datadog opentracer.

        :param service_name: (optional) the name of the service that this
            tracer will be used with. Note if not provided, a service name will
            try to be determined based off of ``sys.argv``. If this fails a
            :class:`ddtrace.settings.ConfigException` will be raised.
        :param config: (optional) a configuration object to specify additional
            options. See the documentation for further information.
        :param scope_manager: (optional) the scope manager for this tracer to
            use. The available managers are listed in the Python OpenTracing repo
            here: https://github.com/opentracing/opentracing-python#scope-managers.
            If ``None`` is provided, defaults to
            :class:`opentracing.scope_managers.ThreadLocalScopeManager`.
        :param dd_tracer: (optional) the Datadog tracer for this tracer to use. This
            should only be passed if a custom Datadog tracer is being used. Defaults
            to the global ``ddtrace.tracer`` tracer.
        """
        # Merge the given config with the default into a new dict
        config = config or {}
        self._config = merge_dicts(DEFAULT_CONFIG, config)

        # Pull out commonly used properties for performance
        self._service_name = service_name or get_application_name()
        self._enabled = self._config.get(keys.ENABLED)
        self._debug = self._config.get(keys.DEBUG)

        if self._debug:
            # Ensure there are no typos in any of the keys
            invalid_keys = config_invalid_keys(self._config)
            if invalid_keys:
                str_invalid_keys = ','.join(invalid_keys)
                raise ConfigException('invalid key(s) given (%s)'.format(str_invalid_keys))

        if not self._service_name:
            raise ConfigException(""" Cannot detect the \'service_name\'.
                                      Please set the \'service_name=\'
                                      keyword argument.
                                  """)

        self._scope_manager = scope_manager or ThreadLocalScopeManager()

        dd_context_provider = get_context_provider_for_scope_manager(self._scope_manager)

        self._dd_tracer = dd_tracer or ddtrace.tracer or DatadogTracer()
        self._dd_tracer.set_tags(self._config.get(keys.GLOBAL_TAGS))
        self._dd_tracer.configure(enabled=self._enabled,
                                  hostname=self._config.get(keys.AGENT_HOSTNAME),
                                  https=self._config.get(keys.AGENT_HTTPS),
                                  port=self._config.get(keys.AGENT_PORT),
                                  sampler=self._config.get(keys.SAMPLER),
                                  settings=self._config.get(keys.SETTINGS),
                                  priority_sampling=self._config.get(keys.PRIORITY_SAMPLING),
                                  context_provider=dd_context_provider,
                                  )
        self._propagators = {
            Format.HTTP_HEADERS: HTTPPropagator(),
            Format.TEXT_MAP: HTTPPropagator(),
        }

    @property
    def scope_manager(self):
        """Returns the scope manager being used by this tracer."""
        return self._scope_manager

    def start_active_span(self, operation_name, child_of=None, references=None,
                          tags=None, start_time=None, ignore_active_span=False,
                          finish_on_close=True):
        """Returns a newly started and activated `Scope`.
        The returned `Scope` supports with-statement contexts. For example::

            with tracer.start_active_span('...') as scope:
                scope.span.set_tag('http.method', 'GET')
                do_some_work()
            # Span.finish() is called as part of Scope deactivation through
            # the with statement.

        It's also possible to not finish the `Span` when the `Scope` context
        expires::

            with tracer.start_active_span('...',
                                          finish_on_close=False) as scope:
                scope.span.set_tag('http.method', 'GET')
                do_some_work()
            # Span.finish() is not called as part of Scope deactivation as
            # `finish_on_close` is `False`.

        :param operation_name: name of the operation represented by the new
            span from the perspective of the current service.
        :param child_of: (optional) a Span or SpanContext instance representing
            the parent in a REFERENCE_CHILD_OF Reference. If specified, the
            `references` parameter must be omitted.
        :param references: (optional) a list of Reference objects that identify
            one or more parent SpanContexts. (See the Reference documentation
            for detail).
        :param tags: an optional dictionary of Span Tags. The caller gives up
            ownership of that dictionary, because the Tracer may use it as-is
            to avoid extra data copying.
        :param start_time: an explicit Span start time as a unix timestamp per
            time.time().
        :param ignore_active_span: (optional) an explicit flag that ignores
            the current active `Scope` and creates a root `Span`.
        :param finish_on_close: whether span should automatically be finished
            when `Scope.close()` is called.
        :return: a `Scope`, already registered via the `ScopeManager`.
        """
        otspan = self.start_span(
            operation_name=operation_name,
            child_of=child_of,
            references=references,
            tags=tags,
            start_time=start_time,
            ignore_active_span=ignore_active_span,
        )

        # activate this new span
        scope = self._scope_manager.activate(otspan, finish_on_close)

        return scope

    def start_span(self, operation_name=None, child_of=None, references=None,
                   tags=None, start_time=None, ignore_active_span=False):
        """Starts and returns a new Span representing a unit of work.

        Starting a root Span (a Span with no causal references)::

            tracer.start_span('...')

        Starting a child Span (see also start_child_span())::

            tracer.start_span(
                '...',
                child_of=parent_span)

        Starting a child Span in a more verbose way::

            tracer.start_span(
                '...',
                references=[opentracing.child_of(parent_span)])

        Note: the precedence when defining a relationship is the following, from highest to lowest:
        1. *child_of*
        2. *references*
        3. `scope_manager.active` (unless *ignore_active_span* is True)
        4. None

        Currently Datadog only supports `child_of` references.

        :param operation_name: name of the operation represented by the new
            span from the perspective of the current service.
        :param child_of: (optional) a Span or SpanContext instance representing
            the parent in a REFERENCE_CHILD_OF Reference. If specified, the
            `references` parameter must be omitted.
        :param references: (optional) a list of Reference objects that identify
            one or more parent SpanContexts. (See the Reference documentation
            for detail)
        :param tags: an optional dictionary of Span Tags. The caller gives up
            ownership of that dictionary, because the Tracer may use it as-is
            to avoid extra data copying.
        :param start_time: an explicit Span start time as a unix timestamp per
            time.time()
        :param ignore_active_span: an explicit flag that ignores the current
            active `Scope` and creates a root `Span`.
        :return: an already-started Span instance.
        """
        ot_parent = None          # 'ot_parent' is more readable than 'child_of'
        ot_parent_context = None  # the parent span's context
        dd_parent = None          # the child_of to pass to the ddtracer

        if child_of is not None:
            ot_parent = child_of      # 'ot_parent' is more readable than 'child_of'
        elif references and isinstance(references, list):
            # we currently only support child_of relations to one span
            ot_parent = references[0].referenced_context

        # - whenever child_of is not None ddspans with parent-child
        #   relationships will share a ddcontext which maintains a hierarchy of
        #   ddspans for the execution flow
        # - when child_of is a ddspan then the ddtracer uses this ddspan to
        #   create the child ddspan
        # - when child_of is a ddcontext then the ddtracer uses the ddcontext to
        #   get_current_span() for the parent
        if ot_parent is None and not ignore_active_span:
            # attempt to get the parent span from the scope manager
            scope = self._scope_manager.active
            parent_span = getattr(scope, 'span', None)
            ot_parent_context = getattr(parent_span, 'context', None)
            # we want the ddcontext of the active span in order to maintain the
            # ddspan hierarchy
            dd_parent = getattr(ot_parent_context, '_dd_context', None)

            # if we cannot get the context then try getting it from the DD tracer
            # this emulates the behaviour of tracer.trace()
            if dd_parent is None:
                dd_parent = self._dd_tracer.get_call_context()
        elif ot_parent is not None and isinstance(ot_parent, Span):
            # a span is given to use as a parent
            ot_parent_context = ot_parent.context
            dd_parent = ot_parent._dd_span
        elif ot_parent is not None and isinstance(ot_parent, SpanContext):
            # a span context is given to use to find the parent ddspan
            dd_parent = ot_parent._dd_context
        elif ot_parent is None:
            # user wants to create a new parent span we don't have to do
            # anything
            pass
        else:
            raise TypeError('invalid span configuration given')

        # create a new otspan and ddspan using the ddtracer and associate it
        # with the new otspan
        ddspan = self._dd_tracer.start_span(
            name=operation_name,
            child_of=dd_parent,
            service=self._service_name,
        )

        # set the start time if one is specified
        ddspan.start = start_time or ddspan.start

        otspan = Span(self, ot_parent_context, operation_name)
        # sync up the OT span with the DD span
        otspan._associate_dd_span(ddspan)

        if tags is not None:
            for k in tags:
                # Make sure we set the tags on the otspan to ensure that the special compatibility tags
                # are handled correctly (resource name, span type, sampling priority, etc).
                otspan.set_tag(k, tags[k])

        return otspan

    def inject(self, span_context, format, carrier):  # noqa: A002
        """Injects a span context into a carrier.

        :param span_context: span context to inject.
        :param format: format to encode the span context with.
        :param carrier: the carrier of the encoded span context.
        """
        propagator = self._propagators.get(format, None)

        if propagator is None:
            raise opentracing.UnsupportedFormatException

        propagator.inject(span_context, carrier)

    def extract(self, format, carrier):  # noqa: A002
        """Extracts a span context from a carrier.

        :param format: format that the carrier is encoded with.
        :param carrier: the carrier to extract from.
        """
        propagator = self._propagators.get(format, None)

        if propagator is None:
            raise opentracing.UnsupportedFormatException

        # we have to manually activate the returned context from a distributed
        # trace
        ot_span_ctx = propagator.extract(carrier)
        dd_span_ctx = ot_span_ctx._dd_context
        self._dd_tracer.context_provider.activate(dd_span_ctx)
        return ot_span_ctx
