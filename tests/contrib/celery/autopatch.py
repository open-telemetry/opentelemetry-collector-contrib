from ddtrace import Pin

if __name__ == '__main__':
    # have to import celery in order to have the post-import hooks run
    import celery

    # now celery.Celery should be patched and should have a pin
    assert Pin.get_from(celery.Celery)
    print('Test success')
