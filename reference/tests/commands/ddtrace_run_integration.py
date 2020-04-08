"""
An integration test that uses a real Redis client
that we expect to be implicitly traced via `ddtrace-run`
"""

import redis

from ddtrace import Pin
from tests.contrib.config import REDIS_CONFIG
from tests.test_tracer import DummyWriter

if __name__ == '__main__':
    r = redis.Redis(port=REDIS_CONFIG['port'])
    pin = Pin.get_from(r)
    assert pin
    assert pin.app == 'redis'
    assert pin.service == 'redis'

    pin.tracer.writer = DummyWriter()
    r.flushall()
    spans = pin.tracer.writer.pop()

    assert len(spans) == 1
    assert spans[0].service == 'redis'
    assert spans[0].resource == 'FLUSHALL'

    long_cmd = 'mget %s' % ' '.join(map(str, range(1000)))
    us = r.execute_command(long_cmd)

    spans = pin.tracer.writer.pop()
    assert len(spans) == 1
    span = spans[0]
    assert span.service == 'redis'
    assert span.name == 'redis.command'
    assert span.span_type == 'redis'
    assert span.error == 0
    assert span.get_metric('out.port') == REDIS_CONFIG['port']
    assert span.get_metric('out.redis_db') == 0
    assert span.get_tag('out.host') == 'localhost'
    assert span.get_tag('redis.raw_command').startswith(u'mget 0 1 2 3')
    assert span.get_tag('redis.raw_command').endswith(u'...')

    print('Test success')
