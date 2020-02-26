
# 3p
from ddtrace.vendor import wrapt

# project
import ddtrace
from ddtrace.ext import mongo as mongox
from ddtrace.contrib.pymongo.client import TracedMongoClient


# TODO(Benjamin): we should instrument register_connection instead, because more generic
# We should also extract the "alias" attribute and set it as a meta
class WrappedConnect(wrapt.ObjectProxy):
    """ WrappedConnect wraps mongoengines 'connect' function to ensure
        that all returned connections are wrapped for tracing.
    """

    def __init__(self, connect):
        super(WrappedConnect, self).__init__(connect)
        ddtrace.Pin(service=mongox.SERVICE, tracer=ddtrace.tracer).onto(self)

    def __call__(self, *args, **kwargs):
        client = self.__wrapped__(*args, **kwargs)
        pin = ddtrace.Pin.get_from(self)
        if pin:
            # mongoengine uses pymongo internally, so we can just piggyback on the
            # existing pymongo integration and make sure that the connections it
            # uses internally are traced.
            client = TracedMongoClient(client)
            ddtrace.Pin(service=pin.service, tracer=pin.tracer).onto(client)

        return client
