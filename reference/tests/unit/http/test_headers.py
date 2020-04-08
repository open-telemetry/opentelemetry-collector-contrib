import pytest

from ddtrace import tracer, Span
from ddtrace.http import store_request_headers, store_response_headers
from ddtrace.settings import Config, IntegrationConfig


class TestHeaders(object):

    @pytest.fixture()
    def span(self):
        yield Span(tracer, 'some_span')

    @pytest.fixture()
    def config(self):
        yield Config()

    @pytest.fixture()
    def integration_config(self, config):
        yield IntegrationConfig(config, 'test')

    def test_it_does_not_break_if_no_headers(self, span, integration_config):
        store_request_headers(None, span, integration_config)
        store_response_headers(None, span, integration_config)

    def test_it_does_not_break_if_headers_are_not_a_dict(self, span, integration_config):
        store_request_headers(list(), span, integration_config)
        store_response_headers(list(), span, integration_config)

    def test_it_accept_headers_as_list_of_tuples(self, span, integration_config):
        """
        :type span: Span
        :type integration_config: IntegrationConfig
        """
        integration_config.http.trace_headers(['Content-Type', 'Max-Age'])
        store_request_headers([('Content-Type', 'some;value;content-type')], span, integration_config)
        assert span.get_tag('http.request.headers.content-type') == 'some;value;content-type'
        assert None is span.get_tag('http.request.headers.other')

    def test_store_multiple_request_headers_as_dict(self, span, integration_config):
        """
        :type span: Span
        :type integration_config: IntegrationConfig
        """
        integration_config.http.trace_headers(['Content-Type', 'Max-Age'])
        store_request_headers({
            'Content-Type': 'some;value;content-type',
            'Max-Age': 'some;value;max_age',
            'Other': 'some;value;other',
        }, span, integration_config)
        assert span.get_tag('http.request.headers.content-type') == 'some;value;content-type'
        assert span.get_tag('http.request.headers.max-age') == 'some;value;max_age'
        assert None is span.get_tag('http.request.headers.other')

    def test_store_multiple_response_headers_as_dict(self, span, integration_config):
        """
        :type span: Span
        :type integration_config: IntegrationConfig
        """
        integration_config.http.trace_headers(['Content-Type', 'Max-Age'])
        store_response_headers({
            'Content-Type': 'some;value;content-type',
            'Max-Age': 'some;value;max_age',
            'Other': 'some;value;other',
        }, span, integration_config)
        assert span.get_tag('http.response.headers.content-type') == 'some;value;content-type'
        assert span.get_tag('http.response.headers.max-age') == 'some;value;max_age'
        assert None is span.get_tag('http.response.headers.other')

    def test_numbers_in_headers_names_are_allowed(self, span, integration_config):
        """
        :type span: Span
        :type integration_config: IntegrationConfig
        """
        integration_config.http.trace_headers('Content-Type123')
        store_response_headers({
            'Content-Type123': 'some;value',
        }, span, integration_config)
        assert span.get_tag('http.response.headers.content-type123') == 'some;value'

    def test_allowed_chars_not_replaced_in_tag_name(self, span, integration_config):
        """
        :type span: Span
        :type integration_config: IntegrationConfig
        """
        # See: https://docs.datadoghq.com/tagging/#defining-tags
        integration_config.http.trace_headers('C0n_t:e/nt-Type')
        store_response_headers({
            'C0n_t:e/nt-Type': 'some;value',
        }, span, integration_config)
        assert span.get_tag('http.response.headers.c0n_t:e/nt-type') == 'some;value'

    def test_period_is_replaced_by_underscore(self, span, integration_config):
        """
        :type span: Span
        :type integration_config: IntegrationConfig
        """
        # Deviation from https://docs.datadoghq.com/tagging/#defining-tags in order to allow
        # consistent representation of headers having the period in the name.
        integration_config.http.trace_headers('api.token')
        store_response_headers({
            'api.token': 'some;value',
        }, span, integration_config)
        assert span.get_tag('http.response.headers.api_token') == 'some;value'

    def test_non_allowed_chars_replaced(self, span, integration_config):
        """
        :type span: Span
        :type integration_config: IntegrationConfig
        """
        # See: https://docs.datadoghq.com/tagging/#defining-tags
        integration_config.http.trace_headers('C!#ontent-Type')
        store_response_headers({
            'C!#ontent-Type': 'some;value',
        }, span, integration_config)
        assert span.get_tag('http.response.headers.c__ontent-type') == 'some;value'

    def test_key_trim_leading_trailing_spaced(self, span, integration_config):
        """
        :type span: Span
        :type integration_config: IntegrationConfig
        """
        integration_config.http.trace_headers('Content-Type')
        store_response_headers({
            '   Content-Type   ': 'some;value',
        }, span, integration_config)
        assert span.get_tag('http.response.headers.content-type') == 'some;value'

    def test_value_not_trim_leading_trailing_spaced(self, span, integration_config):
        """
        :type span: Span
        :type integration_config: IntegrationConfig
        """
        integration_config.http.trace_headers('Content-Type')
        store_response_headers({
            'Content-Type': '   some;value   ',
        }, span, integration_config)
        assert span.get_tag('http.response.headers.content-type') == '   some;value   '

    def test_no_whitelist(self, span, integration_config):
        """
        :type span: Span
        :type integration_config: IntegrationConfig
        """
        store_response_headers({
            'Content-Type': 'some;value',
        }, span, integration_config)
        assert span.get_tag('http.response.headers.content-type') is None

    def test_whitelist_exact(self, span, integration_config):
        """
        :type span: Span
        :type integration_config: IntegrationConfig
        """
        integration_config.http.trace_headers('content-type')
        store_response_headers({
            'Content-Type': 'some;value',
        }, span, integration_config)
        assert span.get_tag('http.response.headers.content-type') == 'some;value'

    def test_whitelist_case_insensitive(self, span, integration_config):
        """
        :type span: Span
        :type integration_config: IntegrationConfig
        """
        integration_config.http.trace_headers('CoNtEnT-tYpE')
        store_response_headers({
            'cOnTeNt-TyPe': 'some;value',
        }, span, integration_config)
        assert span.get_tag('http.response.headers.content-type') == 'some;value'
