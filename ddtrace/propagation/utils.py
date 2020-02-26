def get_wsgi_header(header):
    """Returns a WSGI compliant HTTP header.
    See https://www.python.org/dev/peps/pep-3333/#environ-variables for
    information from the spec.
    """
    return 'HTTP_{}'.format(header.upper().replace('-', '_'))
