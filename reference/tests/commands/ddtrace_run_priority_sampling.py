from ddtrace import tracer

if __name__ == '__main__':
    assert tracer.priority_sampler is not None
    print('Test success')
