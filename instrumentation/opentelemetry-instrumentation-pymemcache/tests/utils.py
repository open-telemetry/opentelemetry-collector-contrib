# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import collections
import socket


class MockSocket:
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

    def recv(self, size):  # pylint: disable=unused-argument
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


class MockSocketModule:
    def __init__(self, connect_failure=None):
        self.connect_failure = connect_failure
        self.sockets = []

    def socket(self):  # noqa: A002
        mock_socket = MockSocket([], connect_failure=self.connect_failure)
        self.sockets.append(mock_socket)
        return mock_socket

    def __getattr__(self, name):
        return getattr(socket, name)


# Compatibility to get a string back from a request
def _str(string_input):
    if isinstance(string_input, str):
        return string_input
    if isinstance(string_input, bytes):
        return string_input.decode()

    return str(string_input)
