import math
import random
import sys
import traceback

from .compat import StringIO, stringify, iteritems, numeric_types, time_ns, is_integer
from .constants import NUMERIC_TAGS, MANUAL_DROP_KEY, MANUAL_KEEP_KEY
from .ext import SpanTypes, errors, priority, net, http
from .internal.logger import get_logger


log = get_logger(__name__)


if sys.version_info.major < 3:
    _getrandbits = random.SystemRandom().getrandbits
else:
    _getrandbits = random.getrandbits


class Span(object):

    __slots__ = [
        # Public span attributes
        'service',
        'name',
        'resource',
        'span_id',
        'trace_id',
        'parent_id',
        'meta',
        'error',
        'metrics',
        'span_type',
        'start_ns',
        'duration_ns',
        'tracer',
        # Sampler attributes
        'sampled',
        # Internal attributes
        '_context',
        'finished',
        '_parent',
        '__weakref__',
    ]

    def __init__(
        self,
        tracer,
        name,

        service=None,
        resource=None,
        span_type=None,
        trace_id=None,
        span_id=None,
        parent_id=None,
        start=None,
        context=None,
    ):
        """
        Create a new span. Call `finish` once the traced operation is over.

        :param ddtrace.Tracer tracer: the tracer that will submit this span when
            finished.
        :param str name: the name of the traced operation.

        :param str service: the service name
        :param str resource: the resource name
        :param str span_type: the span type

        :param int trace_id: the id of this trace's root span.
        :param int parent_id: the id of this span's direct parent span.
        :param int span_id: the id of this span.

        :param int start: the start time of request as a unix epoch in seconds
        :param object context: the Context of the span.
        """
        # required span info
        self.name = name
        self.service = service
        self.resource = resource or name
        self.span_type = span_type.value if isinstance(span_type, SpanTypes) else span_type

        # tags / metatdata
        self.meta = {}
        self.error = 0
        self.metrics = {}

        # timing
        self.start_ns = time_ns() if start is None else int(start * 1e9)
        self.duration_ns = None

        # tracing
        self.trace_id = trace_id or _new_id()
        self.span_id = span_id or _new_id()
        self.parent_id = parent_id
        self.tracer = tracer

        # sampling
        self.sampled = True

        self._context = context
        self._parent = None

        # state
        self.finished = False

    @property
    def start(self):
        """The start timestamp in Unix epoch seconds."""
        return self.start_ns / 1e9

    @start.setter
    def start(self, value):
        self.start_ns = int(value * 1e9)

    @property
    def duration(self):
        """The span duration in seconds."""
        if self.duration_ns is not None:
            return self.duration_ns / 1e9

    @duration.setter
    def duration(self, value):
        self.duration_ns = value * 1e9

    def finish(self, finish_time=None):
        """Mark the end time of the span and submit it to the tracer.
        If the span has already been finished don't do anything

        :param int finish_time: The end time of the span in seconds.
                                Defaults to now.
        """
        if self.finished:
            return
        self.finished = True

        if self.duration_ns is None:
            ft = time_ns() if finish_time is None else int(finish_time * 1e9)
            # be defensive so we don't die if start isn't set
            self.duration_ns = ft - (self.start_ns or ft)

        if self._context:
            try:
                self._context.close_span(self)
            except Exception:
                log.exception('error recording finished trace')
            else:
                # if a tracer is available to process the current context
                if self.tracer:
                    try:
                        self.tracer.record(self._context)
                    except Exception:
                        log.exception('error recording finished trace')

    def set_tag(self, key, value=None):
        """ Set the given key / value tag pair on the span. Keys and values
            must be strings (or stringable). If a casting error occurs, it will
            be ignored.
        """
        # Special case, force `http.status_code` as a string
        # DEV: `http.status_code` *has* to be in `meta` for metrics
        #   calculated in the trace agent
        if key == http.STATUS_CODE:
            value = str(value)

        # Determine once up front
        is_an_int = is_integer(value)

        # Explicitly try to convert expected integers to `int`
        # DEV: Some integrations parse these values from strings, but don't call `int(value)` themselves
        INT_TYPES = (net.TARGET_PORT, )
        if key in INT_TYPES and not is_an_int:
            try:
                value = int(value)
                is_an_int = True
            except (ValueError, TypeError):
                pass

        # Set integers that are less than equal to 2^53 as metrics
        if is_an_int and abs(value) <= 2 ** 53:
            self.set_metric(key, value)
            return

        # All floats should be set as a metric
        elif isinstance(value, float):
            self.set_metric(key, value)
            return

        # Key should explicitly be converted to a float if needed
        elif key in NUMERIC_TAGS:
            try:
                # DEV: `set_metric` will try to cast to `float()` for us
                self.set_metric(key, value)
            except (TypeError, ValueError):
                log.debug('error setting numeric metric %s:%s', key, value)

            return

        elif key == MANUAL_KEEP_KEY:
            self.context.sampling_priority = priority.USER_KEEP
            return
        elif key == MANUAL_DROP_KEY:
            self.context.sampling_priority = priority.USER_REJECT
            return

        try:
            self.meta[key] = stringify(value)
            if key in self.metrics:
                del self.metrics[key]
        except Exception:
            log.debug('error setting tag %s, ignoring it', key, exc_info=True)

    def _remove_tag(self, key):
        if key in self.meta:
            del self.meta[key]

    def get_tag(self, key):
        """ Return the given tag or None if it doesn't exist.
        """
        return self.meta.get(key, None)

    def set_tags(self, tags):
        """ Set a dictionary of tags on the given span. Keys and values
            must be strings (or stringable)
        """
        if tags:
            for k, v in iter(tags.items()):
                self.set_tag(k, v)

    def set_meta(self, k, v):
        self.set_tag(k, v)

    def set_metas(self, kvs):
        self.set_tags(kvs)

    def set_metric(self, key, value):
        # This method sets a numeric tag value for the given key. It acts
        # like `set_meta()` and it simply add a tag without further processing.

        # FIXME[matt] we could push this check to serialization time as well.
        # only permit types that are commonly serializable (don't use
        # isinstance so that we convert unserializable types like numpy
        # numbers)
        if type(value) not in numeric_types:
            try:
                value = float(value)
            except (ValueError, TypeError):
                log.debug('ignoring not number metric %s:%s', key, value)
                return

        # don't allow nan or inf
        if math.isnan(value) or math.isinf(value):
            log.debug('ignoring not real metric %s:%s', key, value)
            return

        if key in self.meta:
            del self.meta[key]
        self.metrics[key] = value

    def set_metrics(self, metrics):
        if metrics:
            for k, v in iteritems(metrics):
                self.set_metric(k, v)

    def get_metric(self, key):
        return self.metrics.get(key)

    def to_dict(self):
        d = {
            'trace_id': self.trace_id,
            'parent_id': self.parent_id,
            'span_id': self.span_id,
            'service': self.service,
            'resource': self.resource,
            'name': self.name,
            'error': self.error,
        }

        # a common mistake is to set the error field to a boolean instead of an
        # int. let's special case that here, because it's sure to happen in
        # customer code.
        err = d.get('error')
        if err and type(err) == bool:
            d['error'] = 1

        if self.start_ns:
            d['start'] = self.start_ns

        if self.duration_ns:
            d['duration'] = self.duration_ns

        if self.meta:
            d['meta'] = self.meta

        if self.metrics:
            d['metrics'] = self.metrics

        if self.span_type:
            d['type'] = self.span_type

        return d

    def set_traceback(self, limit=20):
        """ If the current stack has an exception, tag the span with the
            relevant error info. If not, set the span to the current python stack.
        """
        (exc_type, exc_val, exc_tb) = sys.exc_info()

        if (exc_type and exc_val and exc_tb):
            self.set_exc_info(exc_type, exc_val, exc_tb)
        else:
            tb = ''.join(traceback.format_stack(limit=limit + 1)[:-1])
            self.set_tag(errors.ERROR_STACK, tb)  # FIXME[gabin] Want to replace 'error.stack' tag with 'python.stack'

    def set_exc_info(self, exc_type, exc_val, exc_tb):
        """ Tag the span with an error tuple as from `sys.exc_info()`. """
        if not (exc_type and exc_val and exc_tb):
            return  # nothing to do

        self.error = 1

        # get the traceback
        buff = StringIO()
        traceback.print_exception(exc_type, exc_val, exc_tb, file=buff, limit=20)
        tb = buff.getvalue()

        # readable version of type (e.g. exceptions.ZeroDivisionError)
        exc_type_str = '%s.%s' % (exc_type.__module__, exc_type.__name__)

        self.set_tag(errors.ERROR_MSG, exc_val)
        self.set_tag(errors.ERROR_TYPE, exc_type_str)
        self.set_tag(errors.ERROR_STACK, tb)

    def _remove_exc_info(self):
        """ Remove all exception related information from the span. """
        self.error = 0
        self._remove_tag(errors.ERROR_MSG)
        self._remove_tag(errors.ERROR_TYPE)
        self._remove_tag(errors.ERROR_STACK)

    def pprint(self):
        """ Return a human readable version of the span. """
        lines = [
            ('name', self.name),
            ('id', self.span_id),
            ('trace_id', self.trace_id),
            ('parent_id', self.parent_id),
            ('service', self.service),
            ('resource', self.resource),
            ('type', self.span_type),
            ('start', self.start),
            ('end', '' if not self.duration else self.start + self.duration),
            ('duration', '%fs' % (self.duration or 0)),
            ('error', self.error),
            ('tags', '')
        ]

        lines.extend((' ', '%s:%s' % kv) for kv in sorted(self.meta.items()))
        return '\n'.join('%10s %s' % l for l in lines)

    @property
    def context(self):
        """
        Property that provides access to the ``Context`` associated with this ``Span``.
        The ``Context`` contains state that propagates from span to span in a
        larger trace.
        """
        return self._context

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        try:
            if exc_type:
                self.set_exc_info(exc_type, exc_val, exc_tb)
            self.finish()
        except Exception:
            log.exception('error closing trace')

    def __repr__(self):
        return '<Span(id=%s,trace_id=%s,parent_id=%s,name=%s)>' % (
            self.span_id,
            self.trace_id,
            self.parent_id,
            self.name,
        )


def _new_id():
    """Generate a random trace_id or span_id"""
    return _getrandbits(64)
