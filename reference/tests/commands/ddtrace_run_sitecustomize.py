import sys


if __name__ == '__main__':
    # detect if `-S` is used
    suppress = len(sys.argv) == 2 and sys.argv[1] == '-S'
    if suppress:
        assert 'sitecustomize' not in sys.modules
    else:
        assert 'sitecustomize' in sys.modules

    # ensure the right `sitecustomize` will be imported
    import sitecustomize
    assert sitecustomize.CORRECT_IMPORT
    print('Test success')
