import logging

from ddtrace import tracer

if __name__ == '__main__':
    assert not tracer.log.isEnabledFor(logging.DEBUG)
    print('Test success')
