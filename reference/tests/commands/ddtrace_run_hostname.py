from ddtrace import tracer

if __name__ == '__main__':
    assert tracer.writer.api.hostname == '172.10.0.1'
    assert tracer.writer.api.port == 8120
    print('Test success')
