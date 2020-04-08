from ddtrace import monkey

if __name__ == '__main__':
    assert 'redis' in monkey.get_patched_modules()
    print('Test success')
