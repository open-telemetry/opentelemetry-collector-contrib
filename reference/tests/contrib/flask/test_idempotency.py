import mock
import unittest

import flask
from ddtrace.vendor import wrapt

from ddtrace.contrib.flask import patch, unpatch
from ddtrace.contrib.flask.patch import _w, _u


class FlaskIdempotencyTestCase(unittest.TestCase):
    def tearDown(self):
        # Double check we unpatch after every test
        unpatch()

    def assert_is_patched(self):
        self.assertTrue(flask._datadog_patch)
        self.assertTrue(isinstance(flask.render_template, wrapt.ObjectProxy))

    def assert_is_not_patched(self):
        self.assertFalse(flask._datadog_patch)
        self.assertFalse(isinstance(flask.render_template, wrapt.ObjectProxy))

    def test_datadog_patch(self):
        # If we have been patching/testing in other files,
        #   at least make sure this is where we want it
        if hasattr(flask, '_datadog_patch'):
            self.assertFalse(flask._datadog_patch)

        # Patching sets `_datadog_patch` to `True`
        patch()
        self.assert_is_patched()

        # Unpatching sets `_datadog_patch` to `False`
        unpatch()
        self.assert_is_not_patched()

    # DEV: Use `side_effect` so the original function still gets called
    @mock.patch('ddtrace.contrib.flask._patch._w', side_effect=_w)
    def test_patch_idempotency(self, _w):
        # Ensure we didn't do any patching automatically
        _w.assert_not_called()
        self.assert_is_not_patched()

        # Patch for the first time
        patch()
        _w.assert_called()
        self.assert_is_patched()

        # Reset the mock so we can assert call count
        _w.reset_mock()

        # Call patch a second time
        patch()
        _w.assert_not_called()
        self.assert_is_patched()

    # DEV: Use `side_effect` so the original function still gets called
    @mock.patch('ddtrace.contrib.flask._patch._w', side_effect=_w)
    @mock.patch('ddtrace.contrib.flask._patch._u', side_effect=_u)
    def test_unpatch_idempotency(self, _u, _w):
        # We need to patch in order to unpatch
        patch()
        _w.assert_called()
        self.assert_is_patched()

        # Ensure we didn't do any unpatching automatically
        _u.assert_not_called()

        unpatch()
        _u.assert_called()
        self.assert_is_not_patched()

        # Reset the mock so we can assert call count
        _u.reset_mock()

        # Call unpatch a second time
        unpatch()
        _u.assert_not_called()
        self.assert_is_not_patched()
