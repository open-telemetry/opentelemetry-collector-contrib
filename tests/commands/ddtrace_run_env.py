from ddtrace import tracer

if __name__ == '__main__':
    assert tracer.tags['env'] == 'test'
    print('Test success')
