from ddtrace.contrib.grpc.utils import parse_method_path


def test_parse_method_path_with_package():
    method_path = '/package.service/method'
    parsed = parse_method_path(method_path)
    assert parsed == ('package', 'service', 'method')


def test_parse_method_path_without_package():
    method_path = '/service/method'
    parsed = parse_method_path(method_path)
    assert parsed == (None, 'service', 'method')
