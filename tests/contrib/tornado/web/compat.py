from tornado.concurrent import Future
import tornado.gen
from tornado.ioloop import IOLoop


try:
    from concurrent.futures import ThreadPoolExecutor
except ImportError:
    from tornado.concurrent import DummyExecutor

    class ThreadPoolExecutor(DummyExecutor):
        """
        Fake executor class used to test our tracer when Python 2 is used
        without the `futures` backport. This is not a real use case, but
        it's required to be defensive when we have different `Executor`
        implementations.
        """
        def __init__(self, *args, **kwargs):
            # we accept any kind of interface
            super(ThreadPoolExecutor, self).__init__()


if hasattr(tornado.gen, 'sleep'):
    sleep = tornado.gen.sleep
else:
    # Tornado <= 4.0
    def sleep(duration):
        """
        Compatibility helper that return a Future() that can be yielded.
        This is used because Tornado 4.0 doesn't have a ``gen.sleep()``
        function, that we require to test the ``TracerStackContext``.
        """
        f = Future()
        IOLoop.current().call_later(duration, lambda: f.set_result(None))
        return f
