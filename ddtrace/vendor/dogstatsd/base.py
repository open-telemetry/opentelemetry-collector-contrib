#!/usr/bin/env python
"""
DogStatsd is a Python client for DogStatsd, a Statsd fork for Datadog.
"""
# stdlib
from random import random
import logging
import os
import socket
from threading import Lock

# datadog
from .context import TimedContextManagerDecorator
from .route import get_default_route
from .compat import text

# Logging
log = logging.getLogger('datadog.dogstatsd')

# Default config
DEFAULT_HOST = 'localhost'
DEFAULT_PORT = 8125

# Tag name of entity_id
ENTITY_ID_TAG_NAME = "dd.internal.entity_id"


class DogStatsd(object):
    OK, WARNING, CRITICAL, UNKNOWN = (0, 1, 2, 3)

    def __init__(self, host=DEFAULT_HOST, port=DEFAULT_PORT, max_buffer_size=50, namespace=None,
                 constant_tags=None, use_ms=False, use_default_route=False,
                 socket_path=None):
        """
        Initialize a DogStatsd object.

        >>> statsd = DogStatsd()

        :envvar DD_AGENT_HOST: the host of the DogStatsd server.
        If set, it overrides default value.
        :type DD_AGENT_HOST: string

        :envvar DD_DOGSTATSD_PORT: the port of the DogStatsd server.
        If set, it overrides default value.
        :type DD_DOGSTATSD_PORT: integer

        :param host: the host of the DogStatsd server.
        :type host: string

        :param port: the port of the DogStatsd server.
        :type port: integer

        :param max_buffer_size: Maximum number of metrics to buffer before sending to the server
        if sending metrics in batch
        :type max_buffer_size: integer

        :param namespace: Namespace to prefix all metric names
        :type namespace: string

        :param constant_tags: Tags to attach to all metrics
        :type constant_tags: list of strings

        :param use_ms: Report timed values in milliseconds instead of seconds (default False)
        :type use_ms: boolean

        :envvar DATADOG_TAGS: Tags to attach to every metric reported by dogstatsd client
        :type DATADOG_TAGS: list of strings

        :envvar DD_ENTITY_ID: Tag to identify the client entity.
        :type DD_ENTITY_ID: string

        :param use_default_route: Dynamically set the DogStatsd host to the default route
        (Useful when running the client in a container) (Linux only)
        :type use_default_route: boolean

        :param socket_path: Communicate with dogstatsd through a UNIX socket instead of
        UDP. If set, disables UDP transmission (Linux only)
        :type socket_path: string
        """

        self.lock = Lock()

        # Check host and port env vars
        agent_host = os.environ.get('DD_AGENT_HOST')
        if agent_host and host == DEFAULT_HOST:
            host = agent_host

        dogstatsd_port = os.environ.get('DD_DOGSTATSD_PORT')
        if dogstatsd_port and port == DEFAULT_PORT:
            try:
                port = int(dogstatsd_port)
            except ValueError:
                log.warning("Port number provided in DD_DOGSTATSD_PORT env var is not an integer: \
                %s, using %s as port number", dogstatsd_port, port)

        # Connection
        if socket_path is not None:
            self.socket_path = socket_path
            self.host = None
            self.port = None
        else:
            self.socket_path = None
            self.host = self.resolve_host(host, use_default_route)
            self.port = int(port)

        # Socket
        self.socket = None
        self.max_buffer_size = max_buffer_size
        self._send = self._send_to_server
        self.encoding = 'utf-8'

        # Options
        env_tags = [tag for tag in os.environ.get('DATADOG_TAGS', '').split(',') if tag]
        if constant_tags is None:
            constant_tags = []
        self.constant_tags = constant_tags + env_tags
        entity_id = os.environ.get('DD_ENTITY_ID')
        if entity_id:
            entity_tag = '{name}:{value}'.format(name=ENTITY_ID_TAG_NAME, value=entity_id)
            self.constant_tags.append(entity_tag)
        if namespace is not None:
            namespace = text(namespace)
        self.namespace = namespace
        self.use_ms = use_ms

    def __enter__(self):
        self.open_buffer(self.max_buffer_size)
        return self

    def __exit__(self, type, value, traceback):
        self.close_buffer()

    @staticmethod
    def resolve_host(host, use_default_route):
        """
        Resolve the DogStatsd host.

        Args:
            host (string): host
            use_default_route (bool): use the system default route as host
                (overrides the `host` parameter)
        """
        if not use_default_route:
            return host

        return get_default_route()

    def get_socket(self):
        """
        Return a connected socket.

        Note: connect the socket before assigning it to the class instance to
        avoid bad thread race conditions.
        """
        with self.lock:
            if not self.socket:
                if self.socket_path is not None:
                    sock = socket.socket(socket.AF_UNIX, socket.SOCK_DGRAM)
                    sock.connect(self.socket_path)
                    sock.setblocking(0)
                    self.socket = sock
                else:
                    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
                    sock.connect((self.host, self.port))
                    self.socket = sock

        return self.socket

    def open_buffer(self, max_buffer_size=50):
        """
        Open a buffer to send a batch of metrics in one packet.

        You can also use this as a context manager.

        >>> with DogStatsd() as batch:
        >>>     batch.gauge('users.online', 123)
        >>>     batch.gauge('active.connections', 1001)
        """
        self.max_buffer_size = max_buffer_size
        self.buffer = []
        self._send = self._send_to_buffer

    def close_buffer(self):
        """
        Flush the buffer and switch back to single metric packets.
        """
        self._send = self._send_to_server

        if self.buffer:
            # Only send packets if there are packets to send
            self._flush_buffer()

    def gauge(self, metric, value, tags=None, sample_rate=1):
        """
        Record the value of a gauge, optionally setting a list of tags and a
        sample rate.

        >>> statsd.gauge('users.online', 123)
        >>> statsd.gauge('active.connections', 1001, tags=["protocol:http"])
        """
        return self._report(metric, 'g', value, tags, sample_rate)

    def increment(self, metric, value=1, tags=None, sample_rate=1):
        """
        Increment a counter, optionally setting a value, tags and a sample
        rate.

        >>> statsd.increment('page.views')
        >>> statsd.increment('files.transferred', 124)
        """
        self._report(metric, 'c', value, tags, sample_rate)

    def decrement(self, metric, value=1, tags=None, sample_rate=1):
        """
        Decrement a counter, optionally setting a value, tags and a sample
        rate.

        >>> statsd.decrement('files.remaining')
        >>> statsd.decrement('active.connections', 2)
        """
        metric_value = -value if value else value
        self._report(metric, 'c', metric_value, tags, sample_rate)

    def histogram(self, metric, value, tags=None, sample_rate=1):
        """
        Sample a histogram value, optionally setting tags and a sample rate.

        >>> statsd.histogram('uploaded.file.size', 1445)
        >>> statsd.histogram('album.photo.count', 26, tags=["gender:female"])
        """
        self._report(metric, 'h', value, tags, sample_rate)

    def distribution(self, metric, value, tags=None, sample_rate=1):
        """
        Send a global distribution value, optionally setting tags and a sample rate.

        >>> statsd.distribution('uploaded.file.size', 1445)
        >>> statsd.distribution('album.photo.count', 26, tags=["gender:female"])

        This is a beta feature that must be enabled specifically for your organization.
        """
        self._report(metric, 'd', value, tags, sample_rate)

    def timing(self, metric, value, tags=None, sample_rate=1):
        """
        Record a timing, optionally setting tags and a sample rate.

        >>> statsd.timing("query.response.time", 1234)
        """
        self._report(metric, 'ms', value, tags, sample_rate)

    def timed(self, metric=None, tags=None, sample_rate=1, use_ms=None):
        """
        A decorator or context manager that will measure the distribution of a
        function's/context's run time. Optionally specify a list of tags or a
        sample rate. If the metric is not defined as a decorator, the module
        name and function name will be used. The metric is required as a context
        manager.
        ::

            @statsd.timed('user.query.time', sample_rate=0.5)
            def get_user(user_id):
                # Do what you need to ...
                pass

            # Is equivalent to ...
            with statsd.timed('user.query.time', sample_rate=0.5):
                # Do what you need to ...
                pass

            # Is equivalent to ...
            start = time.time()
            try:
                get_user(user_id)
            finally:
                statsd.timing('user.query.time', time.time() - start)
        """
        return TimedContextManagerDecorator(self, metric, tags, sample_rate, use_ms)

    def set(self, metric, value, tags=None, sample_rate=1):
        """
        Sample a set value.

        >>> statsd.set('visitors.uniques', 999)
        """
        self._report(metric, 's', value, tags, sample_rate)

    def close_socket(self):
        """
        Closes connected socket if connected.
        """
        if self.socket:
            self.socket.close()
            self.socket = None

    def _report(self, metric, metric_type, value, tags, sample_rate):
        """
        Create a metric packet and send it.

        More information about the packets' format: http://docs.datadoghq.com/guides/dogstatsd/
        """
        if value is None:
            return

        if sample_rate != 1 and random() > sample_rate:
            return

        # Resolve the full tag list
        tags = self._add_constant_tags(tags)

        # Create/format the metric packet
        payload = "%s%s:%s|%s%s%s" % (
            (self.namespace + ".") if self.namespace else "",
            metric,
            value,
            metric_type,
            ("|@" + text(sample_rate)) if sample_rate != 1 else "",
            ("|#" + ",".join(tags)) if tags else "",
        )

        # Send it
        self._send(payload)

    def _send_to_server(self, packet):
        try:
            # If set, use socket directly
            (self.socket or self.get_socket()).send(packet.encode(self.encoding))
        except socket.timeout:
            # dogstatsd is overflowing, drop the packets (mimicks the UDP behaviour)
            return
        except (socket.error, socket.herror, socket.gaierror) as se:
            log.warning("Error submitting packet: {}, dropping the packet and closing the socket".format(se))
            self.close_socket()
        except Exception as e:
            log.error("Unexpected error: %s", str(e))
            return

    def _send_to_buffer(self, packet):
        self.buffer.append(packet)
        if len(self.buffer) >= self.max_buffer_size:
            self._flush_buffer()

    def _flush_buffer(self):
        self._send_to_server("\n".join(self.buffer))
        self.buffer = []

    def _escape_event_content(self, string):
        return string.replace('\n', '\\n')

    def _escape_service_check_message(self, string):
        return string.replace('\n', '\\n').replace('m:', 'm\\:')

    def event(self, title, text, alert_type=None, aggregation_key=None,
              source_type_name=None, date_happened=None, priority=None,
              tags=None, hostname=None):
        """
        Send an event. Attributes are the same as the Event API.
            http://docs.datadoghq.com/api/

        >>> statsd.event('Man down!', 'This server needs assistance.')
        >>> statsd.event('The web server restarted', 'The web server is up again', alert_type='success')  # NOQA
        """
        title = self._escape_event_content(title)
        text = self._escape_event_content(text)

        # Append all client level tags to every event
        tags = self._add_constant_tags(tags)

        string = u'_e{%d,%d}:%s|%s' % (len(title), len(text), title, text)
        if date_happened:
            string = '%s|d:%d' % (string, date_happened)
        if hostname:
            string = '%s|h:%s' % (string, hostname)
        if aggregation_key:
            string = '%s|k:%s' % (string, aggregation_key)
        if priority:
            string = '%s|p:%s' % (string, priority)
        if source_type_name:
            string = '%s|s:%s' % (string, source_type_name)
        if alert_type:
            string = '%s|t:%s' % (string, alert_type)
        if tags:
            string = '%s|#%s' % (string, ','.join(tags))

        if len(string) > 8 * 1024:
            raise Exception(u'Event "%s" payload is too big (more than 8KB), '
                            'event discarded' % title)

        self._send(string)

    def service_check(self, check_name, status, tags=None, timestamp=None,
                      hostname=None, message=None):
        """
        Send a service check run.

        >>> statsd.service_check('my_service.check_name', DogStatsd.WARNING)
        """
        message = self._escape_service_check_message(message) if message is not None else ''

        string = u'_sc|{0}|{1}'.format(check_name, status)

        # Append all client level tags to every status check
        tags = self._add_constant_tags(tags)

        if timestamp:
            string = u'{0}|d:{1}'.format(string, timestamp)
        if hostname:
            string = u'{0}|h:{1}'.format(string, hostname)
        if tags:
            string = u'{0}|#{1}'.format(string, ','.join(tags))
        if message:
            string = u'{0}|m:{1}'.format(string, message)

        self._send(string)

    def _add_constant_tags(self, tags):
        if self.constant_tags:
            if tags:
                return tags + self.constant_tags
            else:
                return self.constant_tags
        return tags


statsd = DogStatsd()
