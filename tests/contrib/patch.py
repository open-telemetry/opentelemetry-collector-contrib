import functools
import importlib
import sys
import unittest

from ddtrace.vendor import wrapt

from tests.subprocesstest import SubprocessTestCase, run_in_subprocess


class PatchMixin(unittest.TestCase):
    """
    TestCase for testing the patch logic of an integration.
    """
    def module_imported(self, modname):
        """
        Returns whether a module is imported or not.
        """
        return modname in sys.modules

    def assert_module_imported(self, modname):
        """
        Asserts that the module, given its name is imported.
        """
        assert self.module_imported(modname), '{} module not imported'.format(modname)

    def assert_not_module_imported(self, modname):
        """
        Asserts that the module, given its name is not imported.
        """
        assert not self.module_imported(modname), '{} module is imported'.format(modname)

    def is_wrapped(self, obj):
        return isinstance(obj, wrapt.ObjectProxy)

    def assert_wrapped(self, obj):
        """
        Helper to assert that a given object is properly wrapped by wrapt.
        """
        self.assertTrue(self.is_wrapped(obj), '{} is not wrapped'.format(obj))

    def assert_not_wrapped(self, obj):
        """
        Helper to assert that a given object is not wrapped by wrapt.
        """
        self.assertFalse(self.is_wrapped(obj), '{} is wrapped'.format(obj))

    def assert_not_double_wrapped(self, obj):
        """
        Helper to assert that a given already wrapped object is not wrapped twice.

        This is useful for asserting idempotence.
        """
        self.assert_wrapped(obj)
        self.assert_not_wrapped(obj.__wrapped__)


def raise_if_no_attrs(f):
    """
    A helper for PatchTestCase test methods that will check if there are any
    modules to use else raise a NotImplementedError.

    :param f: method to wrap with a check
    """
    required_attrs = [
        '__module_name__',
        '__integration_name__',
        '__unpatch_func__',
    ]

    @functools.wraps(f)
    def checked_method(self, *args, **kwargs):
        for attr in required_attrs:
            if not getattr(self, attr):
                raise NotImplementedError(f.__doc__)
        return f(self, *args, **kwargs)
    return checked_method


class PatchTestCase(object):
    """
    unittest or other test runners will pick up the base test case as a testcase
    since it inherits from unittest.TestCase unless we wrap it with this empty
    parent class.
    """
    @run_in_subprocess
    class Base(SubprocessTestCase, PatchMixin):
        """Provides default test methods to be used for testing common integration patching logic.
        Each test method provides a default implementation which will use the
        provided attributes (described below). If the attributes are not
        provided a NotImplementedError will be raised for each method that is
        not overridden.

        Attributes:
            __integration_name__ the name of the integration.
            __module_name__ module which the integration patches.
            __unpatch_func__ unpatch function from the integration.

        Example:
        A simple implementation inheriting this TestCase looks like::

            from ddtrace.contrib.redis import unpatch

            class RedisPatchTestCase(PatchTestCase.Base):
                __integration_name__ = 'redis'
                __module_name__ 'redis'
                __unpatch_func__ = unpatch

                def assert_module_patched(self, redis):
                    # assert patching logic
                    # self.assert_wrapped(...)

                def assert_not_module_patched(self, redis):
                    # assert patching logic
                    # self.assert_not_wrapped(...)

                def assert_not_module_double_patched(self, redis):
                    # assert patching logic
                    # self.assert_not_double_wrapped(...)

                # override this particular test case
                def test_patch_import(self):
                    # custom patch before import check

                # optionally override other test methods...
        """
        __integration_name__ = None
        __module_name__ = None
        __unpatch_func__ = None

        def __init__(self, *args, **kwargs):
            # DEV: Python will wrap a function when assigning to a class as an
            # attribute. So we cannot call self.__unpatch_func__() as the `self`
            # reference will be passed as an argument.
            # So we need to unwrap the function and then wrap it in a function
            # that will absorb the unpatch function.
            if self.__unpatch_func__:
                unpatch_func = self.__unpatch_func__.__func__

                def unpatch():
                    unpatch_func()
                self.__unpatch_func__ = unpatch
            super(PatchTestCase.Base, self).__init__(*args, **kwargs)

        def patch(self, *args, **kwargs):
            from ddtrace import patch
            return patch(*args, **kwargs)

        def _gen_test_attrs(self, ops):
            """
            A helper to return test names for tests given a list of different
            operations.
            :return:
            """
            from itertools import permutations
            return [
                'test_{}'.format('_'.join(c)) for c in permutations(ops, len(ops))
            ]

        def test_verify_test_coverage(self):
            """
            This TestCase should cover a variety of combinations of importing,
            patching and unpatching.
            """
            tests = []
            tests += self._gen_test_attrs(['import', 'patch'])
            tests += self._gen_test_attrs(['import', 'patch', 'patch'])
            tests += self._gen_test_attrs(['import', 'patch', 'unpatch'])
            tests += self._gen_test_attrs(['import', 'patch', 'unpatch', 'unpatch'])

            # TODO: it may be possible to generate test cases dynamically. For
            # now focus on the important ones.
            test_ignore = set([
                'test_unpatch_import_patch',
                'test_import_unpatch_patch_unpatch',
                'test_import_unpatch_unpatch_patch',
                'test_patch_import_unpatch_unpatch',
                'test_unpatch_import_patch_unpatch',
                'test_unpatch_import_unpatch_patch',
                'test_unpatch_patch_import_unpatch',
                'test_unpatch_patch_unpatch_import',
                'test_unpatch_unpatch_import_patch',
                'test_unpatch_unpatch_patch_import',
            ])

            for test_attr in tests:
                if test_attr in test_ignore:
                    continue
                assert hasattr(self, test_attr), '{} not found in expected test attrs'.format(test_attr)

        def assert_module_patched(self, module):
            """
            Asserts that the given module is patched.

            For example, the redis integration patches the following methods:
                - redis.StrictRedis.execute_command
                - redis.StrictRedis.pipeline
                - redis.Redis.pipeline
                - redis.client.BasePipeline.execute
                - redis.client.BasePipeline.immediate_execute_command

            So an appropriate assert_module_patched would look like::

                def assert_module_patched(self, redis):
                    self.assert_wrapped(redis.StrictRedis.execute_command)
                    self.assert_wrapped(redis.StrictRedis.pipeline)
                    self.assert_wrapped(redis.Redis.pipeline)
                    self.assert_wrapped(redis.client.BasePipeline.execute)
                    self.assert_wrapped(redis.client.BasePipeline.immediate_execute_command)

            :param module: module to check
            :return: None
            """
            raise NotImplementedError(self.assert_module_patched.__doc__)

        def assert_not_module_patched(self, module):
            """
            Asserts that the given module is not patched.

            For example, the redis integration patches the following methods:
                - redis.StrictRedis.execute_command
                - redis.StrictRedis.pipeline
                - redis.Redis.pipeline
                - redis.client.BasePipeline.execute
                - redis.client.BasePipeline.immediate_execute_command

            So an appropriate assert_not_module_patched would look like::

                def assert_not_module_patched(self, redis):
                    self.assert_not_wrapped(redis.StrictRedis.execute_command)
                    self.assert_not_wrapped(redis.StrictRedis.pipeline)
                    self.assert_not_wrapped(redis.Redis.pipeline)
                    self.assert_not_wrapped(redis.client.BasePipeline.execute)
                    self.assert_not_wrapped(redis.client.BasePipeline.immediate_execute_command)

            :param module:
            :return: None
            """
            raise NotImplementedError(self.assert_not_module_patched.__doc__)

        def assert_not_module_double_patched(self, module):
            """
            Asserts that the given module is not patched twice.

            For example, the redis integration patches the following methods:
                - redis.StrictRedis.execute_command
                - redis.StrictRedis.pipeline
                - redis.Redis.pipeline
                - redis.client.BasePipeline.execute
                - redis.client.BasePipeline.immediate_execute_command

            So an appropriate assert_not_module_double_patched would look like::

                def assert_not_module_double_patched(self, redis):
                    self.assert_not_double_wrapped(redis.StrictRedis.execute_command)
                    self.assert_not_double_wrapped(redis.StrictRedis.pipeline)
                    self.assert_not_double_wrapped(redis.Redis.pipeline)
                    self.assert_not_double_wrapped(redis.client.BasePipeline.execute)
                    self.assert_not_double_wrapped(redis.client.BasePipeline.immediate_execute_command)

            :param module: module to check
            :return: None
            """
            raise NotImplementedError(self.assert_not_module_double_patched.__doc__)

        @raise_if_no_attrs
        def test_import_patch(self):
            """
            The integration should test that each class, method or function that
            is to be patched is in fact done so when ddtrace.patch() is called
            before the module is imported.

            For example:

            an appropriate ``test_patch_import`` would be::

                import redis
                ddtrace.patch(redis=True)
                self.assert_module_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            module = importlib.import_module(self.__module_name__)
            self.assert_not_module_patched(module)
            self.patch(**{self.__integration_name__: True})
            self.assert_module_patched(module)

        @raise_if_no_attrs
        def test_patch_import(self):
            """
            The integration should test that each class, method or function that
            is to be patched is in fact done so when ddtrace.patch() is called
            after the module is imported.

            an appropriate ``test_patch_import`` would be::

                import redis
                ddtrace.patch(redis=True)
                self.assert_module_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            module = importlib.import_module(self.__module_name__)
            self.patch(**{self.__integration_name__: True})
            self.assert_module_patched(module)

        @raise_if_no_attrs
        def test_import_patch_patch(self):
            """
            Proper testing should be done to ensure that multiple calls to the
            integration.patch() method are idempotent. That is, that the
            integration does not patch its library more than once.

            An example for what this might look like for the redis integration::

                import redis
                ddtrace.patch(redis=True)
                self.assert_module_patched(redis)
                ddtrace.patch(redis=True)
                self.assert_not_module_double_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            self.patch(**{self.__module_name__: True})
            module = importlib.import_module(self.__module_name__)
            self.assert_module_patched(module)
            self.patch(**{self.__module_name__: True})
            self.assert_not_module_double_patched(module)

        @raise_if_no_attrs
        def test_patch_import_patch(self):
            """
            Proper testing should be done to ensure that multiple calls to the
            integration.patch() method are idempotent. That is, that the
            integration does not patch its library more than once.

            An example for what this might look like for the redis integration::

                ddtrace.patch(redis=True)
                import redis
                self.assert_module_patched(redis)
                ddtrace.patch(redis=True)
                self.assert_not_module_double_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            self.patch(**{self.__module_name__: True})
            module = importlib.import_module(self.__module_name__)
            self.assert_module_patched(module)
            self.patch(**{self.__module_name__: True})
            self.assert_not_module_double_patched(module)

        @raise_if_no_attrs
        def test_patch_patch_import(self):
            """
            Proper testing should be done to ensure that multiple calls to the
            integration.patch() method are idempotent. That is, that the
            integration does not patch its library more than once.

            An example for what this might look like for the redis integration::

                ddtrace.patch(redis=True)
                ddtrace.patch(redis=True)
                import redis
                self.assert_not_double_wrapped(redis.StrictRedis.execute_command)
            """
            self.assert_not_module_imported(self.__module_name__)
            self.patch(**{self.__module_name__: True})
            self.patch(**{self.__module_name__: True})
            module = importlib.import_module(self.__module_name__)
            self.assert_module_patched(module)
            self.assert_not_module_double_patched(module)

        @raise_if_no_attrs
        def test_import_patch_unpatch_patch(self):
            """
            To ensure that we can thoroughly test the installation/patching of
            an integration we must be able to unpatch it and then subsequently
            patch it again.

            For example::

                import redis
                from ddtrace.contrib.redis import unpatch

                ddtrace.patch(redis=True)
                unpatch()
                ddtrace.patch(redis=True)
                self.assert_module_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            module = importlib.import_module(self.__module_name__)
            self.patch(**{self.__integration_name__: True})
            self.assert_module_patched(module)
            self.__unpatch_func__()
            self.assert_not_module_patched(module)
            self.patch(**{self.__integration_name__: True})
            self.assert_module_patched(module)

        @raise_if_no_attrs
        def test_patch_import_unpatch_patch(self):
            """
            To ensure that we can thoroughly test the installation/patching of
            an integration we must be able to unpatch it and then subsequently
            patch it again.

            For example::

                from ddtrace.contrib.redis import unpatch

                ddtrace.patch(redis=True)
                import redis
                unpatch()
                ddtrace.patch(redis=True)
                self.assert_module_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            self.patch(**{self.__integration_name__: True})
            module = importlib.import_module(self.__module_name__)
            self.assert_module_patched(module)
            self.__unpatch_func__()
            self.assert_not_module_patched(module)
            self.patch(**{self.__integration_name__: True})
            self.assert_module_patched(module)

        @raise_if_no_attrs
        def test_patch_unpatch_import_patch(self):
            """
            To ensure that we can thoroughly test the installation/patching of
            an integration we must be able to unpatch it and then subsequently
            patch it again.

            For example::

                from ddtrace.contrib.redis import unpatch

                ddtrace.patch(redis=True)
                import redis
                unpatch()
                ddtrace.patch(redis=True)
                self.assert_module_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            self.patch(**{self.__integration_name__: True})
            self.__unpatch_func__()
            module = importlib.import_module(self.__module_name__)
            self.assert_not_module_patched(module)
            self.patch(**{self.__integration_name__: True})
            self.assert_module_patched(module)

        @raise_if_no_attrs
        def test_patch_unpatch_patch_import(self):
            """
            To ensure that we can thoroughly test the installation/patching of
            an integration we must be able to unpatch it and then subsequently
            patch it again.

            For example::

                from ddtrace.contrib.redis import unpatch

                ddtrace.patch(redis=True)
                unpatch()
                ddtrace.patch(redis=True)
                import redis
                self.assert_module_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            self.patch(**{self.__integration_name__: True})
            self.__unpatch_func__()
            self.patch(**{self.__integration_name__: True})
            module = importlib.import_module(self.__module_name__)
            self.assert_module_patched(module)

        @raise_if_no_attrs
        def test_unpatch_patch_import(self):
            """
            Make sure unpatching before patch does not break patching.

            For example::

                from ddtrace.contrib.redis import unpatch
                unpatch()
                ddtrace.patch(redis=True)
                import redis
                self.assert_not_module_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            self.__unpatch_func__()
            self.patch(**{self.__integration_name__: True})
            module = importlib.import_module(self.__module_name__)
            self.assert_module_patched(module)

        @raise_if_no_attrs
        def test_patch_unpatch_import(self):
            """
            To ensure that we can thoroughly test the installation/patching of
            an integration we must be able to unpatch it before importing the
            library.

            For example::

                ddtrace.patch(redis=True)
                from ddtrace.contrib.redis import unpatch
                unpatch()
                import redis
                self.assert_not_module_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            self.patch(**{self.__integration_name__: True})
            self.__unpatch_func__()
            module = importlib.import_module(self.__module_name__)
            self.assert_not_module_patched(module)

        @raise_if_no_attrs
        def test_import_unpatch_patch(self):
            """
            To ensure that we can thoroughly test the installation/patching of
            an integration we must be able to unpatch it before patching.

            For example::

                import redis
                from ddtrace.contrib.redis import unpatch
                ddtrace.patch(redis=True)
                unpatch()
                self.assert_not_module_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            module = importlib.import_module(self.__module_name__)
            self.__unpatch_func__()
            self.assert_not_module_patched(module)
            self.patch(**{self.__integration_name__: True})
            self.assert_module_patched(module)

        @raise_if_no_attrs
        def test_import_patch_unpatch(self):
            """
            To ensure that we can thoroughly test the installation/patching of
            an integration we must be able to unpatch it after patching.

            For example::

                import redis
                from ddtrace.contrib.redis import unpatch
                ddtrace.patch(redis=True)
                unpatch()
                self.assert_not_module_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            module = importlib.import_module(self.__module_name__)
            self.assert_not_module_patched(module)
            self.patch(**{self.__integration_name__: True})
            self.assert_module_patched(module)
            self.__unpatch_func__()
            self.assert_not_module_patched(module)

        @raise_if_no_attrs
        def test_patch_import_unpatch(self):
            """
            To ensure that we can thoroughly test the installation/patching of
            an integration we must be able to unpatch it after patching.

            For example::

                from ddtrace.contrib.redis import unpatch
                ddtrace.patch(redis=True)
                import redis
                unpatch()
                self.assert_not_module_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            self.patch(**{self.__integration_name__: True})
            module = importlib.import_module(self.__module_name__)
            self.assert_module_patched(module)
            self.__unpatch_func__()
            self.assert_not_module_patched(module)

        @raise_if_no_attrs
        def test_import_patch_unpatch_unpatch(self):
            """
            Unpatching twice should be a no-op.

            For example::

                import redis
                from ddtrace.contrib.redis import unpatch

                ddtrace.patch(redis=True)
                self.assert_module_patched(redis)
                unpatch()
                self.assert_not_module_patched(redis)
                unpatch()
                self.assert_not_module_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            module = importlib.import_module(self.__module_name__)
            self.patch(**{self.__integration_name__: True})
            self.assert_module_patched(module)
            self.__unpatch_func__()
            self.assert_not_module_patched(module)
            self.__unpatch_func__()
            self.assert_not_module_patched(module)

        @raise_if_no_attrs
        def test_patch_unpatch_import_unpatch(self):
            """
            Unpatching twice should be a no-op.

            For example::

                from ddtrace.contrib.redis import unpatch

                ddtrace.patch(redis=True)
                unpatch()
                import redis
                self.assert_not_module_patched(redis)
                unpatch()
                self.assert_not_module_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            self.patch(**{self.__integration_name__: True})
            self.__unpatch_func__()
            module = importlib.import_module(self.__module_name__)
            self.assert_not_module_patched(module)
            self.__unpatch_func__()
            self.assert_not_module_patched(module)

        @raise_if_no_attrs
        def test_patch_unpatch_unpatch_import(self):
            """
            Unpatching twice should be a no-op.

            For example::

                from ddtrace.contrib.redis import unpatch

                ddtrace.patch(redis=True)
                unpatch()
                unpatch()
                import redis
                self.assert_not_module_patched(redis)
            """
            self.assert_not_module_imported(self.__module_name__)
            self.patch(**{self.__integration_name__: True})
            self.__unpatch_func__()
            self.__unpatch_func__()
            module = importlib.import_module(self.__module_name__)
            self.assert_not_module_patched(module)
