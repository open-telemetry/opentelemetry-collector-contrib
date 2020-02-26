import sys
import unittest


class TestRequestsGevent(unittest.TestCase):
    def test_patch(self):
        """
        Patching `requests` before `gevent` monkeypatching

        This is a regression test for https://github.com/DataDog/dd-trace-py/issues/506

        When using `ddtrace-run` along with `requests` and `gevent` our patching causes
        `requests` and `urllib3` to get loaded before `gevent` has a chance to monkey patch.

        This causes `gevent` to show a warning and under certain versions cause
        a maxiumum recursion exception to be raised.
        """
        # Assert none of our modules have been imported yet
        # DEV: This regression test depends on being able to control import order of these modules
        # DEV: This is not entirely necessary but is a nice safe guard
        self.assertNotIn('ddtrace', sys.modules)
        self.assertNotIn('gevent', sys.modules)
        self.assertNotIn('requests', sys.modules)
        self.assertNotIn('urllib3', sys.modules)

        try:
            # Import ddtrace and patch only `requests`
            # DEV: We do not need to patch `gevent` for the exception to occur
            from ddtrace import patch
            patch(requests=True)

            # Import gevent and monkeypatch
            from gevent import monkey
            monkey.patch_all()

            # This is typically what will fail if `requests` (or `urllib3`)
            # gets loaded before running `monkey.patch_all()`
            # DEV: We are testing that no exception gets raised
            import requests

            # DEV: We **MUST** use an HTTPS request, that is what causes the issue
            requests.get('https://httpbin.org/get')

        finally:
            # Ensure we always unpatch `requests` when we are done
            from ddtrace.contrib.requests import unpatch
            unpatch()
