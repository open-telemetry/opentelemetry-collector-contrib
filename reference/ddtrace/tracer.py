import functools
import logging
from os import environ, getpid

from ddtrace.vendor import debtcollector

from .constants import FILTERS_KEY, SAMPLE_RATE_METRIC_KEY
from .ext import system
from .ext.priority import AUTO_REJECT, AUTO_KEEP
from .internal.logger import get_logger
from .internal.runtime import RuntimeTags, RuntimeWorker
from .internal.writer import AgentWriter
from .provider import DefaultContextProvider
from .context import Context
from .sampler import DatadogSampler, RateSampler, RateByServiceSampler
from .span import Span
from .utils.formats import get_env
from .utils.deprecation import deprecated, RemovedInDDTrace10Warning
from .vendor.dogstatsd import DogStatsd
from . import compat


log = get_logger(__name__)


def _parse_dogstatsd_url(url):
    if url is None:
        return

    # url can be either of the form `udp://<host>:<port>` or `unix://<path>`
    # also support without url scheme included
    if url.startswith('/'):
        url = 'unix://' + url
    elif '://' not in url:
        url = 'udp://' + url

    parsed = compat.parse.urlparse(url)

    if parsed.scheme == 'unix':
        return dict(socket_path=parsed.path)
    elif parsed.scheme == 'udp':
        return dict(host=parsed.hostname, port=parsed.port)
    else:
        raise ValueError('Unknown scheme `%s` for DogStatsD URL `{}`'.format(parsed.scheme))


_INTERNAL_APPLICATION_SPAN_TYPES = [
    "custom",
    "template",
    "web",
    "worker"
]


class Tracer(object):
    """
    Tracer is used to create, sample and submit spans that measure the
    execution time of sections of code.

    If you're running an application that will serve a single trace per thread,
    you can use the global tracer instance::

        from ddtrace import tracer
        trace = tracer.trace('app.request', 'web-server').finish()
    """
    _RUNTIME_METRICS_INTERVAL = 10

    DEFAULT_HOSTNAME = environ.get('DD_AGENT_HOST', environ.get('DATADOG_TRACE_AGENT_HOSTNAME', 'localhost'))
    DEFAULT_PORT = int(environ.get('DD_TRACE_AGENT_PORT', 8126))
    DEFAULT_DOGSTATSD_PORT = int(get_env('dogstatsd', 'port', 8125))
    DEFAULT_DOGSTATSD_URL = get_env('dogstatsd', 'url', 'udp://{}:{}'.format(DEFAULT_HOSTNAME, DEFAULT_DOGSTATSD_PORT))
    DEFAULT_AGENT_URL = environ.get('DD_TRACE_AGENT_URL', 'http://%s:%d' % (DEFAULT_HOSTNAME, DEFAULT_PORT))

    def __init__(self, url=DEFAULT_AGENT_URL, dogstatsd_url=DEFAULT_DOGSTATSD_URL):
        """
        Create a new ``Tracer`` instance. A global tracer is already initialized
        for common usage, so there is no need to initialize your own ``Tracer``.

        :param url: The Datadog agent URL.
        :param url: The DogStatsD URL.
        """
        self.log = log
        self.sampler = None
        self.priority_sampler = None
        self._runtime_worker = None

        uds_path = None
        https = None
        hostname = self.DEFAULT_HOSTNAME
        port = self.DEFAULT_PORT
        if url is not None:
            url_parsed = compat.parse.urlparse(url)
            if url_parsed.scheme in ('http', 'https'):
                hostname = url_parsed.hostname
                port = url_parsed.port
                https = url_parsed.scheme == 'https'
                # FIXME This is needed because of the way of configure() works right now, where it considers `port=None`
                # to be "no port set so let's use the default".
                # It should go away when we remove configure()
                if port is None:
                    if https:
                        port = 443
                    else:
                        port = 80
            elif url_parsed.scheme == 'unix':
                uds_path = url_parsed.path
            else:
                raise ValueError('Unknown scheme `%s` for agent URL' % url_parsed.scheme)

        # Apply the default configuration
        self.configure(
            enabled=True,
            hostname=hostname,
            port=port,
            https=https,
            uds_path=uds_path,
            sampler=DatadogSampler(),
            context_provider=DefaultContextProvider(),
            dogstatsd_url=dogstatsd_url,
        )

        # globally set tags
        self.tags = {}

        # a buffer for service info so we don't perpetually send the same things
        self._services = set()

        # Runtime id used for associating data collected during runtime to
        # traces
        self._pid = getpid()

    @property
    def debug_logging(self):
        return self.log.isEnabledFor(logging.DEBUG)

    @debug_logging.setter
    @deprecated(message='Use logging.setLevel instead', version='1.0.0')
    def debug_logging(self, value):
        self.log.setLevel(logging.DEBUG if value else logging.WARN)

    @deprecated('Use .tracer, not .tracer()', '1.0.0')
    def __call__(self):
        return self

    def global_excepthook(self, tp, value, traceback):
        """The global tracer except hook."""
        self._dogstatsd_client.increment('datadog.tracer.uncaught_exceptions', 1,
                                         tags=['class:%s' % tp.__name__])

    def get_call_context(self, *args, **kwargs):
        """
        Return the current active ``Context`` for this traced execution. This method is
        automatically called in the ``tracer.trace()``, but it can be used in the application
        code during manual instrumentation like::

            from ddtrace import tracer

            async def web_handler(request):
                context = tracer.get_call_context()
                # use the context if needed
                # ...

        This method makes use of a ``ContextProvider`` that is automatically set during the tracer
        initialization, or while using a library instrumentation.
        """
        return self._context_provider.active(*args, **kwargs)

    @property
    def context_provider(self):
        """Returns the current Tracer Context Provider"""
        return self._context_provider

    # TODO: deprecate this method and make sure users create a new tracer if they need different parameters
    @debtcollector.removals.removed_kwarg("dogstatsd_host", "Use `dogstatsd_url` instead",
                                          category=RemovedInDDTrace10Warning)
    @debtcollector.removals.removed_kwarg("dogstatsd_port", "Use `dogstatsd_url` instead",
                                          category=RemovedInDDTrace10Warning)
    def configure(self, enabled=None, hostname=None, port=None, uds_path=None, https=None,
                  sampler=None, context_provider=None, wrap_executor=None, priority_sampling=None,
                  settings=None, collect_metrics=None, dogstatsd_host=None, dogstatsd_port=None,
                  dogstatsd_url=None):
        """
        Configure an existing Tracer the easy way.
        Allow to configure or reconfigure a Tracer instance.

        :param bool enabled: If True, finished traces will be submitted to the API.
            Otherwise they'll be dropped.
        :param str hostname: Hostname running the Trace Agent
        :param int port: Port of the Trace Agent
        :param str uds_path: The Unix Domain Socket path of the agent.
        :param bool https: Whether to use HTTPS or HTTP.
        :param object sampler: A custom Sampler instance, locally deciding to totally drop the trace or not.
        :param object context_provider: The ``ContextProvider`` that will be used to retrieve
            automatically the current call context. This is an advanced option that usually
            doesn't need to be changed from the default value
        :param object wrap_executor: callable that is used when a function is decorated with
            ``Tracer.wrap()``. This is an advanced option that usually doesn't need to be changed
            from the default value
        :param priority_sampling: enable priority sampling, this is required for
            complete distributed tracing support. Enabled by default.
        :param collect_metrics: Whether to enable runtime metrics collection.
        :param str dogstatsd_host: Host for UDP connection to DogStatsD (deprecated: use dogstatsd_url)
        :param int dogstatsd_port: Port for UDP connection to DogStatsD (deprecated: use dogstatsd_url)
        :param str dogstatsd_url: URL for UDP or Unix socket connection to DogStatsD
        """
        if enabled is not None:
            self.enabled = enabled

        filters = None
        if settings is not None:
            filters = settings.get(FILTERS_KEY)

        # If priority sampling is not set or is True and no priority sampler is set yet
        if priority_sampling in (None, True) and not self.priority_sampler:
            self.priority_sampler = RateByServiceSampler()
        # Explicitly disable priority sampling
        elif priority_sampling is False:
            self.priority_sampler = None

        if sampler is not None:
            self.sampler = sampler

        if dogstatsd_host is not None and dogstatsd_url is None:
            dogstatsd_url = 'udp://{}:{}'.format(dogstatsd_host, dogstatsd_port or self.DEFAULT_DOGSTATSD_PORT)

        if dogstatsd_url is not None:
            dogstatsd_kwargs = _parse_dogstatsd_url(dogstatsd_url)
            self.log.debug('Connecting to DogStatsd(%s)', dogstatsd_url)
            self._dogstatsd_client = DogStatsd(**dogstatsd_kwargs)

        if hostname is not None or port is not None or uds_path is not None or https is not None or \
                filters is not None or priority_sampling is not None or sampler is not None:
            # Preserve hostname and port when overriding filters or priority sampling
            # This is clumsy and a good reason to get rid of this configure() API
            if hasattr(self, 'writer') and hasattr(self.writer, 'api'):
                default_hostname = self.writer.api.hostname
                default_port = self.writer.api.port
                if https is None:
                    https = self.writer.api.https
            else:
                default_hostname = self.DEFAULT_HOSTNAME
                default_port = self.DEFAULT_PORT

            self.writer = AgentWriter(
                hostname or default_hostname,
                port or default_port,
                uds_path=uds_path,
                https=https,
                filters=filters,
                sampler=self.sampler,
                priority_sampler=self.priority_sampler,
                dogstatsd=self._dogstatsd_client,
            )

        # HACK: since we recreated our dogstatsd agent, replace the old write one
        self.writer.dogstatsd = self._dogstatsd_client

        if context_provider is not None:
            self._context_provider = context_provider

        if wrap_executor is not None:
            self._wrap_executor = wrap_executor

        # Since we've recreated our dogstatsd agent, we need to restart metric collection with that new agent
        if self._runtime_worker:
            runtime_metrics_was_running = True
            self._runtime_worker.stop()
            self._runtime_worker.join()
            self._runtime_worker = None
        else:
            runtime_metrics_was_running = False

        if (collect_metrics is None and runtime_metrics_was_running) or collect_metrics:
            self._start_runtime_worker()

    def start_span(self, name, child_of=None, service=None, resource=None, span_type=None):
        """
        Return a span that will trace an operation called `name`. This method allows
        parenting using the ``child_of`` kwarg. If it's missing, the newly created span is a
        root span.

        :param str name: the name of the operation being traced.
        :param object child_of: a ``Span`` or a ``Context`` instance representing the parent for this span.
        :param str service: the name of the service being traced.
        :param str resource: an optional name of the resource being tracked.
        :param str span_type: an optional operation type.

        To start a new root span, simply::

            span = tracer.start_span('web.request')

        If you want to create a child for a root span, just::

            root_span = tracer.start_span('web.request')
            span = tracer.start_span('web.decoder', child_of=root_span)

        Or if you have a ``Context`` object::

            context = tracer.get_call_context()
            span = tracer.start_span('web.worker', child_of=context)
        """
        if child_of is not None:
            # retrieve if the span is a child_of a Span or a of Context
            child_of_context = isinstance(child_of, Context)
            context = child_of if child_of_context else child_of.context
            parent = child_of.get_current_span() if child_of_context else child_of
        else:
            context = Context()
            parent = None

        if parent:
            trace_id = parent.trace_id
            parent_span_id = parent.span_id
        else:
            trace_id = context.trace_id
            parent_span_id = context.span_id

        if trace_id:
            # child_of a non-empty context, so either a local child span or from a remote context

            # when not provided, inherit from parent's service
            if parent:
                service = service or parent.service

            span = Span(
                self,
                name,
                trace_id=trace_id,
                parent_id=parent_span_id,
                service=service,
                resource=resource,
                span_type=span_type,
            )

            # Extra attributes when from a local parent
            if parent:
                span.sampled = parent.sampled
                span._parent = parent

        else:
            # this is the root span of a new trace
            span = Span(
                self,
                name,
                service=service,
                resource=resource,
                span_type=span_type,
            )

            span.sampled = self.sampler.sample(span)
            # Old behavior
            # DEV: The new sampler sets metrics and priority sampling on the span for us
            if not isinstance(self.sampler, DatadogSampler):
                if span.sampled:
                    # When doing client sampling in the client, keep the sample rate so that we can
                    # scale up statistics in the next steps of the pipeline.
                    if isinstance(self.sampler, RateSampler):
                        span.set_metric(SAMPLE_RATE_METRIC_KEY, self.sampler.sample_rate)

                    if self.priority_sampler:
                        # At this stage, it's important to have the service set. If unset,
                        # priority sampler will use the default sampling rate, which might
                        # lead to oversampling (that is, dropping too many traces).
                        if self.priority_sampler.sample(span):
                            context.sampling_priority = AUTO_KEEP
                        else:
                            context.sampling_priority = AUTO_REJECT
                else:
                    if self.priority_sampler:
                        # If dropped by the local sampler, distributed instrumentation can drop it too.
                        context.sampling_priority = AUTO_REJECT
            else:
                context.sampling_priority = AUTO_KEEP if span.sampled else AUTO_REJECT
                # We must always mark the span as sampled so it is forwarded to the agent
                span.sampled = True

            # add tags to root span to correlate trace with runtime metrics
            # only applied to spans with types that are internal to applications
            if self._runtime_worker and span.span_type in _INTERNAL_APPLICATION_SPAN_TYPES:
                span.set_tag('language', 'python')

        # add common tags
        if self.tags:
            span.set_tags(self.tags)
        if not span._parent:
            span.set_tag(system.PID, getpid())

        # add it to the current context
        context.add_span(span)

        # check for new process if runtime metrics worker has already been started
        self._check_new_process()

        # update set of services handled by tracer
        if service and service not in self._services:
            self._services.add(service)

            # The constant tags for the dogstatsd client needs to updated with any new
            # service(s) that may have been added.
            self._update_dogstatsd_constant_tags()

        return span

    def _update_dogstatsd_constant_tags(self):
        """ Prepare runtime tags for ddstatsd.
        """
        # DEV: ddstatsd expects tags in the form ['key1:value1', 'key2:value2', ...]
        tags = [
            '{}:{}'.format(k, v)
            for k, v in RuntimeTags()
        ]
        self.log.debug('Updating constant tags %s', tags)
        self._dogstatsd_client.constant_tags = tags

    def _start_runtime_worker(self):
        self._runtime_worker = RuntimeWorker(self._dogstatsd_client, self._RUNTIME_METRICS_INTERVAL)
        self._runtime_worker.start()

    def _check_new_process(self):
        """ Checks if the tracer is in a new process (was forked) and performs
            the necessary updates if it is a new process
        """
        pid = getpid()
        if self._pid == pid:
            return

        self._pid = pid

        # Assume that the services of the child are not necessarily a subset of those
        # of the parent.
        self._services = set()

        if self._runtime_worker is not None:
            self._start_runtime_worker()

        # force an immediate update constant tags since we have reset services
        # and generated a new runtime id
        self._update_dogstatsd_constant_tags()

        # Re-create the background writer thread
        self.writer = self.writer.recreate()

    def trace(self, name, service=None, resource=None, span_type=None):
        """
        Return a span that will trace an operation called `name`. The context that created
        the span as well as the span parenting, are automatically handled by the tracing
        function.

        :param str name: the name of the operation being traced
        :param str service: the name of the service being traced. If not set,
                            it will inherit the service from its parent.
        :param str resource: an optional name of the resource being tracked.
        :param str span_type: an optional operation type.

        You must call `finish` on all spans, either directly or with a context
        manager::

            >>> span = tracer.trace('web.request')
                try:
                    # do something
                finally:
                    span.finish()

            >>> with tracer.trace('web.request') as span:
                    # do something

        Trace will store the current active span and subsequent child traces will
        become its children::

            parent = tracer.trace('parent')     # has no parent span
            child  = tracer.trace('child')      # is a child of a parent
            child.finish()
            parent.finish()

            parent2 = tracer.trace('parent2')   # has no parent span
            parent2.finish()
        """
        # retrieve the Context using the context provider and create
        # a new Span that could be a root or a nested span
        context = self.get_call_context()
        return self.start_span(
            name,
            child_of=context,
            service=service,
            resource=resource,
            span_type=span_type,
        )

    def current_root_span(self):
        """Returns the root span of the current context.

        This is useful for attaching information related to the trace as a
        whole without needing to add to child spans.

        Usage is simple, for example::

            # get the root span
            root_span = tracer.current_root_span()
            # set the host just once on the root span
            if root_span:
                root_span.set_tag('host', '127.0.0.1')
        """
        ctx = self.get_call_context()
        if ctx:
            return ctx.get_current_root_span()
        return None

    def current_span(self):
        """
        Return the active span for the current call context or ``None``
        if no spans are available.
        """
        ctx = self.get_call_context()
        if ctx:
            return ctx.get_current_span()
        return None

    def record(self, context):
        """
        Record the given ``Context`` if it's finished.
        """
        # extract and enqueue the trace if it's sampled
        trace, sampled = context.get()
        if trace and sampled:
            self.write(trace)

    def write(self, spans):
        """
        Send the trace to the writer to enqueue the spans list in the agent
        sending queue.
        """
        if not spans:
            return  # nothing to do

        if self.log.isEnabledFor(logging.DEBUG):
            self.log.debug('writing %s spans (enabled:%s)', len(spans), self.enabled)
            for span in spans:
                self.log.debug('\n%s', span.pprint())

        if self.enabled and self.writer:
            # only submit the spans if we're actually enabled (and don't crash :)
            self.writer.write(spans=spans)

    @deprecated(message='Manually setting service info is no longer necessary', version='1.0.0')
    def set_service_info(self, *args, **kwargs):
        """Set the information about the given service.
        """
        return

    def wrap(self, name=None, service=None, resource=None, span_type=None):
        """
        A decorator used to trace an entire function. If the traced function
        is a coroutine, it traces the coroutine execution when is awaited.
        If a ``wrap_executor`` callable has been provided in the ``Tracer.configure()``
        method, it will be called instead of the default one when the function
        decorator is invoked.

        :param str name: the name of the operation being traced. If not set,
                         defaults to the fully qualified function name.
        :param str service: the name of the service being traced. If not set,
                            it will inherit the service from it's parent.
        :param str resource: an optional name of the resource being tracked.
        :param str span_type: an optional operation type.

        >>> @tracer.wrap('my.wrapped.function', service='my.service')
            def run():
                return 'run'

        >>> # name will default to 'execute' if unset
            @tracer.wrap()
            def execute():
                return 'executed'

        >>> # or use it in asyncio coroutines
            @tracer.wrap()
            async def coroutine():
                return 'executed'

        >>> @tracer.wrap()
            @asyncio.coroutine
            def coroutine():
                return 'executed'

        You can access the current span using `tracer.current_span()` to set
        tags:

        >>> @tracer.wrap()
            def execute():
                span = tracer.current_span()
                span.set_tag('a', 'b')
        """
        def wrap_decorator(f):
            # FIXME[matt] include the class name for methods.
            span_name = name if name else '%s.%s' % (f.__module__, f.__name__)

            # detect if the the given function is a coroutine to use the
            # right decorator; this initial check ensures that the
            # evaluation is done only once for each @tracer.wrap
            if compat.iscoroutinefunction(f):
                # call the async factory that creates a tracing decorator capable
                # to await the coroutine execution before finishing the span. This
                # code is used for compatibility reasons to prevent Syntax errors
                # in Python 2
                func_wrapper = compat.make_async_decorator(
                    self, f, span_name,
                    service=service,
                    resource=resource,
                    span_type=span_type,
                )
            else:
                @functools.wraps(f)
                def func_wrapper(*args, **kwargs):
                    # if a wrap executor has been configured, it is used instead
                    # of the default tracing function
                    if getattr(self, '_wrap_executor', None):
                        return self._wrap_executor(
                            self,
                            f, args, kwargs,
                            span_name,
                            service=service,
                            resource=resource,
                            span_type=span_type,
                        )

                    # otherwise fallback to a default tracing
                    with self.trace(span_name, service=service, resource=resource, span_type=span_type):
                        return f(*args, **kwargs)

            return func_wrapper
        return wrap_decorator

    def set_tags(self, tags):
        """ Set some tags at the tracer level.
        This will append those tags to each span created by the tracer.

        :param dict tags: dict of tags to set at tracer level
        """
        self.tags.update(tags)
