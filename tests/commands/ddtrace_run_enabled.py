from ddtrace import tracer

if __name__ == '__main__':
    assert tracer.enabled
    print('Test success')
