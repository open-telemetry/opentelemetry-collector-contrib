import mock
import re
import socket
import threading
import time
import warnings

from unittest import TestCase

import pytest

from ddtrace.api import API, Response
from ddtrace.compat import iteritems, httplib, PY3
from ddtrace.internal.runtime.container import CGroupInfo
from ddtrace.vendor.six.moves import BaseHTTPServer, socketserver


class _BaseHTTPRequestHandler(BaseHTTPServer.BaseHTTPRequestHandler):
    error_message_format = '%(message)s\n'
    error_content_type = 'text/plain'

    @staticmethod
    def log_message(format, *args):  # noqa: A002
        pass


class _APIEndpointRequestHandlerTest(_BaseHTTPRequestHandler):

    def do_PUT(self):
        self.send_error(200, 'OK')


class _TimeoutAPIEndpointRequestHandlerTest(_BaseHTTPRequestHandler):
    def do_PUT(self):
        # This server sleeps longer than our timeout
        time.sleep(5)


class _ResetAPIEndpointRequestHandlerTest(_BaseHTTPRequestHandler):

    def do_PUT(self):
        return


_HOST = '0.0.0.0'
_TIMEOUT_PORT = 8743
_RESET_PORT = _TIMEOUT_PORT + 1


class UDSHTTPServer(socketserver.UnixStreamServer, BaseHTTPServer.HTTPServer):
    def server_bind(self):
        BaseHTTPServer.HTTPServer.server_bind(self)


def _make_uds_server(path, request_handler):
    server = UDSHTTPServer(path, request_handler)
    t = threading.Thread(target=server.serve_forever)
    # Set daemon just in case something fails
    t.daemon = True
    t.start()
    return server, t


@pytest.fixture
def endpoint_uds_server(tmp_path):
    server, thread = _make_uds_server(str(tmp_path / 'uds_server_socket'), _APIEndpointRequestHandlerTest)
    try:
        yield server
    finally:
        server.shutdown()
        thread.join()


def _make_server(port, request_handler):
    server = BaseHTTPServer.HTTPServer((_HOST, port), request_handler)
    t = threading.Thread(target=server.serve_forever)
    # Set daemon just in case something fails
    t.daemon = True
    t.start()
    return server, t


@pytest.fixture(scope='module')
def endpoint_test_timeout_server():
    server, thread = _make_server(_TIMEOUT_PORT, _TimeoutAPIEndpointRequestHandlerTest)
    try:
        yield thread
    finally:
        server.shutdown()
        thread.join()


@pytest.fixture(scope='module')
def endpoint_test_reset_server():
    server, thread = _make_server(_RESET_PORT, _ResetAPIEndpointRequestHandlerTest)
    try:
        yield thread
    finally:
        server.shutdown()
        thread.join()


class ResponseMock:
    def __init__(self, content, status=200):
        self.status = status
        self.content = content

    def read(self):
        return self.content


def test_api_str():
    api = API('localhost', 8126, https=True)
    assert str(api) == 'https://localhost:8126'
    api = API('localhost', 8126, '/path/to/uds')
    assert str(api) == 'unix:///path/to/uds'


class APITests(TestCase):

    def setUp(self):
        # DEV: Mock here instead of in tests, before we have patched `httplib.HTTPConnection`
        self.conn = mock.MagicMock(spec=httplib.HTTPConnection)
        self.api = API('localhost', 8126)

    def tearDown(self):
        del self.api
        del self.conn

    def test_typecast_port(self):
        api = API('localhost', u'8126')
        self.assertEqual(api.port, 8126)

    @mock.patch('logging.Logger.debug')
    def test_parse_response_json(self, log):
        test_cases = {
            'OK': dict(
                js=None,
                log='Cannot parse Datadog Agent response, please make sure your Datadog Agent is up to date',
            ),
            'OK\n': dict(
                js=None,
                log='Cannot parse Datadog Agent response, please make sure your Datadog Agent is up to date',
            ),
            'error:unsupported-endpoint': dict(
                js=None,
                log='Unable to parse Datadog Agent JSON response: \'error:unsupported-endpoint\'',
            ),
            42: dict(  # int as key to trigger TypeError
                js=None,
                log='Unable to parse Datadog Agent JSON response: 42',
            ),
            '{}': dict(js={}),
            '[]': dict(js=[]),

            # Priority sampling "rate_by_service" response
            ('{"rate_by_service": '
             '{"service:,env:":0.5, "service:mcnulty,env:test":0.9, "service:postgres,env:test":0.6}}'): dict(
                js=dict(
                    rate_by_service={
                        'service:,env:': 0.5,
                        'service:mcnulty,env:test': 0.9,
                        'service:postgres,env:test': 0.6,
                    },
                ),
            ),
            ' [4,2,1] ': dict(js=[4, 2, 1]),
        }

        for k, v in iteritems(test_cases):
            log.reset_mock()

            r = Response.from_http_response(ResponseMock(k))
            js = r.get_json()
            assert v['js'] == js
            if 'log' in v:
                log.assert_called_once()
                msg = log.call_args[0][0] % log.call_args[0][1:]
                assert re.match(v['log'], msg), msg

    @mock.patch('ddtrace.compat.httplib.HTTPConnection')
    def test_put_connection_close(self, HTTPConnection):
        """
        When calling API._put
            we close the HTTPConnection we create
        """
        HTTPConnection.return_value = self.conn

        with warnings.catch_warnings(record=True) as w:
            self.api._put('/test', '<test data>', 1)

            self.assertEqual(len(w), 0, 'Test raised unexpected warnings: {0!r}'.format(w))

        self.conn.request.assert_called_once()
        self.conn.close.assert_called_once()

    @mock.patch('ddtrace.compat.httplib.HTTPConnection')
    def test_put_connection_close_exception(self, HTTPConnection):
        """
        When calling API._put raises an exception
            we close the HTTPConnection we create
        """
        HTTPConnection.return_value = self.conn
        # Ensure calling `request` raises an exception
        self.conn.request.side_effect = Exception

        with warnings.catch_warnings(record=True) as w:
            with self.assertRaises(Exception):
                self.api._put('/test', '<test data>', 1)

            self.assertEqual(len(w), 0, 'Test raised unexpected warnings: {0!r}'.format(w))

        self.conn.request.assert_called_once()
        self.conn.close.assert_called_once()


def test_https():
    conn = mock.MagicMock(spec=httplib.HTTPSConnection)
    api = API('localhost', 8126, https=True)
    with mock.patch('ddtrace.compat.httplib.HTTPSConnection') as HTTPSConnection:
        HTTPSConnection.return_value = conn
        api._put('/test', '<test data>', 1)
    conn.request.assert_called_once()
    conn.close.assert_called_once()


def test_flush_connection_timeout_connect():
    payload = mock.Mock()
    payload.get_payload.return_value = 'foobar'
    payload.length = 12
    api = API(_HOST, 2019)
    response = api._flush(payload)
    if PY3:
        assert isinstance(response, (OSError, ConnectionRefusedError))  # noqa: F821
    else:
        assert isinstance(response, socket.error)
    assert response.errno in (99, 111)


def test_flush_connection_timeout(endpoint_test_timeout_server):
    payload = mock.Mock()
    payload.get_payload.return_value = 'foobar'
    payload.length = 12
    api = API(_HOST, _TIMEOUT_PORT)
    response = api._flush(payload)
    assert isinstance(response, socket.timeout)


def test_flush_connection_reset(endpoint_test_reset_server):
    payload = mock.Mock()
    payload.get_payload.return_value = 'foobar'
    payload.length = 12
    api = API(_HOST, _RESET_PORT)
    response = api._flush(payload)
    if PY3:
        assert isinstance(response, (httplib.BadStatusLine, ConnectionResetError))  # noqa: F821
    else:
        assert isinstance(response, httplib.BadStatusLine)


def test_flush_connection_uds(endpoint_uds_server):
    payload = mock.Mock()
    payload.get_payload.return_value = 'foobar'
    payload.length = 12
    api = API(_HOST, 2019, uds_path=endpoint_uds_server.server_address)
    response = api._flush(payload)
    assert response.status == 200


@mock.patch('ddtrace.internal.runtime.container.get_container_info')
def test_api_container_info(get_container_info):
    # When we have container information
    # DEV: `get_container_info` will return a `CGroupInfo` with a `container_id` or `None`
    info = CGroupInfo(container_id='test-container-id')
    get_container_info.return_value = info

    api = API(_HOST, 8126)
    assert api._container_info is info
    assert api._headers['Datadog-Container-Id'] == 'test-container-id'

    # When we do not have container information
    get_container_info.return_value = None

    api = API(_HOST, 8126)
    assert api._container_info is None
    assert 'Datadog-Container-Id' not in api._headers
