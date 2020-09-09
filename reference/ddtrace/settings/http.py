from ..internal.logger import get_logger
from ..utils.http import normalize_header_name

log = get_logger(__name__)


class HttpConfig(object):
    """
    Configuration object that expose an API to set and retrieve both global and integration specific settings
    related to the http context.
    """

    def __init__(self):
        self._allowlist_headers = set()
        self.trace_query_string = None

    @property
    def is_header_tracing_configured(self):
        return len(self._allowlist_headers) > 0

    def trace_headers(self, allowlist):
        """
        Registers a set of headers to be traced at global level or integration level.
        :param allowlist: the case-insensitive list of traced headers
        :type allowlist: list of str or str
        :return: self
        :rtype: HttpConfig
        """
        if not allowlist:
            return

        allowlist = [allowlist] if isinstance(allowlist, str) else allowlist
        for allowlist_entry in allowlist:
            normalized_header_name = normalize_header_name(allowlist_entry)
            if not normalized_header_name:
                continue
            self._allowlist_headers.add(normalized_header_name)

        return self

    def header_is_traced(self, header_name):
        """
        Returns whether or not the current header should be traced.
        :param header_name: the header name
        :type header_name: str
        :rtype: bool
        """
        normalized_header_name = normalize_header_name(header_name)
        log.debug('Checking header \'%s\' tracing in allowlist %s', normalized_header_name, self._allowlist_headers)
        return normalized_header_name in self._allowlist_headers

    def __repr__(self):
        return '<{} traced_headers={} trace_query_string={}>'.format(
            self.__class__.__name__, self._allowlist_headers, self.trace_query_string)
