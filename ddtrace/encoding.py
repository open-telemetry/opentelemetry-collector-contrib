import json
import struct

from .internal.logger import get_logger


# Try to import msgpack, fallback to just JSON if something went wrong
# DEV: We are ok with the pure Python fallback for msgpack if the C-extension failed to install
try:
    from ddtrace.vendor import msgpack
    # DEV: `use_bin_type` only exists since `0.4.0`, but we vendor a more recent version
    MSGPACK_PARAMS = {'use_bin_type': True}
    MSGPACK_ENCODING = True
except ImportError:
    # fallback to JSON
    MSGPACK_PARAMS = {}
    MSGPACK_ENCODING = False

log = get_logger(__name__)


class Encoder(object):
    """
    Encoder interface that provides the logic to encode traces and service.
    """
    def __init__(self):
        """
        When extending the ``Encoder`` class, ``headers`` must be set because
        they're returned by the encoding methods, so that the API transport doesn't
        need to know what is the right header to suggest the decoding format to the
        agent
        """
        self.content_type = ''

    def encode_traces(self, traces):
        """
        Encodes a list of traces, expecting a list of items where each items
        is a list of spans. Before dump the string in a serialized format all
        traces are normalized, calling the ``to_dict()`` method. The traces
        nesting is not changed.

        :param traces: A list of traces that should be serialized
        """
        normalized_traces = [[span.to_dict() for span in trace] for trace in traces]
        return self.encode(normalized_traces)

    def encode_trace(self, trace):
        """
        Encodes a trace, expecting a list of spans. Before dump the string in a
        serialized format all traces are normalized, calling the ``to_dict()`` method.
        The traces nesting is not changed.

        :param trace: A list of traces that should be serialized
        """
        return self.encode([span.to_dict() for span in trace])

    def encode(self, obj):
        """
        Defines the underlying format used during traces or services encoding.
        This method must be implemented and should only be used by the internal functions.
        """
        raise NotImplementedError

    def decode(self, data):
        """
        Defines the underlying format used during traces or services encoding.
        This method must be implemented and should only be used by the internal functions.
        """
        raise NotImplementedError

    def join_encoded(self, objs):
        """Helper used to join a list of encoded objects into an encoded list of objects"""
        raise NotImplementedError


class JSONEncoder(Encoder):
    def __init__(self):
        # TODO[manu]: add instructions about how users can switch to Msgpack
        log.debug('using JSON encoder; application performance may be degraded')
        self.content_type = 'application/json'

    def encode(self, obj):
        return json.dumps(obj)

    def decode(self, data):
        return json.loads(data)

    def join_encoded(self, objs):
        """Join a list of encoded objects together as a json array"""
        return '[' + ','.join(objs) + ']'


class MsgpackEncoder(Encoder):
    def __init__(self):
        log.debug('using Msgpack encoder')
        self.content_type = 'application/msgpack'

    def encode(self, obj):
        return msgpack.packb(obj)

    def decode(self, data):
        return msgpack.unpackb(data)

    def join_encoded(self, objs):
        """Join a list of encoded objects together as a msgpack array"""
        buf = b''.join(objs)

        # Prepend array header to buffer
        # https://github.com/msgpack/msgpack-python/blob/f46523b1af7ff2d408da8500ea36a4f9f2abe915/msgpack/fallback.py#L948-L955
        count = len(objs)
        if count <= 0xf:
            return struct.pack('B', 0x90 + count) + buf
        elif count <= 0xffff:
            return struct.pack('>BH', 0xdc, count) + buf
        else:
            return struct.pack('>BI', 0xdd, count) + buf


def get_encoder():
    """
    Switching logic that choose the best encoder for the API transport.
    The default behavior is to use Msgpack if we have a CPP implementation
    installed, falling back to the Python built-in JSON encoder.
    """
    if MSGPACK_ENCODING:
        return MsgpackEncoder()
    else:
        return JSONEncoder()
