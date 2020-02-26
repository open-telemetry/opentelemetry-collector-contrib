import collections
import socket


class MockSocket(object):
    def __init__(self, recv_bufs, connect_failure=None):
        self.recv_bufs = collections.deque(recv_bufs)
        self.send_bufs = []
        self.closed = False
        self.timeouts = []
        self.connect_failure = connect_failure
        self.connections = []
        self.socket_options = []

    def sendall(self, value):
        self.send_bufs.append(value)

    def close(self):
        self.closed = True

    def recv(self, size):
        value = self.recv_bufs.popleft()
        if isinstance(value, Exception):
            raise value
        return value

    def settimeout(self, timeout):
        self.timeouts.append(timeout)

    def connect(self, server):
        if isinstance(self.connect_failure, Exception):
            raise self.connect_failure
        self.connections.append(server)

    def setsockopt(self, level, option, value):
        self.socket_options.append((level, option, value))


class MockSocketModule(object):
    def __init__(self, connect_failure=None):
        self.connect_failure = connect_failure
        self.sockets = []

    def socket(self, family, type):  # noqa: A002
        socket = MockSocket([], connect_failure=self.connect_failure)
        self.sockets.append(socket)
        return socket

    def __getattr__(self, name):
        return getattr(socket, name)


# Compatibility to get a string back from a request
def _str(s):
    if type(s) is str:
        return s
    elif type(s) is bytes:
        return s.decode()
    else:
        return str(s)
