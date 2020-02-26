from ddtrace import tracer

if __name__ == '__main__':
    assert tracer.tags['a'] == 'True'
    assert tracer.tags['b'] == '0'
    assert tracer.tags['c'] == 'C'
    print('Test success')
