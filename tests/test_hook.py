import mock

from ddtrace.compat import reload_module
from ddtrace.utils.hook import (
    register_post_import_hook,
    deregister_post_import_hook,
)

from tests.subprocesstest import SubprocessTestCase, run_in_subprocess


@run_in_subprocess
class TestHook(SubprocessTestCase):
    def test_register_post_import_hook_before_import(self):
        """
        Test that a hook is fired after registering.
        """
        test_hook = mock.MagicMock()
        register_post_import_hook('tests.utils.test_module', test_hook)
        import tests.utils.test_module  # noqa
        test_hook.assert_called_once()

    def test_register_post_import_hook_after_import(self):
        """
        Test that a hook is fired when the module is imported with an
        appropriate log debug message.
        """
        test_hook = mock.MagicMock()
        with mock.patch('ddtrace.utils.hook.log') as log_mock:
            import tests.utils.test_module  # noqa
            register_post_import_hook('tests.utils.test_module', test_hook)
            test_hook.assert_called_once()
            calls = [
                mock.call('module "%s" already imported, firing hook', "tests.utils.test_module")
            ]
            log_mock.debug.assert_has_calls(calls)

    def test_register_post_import_hook_reimport(self):
        """
        Test that a hook is fired when the module is reimported.
        """
        test_hook = mock.MagicMock()
        register_post_import_hook('tests.utils.test_module', test_hook)
        import tests.utils.test_module
        reload_module(tests.utils.test_module)
        self.assertEqual(test_hook.call_count, 2)

    def test_register_post_import_hook_multiple(self):
        """
        Test that multiple hooks are fired after registering.
        """
        test_hook = mock.MagicMock()
        test_hook2 = mock.MagicMock()
        register_post_import_hook('tests.utils.test_module', test_hook)
        register_post_import_hook('tests.utils.test_module', test_hook2)
        import tests.utils.test_module  # noqa
        test_hook.assert_called_once()
        test_hook2.assert_called_once()

    def test_register_post_import_hook_different_modules(self):
        """
        Test that multiple hooks hooked on different modules are fired after registering.
        """
        test_hook = mock.MagicMock()
        test_hook_redis = mock.MagicMock()
        register_post_import_hook('tests.utils.test_module', test_hook)
        register_post_import_hook('ddtrace.contrib.redis', test_hook_redis)
        import tests.utils.test_module  # noqa
        import ddtrace.contrib.redis  # noqa
        test_hook.assert_called_once()
        test_hook_redis.assert_called_once()

    def test_register_post_import_hook_duplicate_register(self):
        """
        Test that a function can be registered as a hook twice.
        """
        test_hook = mock.MagicMock()
        with mock.patch('ddtrace.utils.hook.log') as log_mock:
            register_post_import_hook('tests.utils.test_module', test_hook)
            register_post_import_hook('tests.utils.test_module', test_hook)
            import tests.utils.test_module  # noqa

            self.assertEqual(log_mock.debug.mock_calls, [
                mock.call('hook "%s" already exists on module "%s"', test_hook, 'tests.utils.test_module'),
            ])

    def test_deregister_post_import_hook_no_register(self):
        """
        Test that deregistering import hooks that do not exist is a no-op.
        """
        def hook():
            return

        outcome = deregister_post_import_hook('tests.utils.test_module', hook)
        self.assertFalse(outcome)
        import tests.utils.test_module  # noqa

    def test_deregister_post_import_hook_after_register(self):
        """
        Test that import hooks can be deregistered after being registered.
        """
        test_hook = mock.MagicMock()
        register_post_import_hook('tests.utils.test_module', test_hook)
        outcome = deregister_post_import_hook('tests.utils.test_module', test_hook)
        self.assertTrue(outcome)
        import tests.utils.test_module  # noqa
        self.assertEqual(test_hook.call_count, 0, 'hook has been deregistered and should have been removed')

    def test_deregister_post_import_hook_after_register_multiple_all(self):
        """
        Test that multiple import hooks can be deregistered.
        """
        test_hook = mock.MagicMock()
        test_hook2 = mock.MagicMock()
        register_post_import_hook('tests.utils.test_module', test_hook)
        register_post_import_hook('tests.utils.test_module', test_hook2)

        outcome = deregister_post_import_hook('tests.utils.test_module', test_hook)
        self.assertTrue(outcome)
        outcome = deregister_post_import_hook('tests.utils.test_module', test_hook2)
        self.assertTrue(outcome)
        import tests.utils.test_module  # noqa
        self.assertEqual(test_hook.call_count, 0, 'hook has been deregistered and should be removed')
        self.assertEqual(test_hook2.call_count, 0, 'hook has been deregistered and should be removed')

    def test_deregister_post_import_hook_after_register_multiple(self):
        """
        Test that only the specified import hook can be deregistered after being registered.
        """
        # Enforce a spec so that hasattr doesn't vacuously return True.
        test_hook = mock.MagicMock(spec=[])
        test_hook2 = mock.MagicMock(spec=[])
        register_post_import_hook('tests.utils.test_module', test_hook)
        register_post_import_hook('tests.utils.test_module', test_hook2)

        outcome = deregister_post_import_hook('tests.utils.test_module', test_hook)
        self.assertTrue(outcome)
        import tests.utils.test_module  # noqa
        self.assertEqual(test_hook.call_count, 0, 'hook has been deregistered and should be removed')
        self.assertEqual(test_hook2.call_count, 1, 'hook should have been called')

    def test_deregister_post_import_hook_after_import(self):
        """
        Test that import hooks can be deregistered after being registered.
        """
        test_hook = mock.MagicMock()
        register_post_import_hook('tests.utils.test_module', test_hook)

        import tests.utils.test_module
        test_hook.assert_called_once()
        outcome = deregister_post_import_hook('tests.utils.test_module', test_hook)
        self.assertTrue(outcome)
        reload_module(tests.utils.test_module)
        self.assertEqual(test_hook.call_count, 1, 'hook should only be called once')

    def test_hook_exception(self):
        """
        Test that when a hook throws an exception that it is caught and logged
        as a warning.
        """
        def test_hook(module):
            raise Exception('test_hook_failed')
        register_post_import_hook('tests.utils.test_module', test_hook)

        with mock.patch('ddtrace.utils.hook.log') as log_mock:
            import tests.utils.test_module  # noqa
            calls = [
                mock.call('hook "%s" for module "%s" failed',
                          test_hook, 'tests.utils.test_module', exc_info=True)
            ]
            log_mock.warning.assert_has_calls(calls)

    def test_hook_called_with_module(self):
        """
        Test that a hook is called with the module that it is hooked on.
        """
        def test_hook(module):
            self.assertTrue(hasattr(module, 'A'))
        register_post_import_hook('tests.utils.test_module', test_hook)
        import tests.utils.test_module  # noqa
