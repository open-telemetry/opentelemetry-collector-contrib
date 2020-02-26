"""
Helper(s), resolve the system's default interface.
"""
# stdlib
import socket
import struct


class UnresolvableDefaultRoute(Exception):
    """
    Unable to resolve system's default route.
    """


def get_default_route():
    """
    Return the system default interface using the proc filesystem.

    Returns:
        string: default route

    Raises:
        `NotImplementedError`: No proc filesystem is found (non-Linux systems)
        `StopIteration`: No default route found
    """
    try:
        with open('/proc/net/route') as f:
            for line in f.readlines():
                fields = line.strip().split()
                if fields[1] == '00000000':
                    return socket.inet_ntoa(struct.pack('<L', int(fields[2], 16)))
    except IOError:
        raise NotImplementedError(
            u"Unable to open `/proc/net/route`. "
            u"`use_default_route` option is available on Linux only."
        )

    raise UnresolvableDefaultRoute(u"Unable to resolve the system default's route.")
