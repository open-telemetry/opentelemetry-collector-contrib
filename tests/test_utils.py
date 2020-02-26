import os
import unittest
import warnings

from ddtrace.utils.deprecation import deprecation, deprecated, format_message
from ddtrace.utils.formats import asbool, get_env, flatten_dict


class TestUtils(unittest.TestCase):
    def test_asbool(self):
        # ensure the value is properly cast
        self.assertTrue(asbool('True'))
        self.assertTrue(asbool('true'))
        self.assertTrue(asbool('1'))
        self.assertFalse(asbool('False'))
        self.assertFalse(asbool('false'))
        self.assertFalse(asbool(None))
        self.assertFalse(asbool(''))
        self.assertTrue(asbool(True))
        self.assertFalse(asbool(False))

    def test_get_env(self):
        # ensure `get_env` returns a default value if environment variables
        # are not set
        value = get_env('django', 'distributed_tracing')
        self.assertIsNone(value)
        value = get_env('django', 'distributed_tracing', False)
        self.assertFalse(value)

    def test_get_env_found(self):
        # ensure `get_env` returns a value if the environment variable is set
        os.environ['DD_REQUESTS_DISTRIBUTED_TRACING'] = '1'
        value = get_env('requests', 'distributed_tracing')
        self.assertEqual(value, '1')

    def test_get_env_found_legacy(self):
        # ensure `get_env` returns a value if legacy environment variables
        # are used, raising a Deprecation warning
        with warnings.catch_warnings(record=True) as w:
            warnings.simplefilter('always')
            os.environ['DATADOG_REQUESTS_DISTRIBUTED_TRACING'] = '1'
            value = get_env('requests', 'distributed_tracing')
            self.assertEqual(value, '1')
            self.assertEqual(len(w), 1)
            self.assertTrue(issubclass(w[-1].category, DeprecationWarning))
            self.assertTrue('Use `DD_` prefix instead' in str(w[-1].message))

    def test_get_env_key_priority(self):
        # ensure `get_env` use `DD_` with highest priority
        os.environ['DD_REQUESTS_DISTRIBUTED_TRACING'] = 'highest'
        os.environ['DATADOG_REQUESTS_DISTRIBUTED_TRACING'] = 'lowest'
        value = get_env('requests', 'distributed_tracing')
        self.assertEqual(value, 'highest')

    def test_deprecation_formatter(self):
        # ensure the formatter returns the proper message
        msg = format_message(
            'deprecated_function',
            'use something else instead',
            '1.0.0',
        )
        expected = (
            '\'deprecated_function\' is deprecated and will be remove in future versions (1.0.0). '
            'use something else instead'
        )
        self.assertEqual(msg, expected)

    def test_deprecation(self):
        # ensure `deprecation` properly raise a DeprecationWarning
        with warnings.catch_warnings(record=True) as w:
            warnings.simplefilter('always')
            deprecation(
                name='fn',
                message='message',
                version='1.0.0'
            )
            self.assertEqual(len(w), 1)
            self.assertTrue(issubclass(w[-1].category, DeprecationWarning))
            self.assertIn('message', str(w[-1].message))

    def test_deprecated_decorator(self):
        # ensure `deprecated` decorator properly raise a DeprecationWarning
        @deprecated('decorator', version='1.0.0')
        def fxn():
            pass

        with warnings.catch_warnings(record=True) as w:
            warnings.simplefilter('always')
            fxn()
            self.assertEqual(len(w), 1)
            self.assertTrue(issubclass(w[-1].category, DeprecationWarning))
            self.assertIn('decorator', str(w[-1].message))

    def test_flatten_dict(self):
        """ ensure that flattening of a nested dict results in a normalized, 1-level dict """
        d = dict(A=1, B=2, C=dict(A=3, B=4, C=dict(A=5, B=6)))
        e = dict(A=1, B=2, C_A=3, C_B=4, C_C_A=5, C_C_B=6)
        self.assertEquals(flatten_dict(d, sep='_'), e)
