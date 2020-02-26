import grpc
from grpc._grpcio_metadata import __version__ as _GRPC_VERSION
import time
from grpc.framework.foundation import logging_pool
from ddtrace.constants import ANALYTICS_SAMPLE_RATE_KEY
from ddtrace.contrib.grpc import patch, unpatch
from ddtrace.contrib.grpc import constants
from ddtrace.contrib.grpc.patch import _unpatch_server
from ddtrace.ext import errors
from ddtrace import Pin

from ...base import BaseTracerTestCase

from .hello_pb2 import HelloRequest, HelloReply
from .hello_pb2_grpc import add_HelloServicer_to_server, HelloStub, HelloServicer

_GRPC_PORT = 50531
_GRPC_VERSION = tuple([int(i) for i in _GRPC_VERSION.split('.')])


class GrpcTestCase(BaseTracerTestCase):
    def setUp(self):
        super(GrpcTestCase, self).setUp()
        patch()
        Pin.override(constants.GRPC_PIN_MODULE_SERVER, tracer=self.tracer)
        Pin.override(constants.GRPC_PIN_MODULE_CLIENT, tracer=self.tracer)
        self._start_server()

    def tearDown(self):
        self._stop_server()
        # Remove any remaining spans
        self.tracer.writer.pop()
        # Unpatch grpc
        unpatch()
        super(GrpcTestCase, self).tearDown()

    def get_spans_with_sync_and_assert(self, size=0, retry=20):
        # testing instrumentation with grpcio < 1.14.0 presents a problem for
        # checking spans written to the dummy tracer
        # see https://github.com/grpc/grpc/issues/14621

        spans = super(GrpcTestCase, self).get_spans()

        if _GRPC_VERSION >= (1, 14):
            assert len(spans) == size
            return spans

        for _ in range(retry):
            if len(spans) == size:
                assert len(spans) == size
                return spans
            time.sleep(0.1)

        return spans

    def _start_server(self):
        self._server = grpc.server(logging_pool.pool(2))
        self._server.add_insecure_port('[::]:%d' % (_GRPC_PORT))
        add_HelloServicer_to_server(_HelloServicer(), self._server)
        self._server.start()

    def _stop_server(self):
        self._server.stop(0)

    def _check_client_span(self, span, service, method_name, method_kind):
        assert span.name == 'grpc'
        assert span.resource == '/helloworld.Hello/{}'.format(method_name)
        assert span.service == service
        assert span.error == 0
        assert span.span_type == 'grpc'
        assert span.get_tag('grpc.method.path') == '/helloworld.Hello/{}'.format(method_name)
        assert span.get_tag('grpc.method.package') == 'helloworld'
        assert span.get_tag('grpc.method.service') == 'Hello'
        assert span.get_tag('grpc.method.name') == method_name
        assert span.get_tag('grpc.method.kind') == method_kind
        assert span.get_tag('grpc.status.code') == 'StatusCode.OK'
        assert span.get_tag('grpc.host') == 'localhost'
        assert span.get_tag('grpc.port') == '50531'

    def _check_server_span(self, span, service, method_name, method_kind):
        assert span.name == 'grpc'
        assert span.resource == '/helloworld.Hello/{}'.format(method_name)
        assert span.service == service
        assert span.error == 0
        assert span.span_type == 'grpc'
        assert span.get_tag('grpc.method.path') == '/helloworld.Hello/{}'.format(method_name)
        assert span.get_tag('grpc.method.package') == 'helloworld'
        assert span.get_tag('grpc.method.service') == 'Hello'
        assert span.get_tag('grpc.method.name') == method_name
        assert span.get_tag('grpc.method.kind') == method_kind

    def test_insecure_channel_using_args_parameter(self):
        def insecure_channel_using_args(target):
            return grpc.insecure_channel(target)
        self._test_insecure_channel(insecure_channel_using_args)

    def test_insecure_channel_using_kwargs_parameter(self):
        def insecure_channel_using_kwargs(target):
            return grpc.insecure_channel(target=target)
        self._test_insecure_channel(insecure_channel_using_kwargs)

    def _test_insecure_channel(self, insecure_channel_function):
        target = 'localhost:%d' % (_GRPC_PORT)
        with insecure_channel_function(target) as channel:
            stub = HelloStub(channel)
            stub.SayHello(HelloRequest(name='test'))

        spans = self.get_spans_with_sync_and_assert(size=2)
        server_span, client_span = spans

        self._check_client_span(client_span, 'grpc-client', 'SayHello', 'unary')
        self._check_server_span(server_span, 'grpc-server', 'SayHello', 'unary')

    def test_secure_channel_using_args_parameter(self):
        def secure_channel_using_args(target, **kwargs):
            return grpc.secure_channel(target, **kwargs)
        self._test_secure_channel(secure_channel_using_args)

    def test_secure_channel_using_kwargs_parameter(self):
        def secure_channel_using_kwargs(target, **kwargs):
            return grpc.secure_channel(target=target, **kwargs)
        self._test_secure_channel(secure_channel_using_kwargs)

    def _test_secure_channel(self, secure_channel_function):
        target = 'localhost:%d' % (_GRPC_PORT)
        with secure_channel_function(target, credentials=grpc.ChannelCredentials(None)) as channel:
            stub = HelloStub(channel)
            stub.SayHello(HelloRequest(name='test'))

        spans = self.get_spans_with_sync_and_assert(size=2)
        server_span, client_span = spans

        self._check_client_span(client_span, 'grpc-client', 'SayHello', 'unary')
        self._check_server_span(server_span, 'grpc-server', 'SayHello', 'unary')

    def test_pin_not_activated(self):
        self.tracer.configure(enabled=False)
        with grpc.insecure_channel('localhost:%d' % (_GRPC_PORT)) as channel:
            stub = HelloStub(channel)
            stub.SayHello(HelloRequest(name='test'))

        spans = self.get_spans_with_sync_and_assert()
        assert len(spans) == 0

    def test_pin_tags_are_put_in_span(self):
        # DEV: stop and restart server to catch overriden pin
        self._stop_server()
        Pin.override(constants.GRPC_PIN_MODULE_SERVER, service='server1')
        Pin.override(constants.GRPC_PIN_MODULE_SERVER, tags={'tag1': 'server'})
        Pin.override(constants.GRPC_PIN_MODULE_CLIENT, tags={'tag2': 'client'})
        self._start_server()
        with grpc.insecure_channel('localhost:%d' % (_GRPC_PORT)) as channel:
            stub = HelloStub(channel)
            stub.SayHello(HelloRequest(name='test'))

        spans = self.get_spans_with_sync_and_assert(size=2)
        assert spans[0].service == 'server1'
        assert spans[0].get_tag('tag1') == 'server'
        assert spans[1].get_tag('tag2') == 'client'

    def test_pin_can_be_defined_per_channel(self):
        Pin.override(constants.GRPC_PIN_MODULE_CLIENT, service='grpc1')
        channel1 = grpc.insecure_channel('localhost:%d' % (_GRPC_PORT))

        Pin.override(constants.GRPC_PIN_MODULE_CLIENT, service='grpc2')
        channel2 = grpc.insecure_channel('localhost:%d' % (_GRPC_PORT))

        stub1 = HelloStub(channel1)
        stub1.SayHello(HelloRequest(name='test'))
        channel1.close()

        # DEV: make sure we have two spans before proceeding
        spans = self.get_spans_with_sync_and_assert(size=2)

        stub2 = HelloStub(channel2)
        stub2.SayHello(HelloRequest(name='test'))
        channel2.close()

        spans = self.get_spans_with_sync_and_assert(size=4)

        # DEV: Server service default, client services override
        self._check_server_span(spans[0], 'grpc-server', 'SayHello', 'unary')
        self._check_client_span(spans[1], 'grpc1', 'SayHello', 'unary')
        self._check_server_span(spans[2], 'grpc-server', 'SayHello', 'unary')
        self._check_client_span(spans[3], 'grpc2', 'SayHello', 'unary')

    def test_analytics_default(self):
        with grpc.secure_channel('localhost:%d' % (_GRPC_PORT), credentials=grpc.ChannelCredentials(None)) as channel:
            stub = HelloStub(channel)
            stub.SayHello(HelloRequest(name='test'))

        spans = self.get_spans_with_sync_and_assert(size=2)
        assert spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY) is None
        assert spans[1].get_metric(ANALYTICS_SAMPLE_RATE_KEY) is None

    def test_analytics_with_rate(self):
        with self.override_config(
            'grpc_server',
            dict(analytics_enabled=True, analytics_sample_rate=0.75)
        ):
            with self.override_config(
                'grpc',
                dict(analytics_enabled=True, analytics_sample_rate=0.5)
            ):
                with grpc.secure_channel(
                        'localhost:%d' % (_GRPC_PORT),
                        credentials=grpc.ChannelCredentials(None)
                ) as channel:
                    stub = HelloStub(channel)
                    stub.SayHello(HelloRequest(name='test'))

        spans = self.get_spans_with_sync_and_assert(size=2)
        assert spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY) == 0.75
        assert spans[1].get_metric(ANALYTICS_SAMPLE_RATE_KEY) == 0.5

    def test_analytics_without_rate(self):
        with self.override_config(
            'grpc_server',
            dict(analytics_enabled=True)
        ):
            with self.override_config(
                'grpc',
                dict(analytics_enabled=True)
            ):
                with grpc.secure_channel(
                        'localhost:%d' % (_GRPC_PORT),
                        credentials=grpc.ChannelCredentials(None)
                ) as channel:
                    stub = HelloStub(channel)
                    stub.SayHello(HelloRequest(name='test'))

        spans = self.get_spans_with_sync_and_assert(size=2)
        assert spans[0].get_metric(ANALYTICS_SAMPLE_RATE_KEY) == 1.0
        assert spans[1].get_metric(ANALYTICS_SAMPLE_RATE_KEY) == 1.0

    def test_server_stream(self):
        with grpc.insecure_channel('localhost:%d' % (_GRPC_PORT)) as channel:
            stub = HelloStub(channel)
            responses_iterator = stub.SayHelloTwice(HelloRequest(name='test'))
            assert len(list(responses_iterator)) == 2

        spans = self.get_spans_with_sync_and_assert(size=2)
        server_span, client_span = spans
        self._check_client_span(client_span, 'grpc-client', 'SayHelloTwice', 'server_streaming')
        self._check_server_span(server_span, 'grpc-server', 'SayHelloTwice', 'server_streaming')

    def test_client_stream(self):
        requests_iterator = iter(
            HelloRequest(name=name) for name in
            ['first', 'second']
        )

        with grpc.insecure_channel('localhost:%d' % (_GRPC_PORT)) as channel:
            stub = HelloStub(channel)
            response = stub.SayHelloLast(requests_iterator)
            assert response.message == 'first;second'

        spans = self.get_spans_with_sync_and_assert(size=2)
        server_span, client_span = spans
        self._check_client_span(client_span, 'grpc-client', 'SayHelloLast', 'client_streaming')
        self._check_server_span(server_span, 'grpc-server', 'SayHelloLast', 'client_streaming')

    def test_bidi_stream(self):
        requests_iterator = iter(
            HelloRequest(name=name) for name in
            ['first', 'second', 'third', 'fourth', 'fifth']
        )

        with grpc.insecure_channel('localhost:%d' % (_GRPC_PORT)) as channel:
            stub = HelloStub(channel)
            responses = stub.SayHelloRepeatedly(requests_iterator)
            messages = [r.message for r in responses]
            assert list(messages) == ['first;second', 'third;fourth', 'fifth']

        spans = self.get_spans_with_sync_and_assert(size=2)
        server_span, client_span = spans
        self._check_client_span(client_span, 'grpc-client', 'SayHelloRepeatedly', 'bidi_streaming')
        self._check_server_span(server_span, 'grpc-server', 'SayHelloRepeatedly', 'bidi_streaming')

    def test_priority_sampling(self):
        # DEV: Priority sampling is enabled by default
        # Setting priority sampling reset the writer, we need to re-override it

        with grpc.insecure_channel('localhost:%d' % (_GRPC_PORT)) as channel:
            stub = HelloStub(channel)
            response = stub.SayHello(HelloRequest(name='propogator'))

        spans = self.get_spans_with_sync_and_assert(size=2)
        server_span, client_span = spans

        assert 'x-datadog-trace-id={}'.format(client_span.trace_id) in response.message
        assert 'x-datadog-parent-id={}'.format(client_span.span_id) in response.message
        assert 'x-datadog-sampling-priority=1' in response.message

    def test_unary_abort(self):
        with grpc.secure_channel('localhost:%d' % (_GRPC_PORT), credentials=grpc.ChannelCredentials(None)) as channel:
            stub = HelloStub(channel)
            with self.assertRaises(grpc.RpcError):
                stub.SayHello(HelloRequest(name='abort'))

        spans = self.get_spans_with_sync_and_assert(size=2)
        server_span, client_span = spans

        assert client_span.resource == '/helloworld.Hello/SayHello'
        assert client_span.error == 1
        assert client_span.get_tag(errors.ERROR_MSG) == 'aborted'
        assert client_span.get_tag(errors.ERROR_TYPE) == 'StatusCode.ABORTED'
        assert client_span.get_tag('grpc.status.code') == 'StatusCode.ABORTED'

    def test_custom_interceptor_exception(self):
        # add an interceptor that raises a custom exception and check error tags
        # are added to spans
        raise_exception_interceptor = _RaiseExceptionClientInterceptor()
        with grpc.insecure_channel('localhost:%d' % (_GRPC_PORT)) as channel:
            with self.assertRaises(_CustomException):
                intercept_channel = grpc.intercept_channel(
                    channel,
                    raise_exception_interceptor
                )
                stub = HelloStub(intercept_channel)
                stub.SayHello(HelloRequest(name='custom-exception'))

        spans = self.get_spans_with_sync_and_assert(size=2)
        server_span, client_span = spans

        assert client_span.resource == '/helloworld.Hello/SayHello'
        assert client_span.error == 1
        assert client_span.get_tag(errors.ERROR_MSG) == 'custom'
        assert client_span.get_tag(errors.ERROR_TYPE) == 'tests.contrib.grpc.test_grpc._CustomException'
        assert client_span.get_tag(errors.ERROR_STACK) is not None
        assert client_span.get_tag('grpc.status.code') == 'StatusCode.INTERNAL'

        # no exception on server end
        assert server_span.resource == '/helloworld.Hello/SayHello'
        assert server_span.error == 0
        assert server_span.get_tag(errors.ERROR_MSG) is None
        assert server_span.get_tag(errors.ERROR_TYPE) is None
        assert server_span.get_tag(errors.ERROR_STACK) is None

    def test_client_cancellation(self):
        # unpatch and restart server since we are only testing here caller cancellation
        self._stop_server()
        _unpatch_server()
        self._start_server()

        # have servicer sleep whenever request is handled to ensure we can cancel before server responds
        # to requests
        requests_iterator = iter(
            HelloRequest(name=name) for name in
            ['sleep']
        )

        with grpc.insecure_channel('localhost:%d' % (_GRPC_PORT)) as channel:
            with self.assertRaises(grpc.RpcError):
                stub = HelloStub(channel)
                responses = stub.SayHelloRepeatedly(requests_iterator)
                responses.cancel()
                next(responses)

        spans = self.get_spans_with_sync_and_assert(size=1)
        client_span = spans[0]

        assert client_span.resource == '/helloworld.Hello/SayHelloRepeatedly'
        assert client_span.error == 1
        assert client_span.get_tag(errors.ERROR_MSG) == 'Locally cancelled by application!'
        assert client_span.get_tag(errors.ERROR_TYPE) == 'StatusCode.CANCELLED'
        assert client_span.get_tag(errors.ERROR_STACK) is None
        assert client_span.get_tag('grpc.status.code') == 'StatusCode.CANCELLED'

    def test_unary_exception(self):
        with grpc.secure_channel('localhost:%d' % (_GRPC_PORT), credentials=grpc.ChannelCredentials(None)) as channel:
            stub = HelloStub(channel)
            with self.assertRaises(grpc.RpcError):
                stub.SayHello(HelloRequest(name='exception'))

        spans = self.get_spans_with_sync_and_assert(size=2)
        server_span, client_span = spans

        assert client_span.resource == '/helloworld.Hello/SayHello'
        assert client_span.error == 1
        assert client_span.get_tag(errors.ERROR_MSG) == 'exception'
        assert client_span.get_tag(errors.ERROR_TYPE) == 'StatusCode.INVALID_ARGUMENT'
        assert client_span.get_tag('grpc.status.code') == 'StatusCode.INVALID_ARGUMENT'

        assert server_span.resource == '/helloworld.Hello/SayHello'
        assert server_span.error == 1
        assert server_span.get_tag(errors.ERROR_MSG) == 'exception'
        assert server_span.get_tag(errors.ERROR_TYPE) == 'StatusCode.INVALID_ARGUMENT'
        assert 'Traceback' in server_span.get_tag(errors.ERROR_STACK)
        assert 'grpc.StatusCode.INVALID_ARGUMENT' in server_span.get_tag(errors.ERROR_STACK)

    def test_client_stream_exception(self):
        requests_iterator = iter(
            HelloRequest(name=name) for name in
            ['first', 'exception']
        )

        with grpc.insecure_channel('localhost:%d' % (_GRPC_PORT)) as channel:
            stub = HelloStub(channel)
            with self.assertRaises(grpc.RpcError):
                stub.SayHelloLast(requests_iterator)

        spans = self.get_spans_with_sync_and_assert(size=2)
        server_span, client_span = spans

        assert client_span.resource == '/helloworld.Hello/SayHelloLast'
        assert client_span.error == 1
        assert client_span.get_tag(errors.ERROR_MSG) == 'exception'
        assert client_span.get_tag(errors.ERROR_TYPE) == 'StatusCode.INVALID_ARGUMENT'
        assert client_span.get_tag('grpc.status.code') == 'StatusCode.INVALID_ARGUMENT'

        assert server_span.resource == '/helloworld.Hello/SayHelloLast'
        assert server_span.error == 1
        assert server_span.get_tag(errors.ERROR_MSG) == 'exception'
        assert server_span.get_tag(errors.ERROR_TYPE) == 'StatusCode.INVALID_ARGUMENT'
        assert 'Traceback' in server_span.get_tag(errors.ERROR_STACK)
        assert 'grpc.StatusCode.INVALID_ARGUMENT' in server_span.get_tag(errors.ERROR_STACK)

    def test_server_stream_exception(self):
        with grpc.secure_channel('localhost:%d' % (_GRPC_PORT), credentials=grpc.ChannelCredentials(None)) as channel:
            stub = HelloStub(channel)
            with self.assertRaises(grpc.RpcError):
                list(stub.SayHelloTwice(HelloRequest(name='exception')))

        spans = self.get_spans_with_sync_and_assert(size=2)
        server_span, client_span = spans

        assert client_span.resource == '/helloworld.Hello/SayHelloTwice'
        assert client_span.error == 1
        assert client_span.get_tag(errors.ERROR_MSG) == 'exception'
        assert client_span.get_tag(errors.ERROR_TYPE) == 'StatusCode.RESOURCE_EXHAUSTED'
        assert client_span.get_tag('grpc.status.code') == 'StatusCode.RESOURCE_EXHAUSTED'

        assert server_span.resource == '/helloworld.Hello/SayHelloTwice'
        assert server_span.error == 1
        assert server_span.get_tag(errors.ERROR_MSG) == 'exception'
        assert server_span.get_tag(errors.ERROR_TYPE) == 'StatusCode.RESOURCE_EXHAUSTED'
        assert 'Traceback' in server_span.get_tag(errors.ERROR_STACK)
        assert 'grpc.StatusCode.RESOURCE_EXHAUSTED' in server_span.get_tag(errors.ERROR_STACK)


class _HelloServicer(HelloServicer):
    def SayHello(self, request, context):
        if request.name == 'propogator':
            metadata = context.invocation_metadata()
            context.set_code(grpc.StatusCode.OK)
            message = ';'.join(
                w.key + '=' + w.value
                for w in metadata
                if w.key.startswith('x-datadog')
            )
            return HelloReply(message=message)

        if request.name == 'abort':
            context.abort(grpc.StatusCode.ABORTED, 'aborted')

        if request.name == 'exception':
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, 'exception')

        return HelloReply(message='Hello {}'.format(request.name))

    def SayHelloTwice(self, request, context):
        yield HelloReply(message='first response')

        if request.name == 'exception':
            context.abort(grpc.StatusCode.RESOURCE_EXHAUSTED, 'exception')

        yield HelloReply(message='secondresponse')

    def SayHelloLast(self, request_iterator, context):
        names = [r.name for r in list(request_iterator)]

        if 'exception' in names:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, 'exception')

        return HelloReply(message='{}'.format(
            ';'.join(names)))

    def SayHelloRepeatedly(self, request_iterator, context):
        last_request = None
        for request in request_iterator:
            if last_request is not None:
                yield HelloReply(message='{}'.format(
                    ';'.join([last_request.name, request.name])
                ))
                last_request = None
            else:
                last_request = request

        # response for dangling request
        if last_request is not None:
            yield HelloReply(message='{}'.format(last_request.name))


class _CustomException(Exception):
    pass


class _RaiseExceptionClientInterceptor(grpc.UnaryUnaryClientInterceptor):
    def _intercept_call(self, continuation, client_call_details,
                        request_or_iterator):
        # allow computation to complete
        continuation(client_call_details, request_or_iterator).result()

        raise _CustomException('custom')

    def intercept_unary_unary(self, continuation, client_call_details, request):
        return self._intercept_call(continuation, client_call_details, request)
