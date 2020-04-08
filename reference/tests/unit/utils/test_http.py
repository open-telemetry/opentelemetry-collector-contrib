from ddtrace.utils.http import normalize_header_name


class TestHeaderNameNormalization(object):

    def test_name_is_trimmed(self):
        assert normalize_header_name('   content-type   ') == 'content-type'

    def test_name_is_lowered(self):
        assert normalize_header_name('Content-Type') == 'content-type'

    def test_none_does_not_raise_exception(self):
        assert normalize_header_name(None) is None

    def test_empty_does_not_raise_exception(self):
        assert normalize_header_name('') == ''
