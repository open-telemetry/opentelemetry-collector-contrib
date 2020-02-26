from .encoding import get_encoder


class PayloadFull(Exception):
    """The payload is full."""
    pass


class Payload(object):
    """
    Trace agent API payload buffer class

    This class is used to encoded and store traces to build the payload we send to
    the trace agent.

    DEV: We encoded and buffer traces so that we can reliable determine the size of
         the payload easily so we can flush based on the payload size.
    """
    __slots__ = ('traces', 'size', 'encoder', 'max_payload_size')

    # Trace agent limit payload size of 10 MB
    # 5 MB should be a good average efficient size
    DEFAULT_MAX_PAYLOAD_SIZE = 5 * 1000000

    def __init__(self, encoder=None, max_payload_size=DEFAULT_MAX_PAYLOAD_SIZE):
        """
        Constructor for Payload

        :param encoder: The encoded to use, default is the default encoder
        :type encoder: ``ddtrace.encoding.Encoder``
        :param max_payload_size: The max number of bytes a payload should be before
            being considered full (default: 5mb)
        """
        self.max_payload_size = max_payload_size
        self.encoder = encoder or get_encoder()
        self.traces = []
        self.size = 0

    def add_trace(self, trace):
        """
        Encode and append a trace to this payload

        :param trace: A trace to append
        :type trace: A list of :class:`ddtrace.span.Span`
        """
        # No trace or empty trace was given, ignore
        if not trace:
            return

        # Encode the trace, append, and add it's length to the size
        encoded = self.encoder.encode_trace(trace)
        if len(encoded) + self.size > self.max_payload_size:
            raise PayloadFull()
        self.traces.append(encoded)
        self.size += len(encoded)

    @property
    def length(self):
        """
        Get the number of traces in this payload

        :returns: The number of traces in the payload
        :rtype: int
        """
        return len(self.traces)

    @property
    def empty(self):
        """
        Whether this payload is empty or not

        :returns: Whether this payload is empty or not
        :rtype: bool
        """
        return self.length == 0

    def get_payload(self):
        """
        Get the fully encoded payload

        :returns: The fully encoded payload
        :rtype: str | bytes
        """
        # DEV: `self.traces` is an array of encoded traces, `join_encoded` joins them together
        return self.encoder.join_encoded(self.traces)

    def __repr__(self):
        """Get the string representation of this payload"""
        return '{0}(length={1}, size={2} B, max_payload_size={3} B)'.format(
            self.__class__.__name__, self.length, self.size, self.max_payload_size)
