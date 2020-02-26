from __future__ import print_function

from ddtrace import tracer

if __name__ == '__main__':
    # check both configurations with host:port or unix socket
    if tracer._dogstatsd_client.socket_path is None:
        assert tracer._dogstatsd_client.host == '172.10.0.1'
        assert tracer._dogstatsd_client.port == 8120
    else:
        assert tracer._dogstatsd_client.socket_path.endswith('dogstatsd.sock')
    print('Test success')
