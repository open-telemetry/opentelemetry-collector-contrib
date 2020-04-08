"""
subprocesstest enables unittest test cases and suites to be run in separate
python interpreter instances.

A base class SubprocessTestCase is provided that, when extended, will run test
cases marked with @run_in_subprocess in a separate python interpreter.
"""
import os
import subprocess
import sys
import unittest


SUBPROC_TEST_ATTR = '_subproc_test'
SUBPROC_ENV_VAR = 'SUBPROCESS_TEST'


def run_in_subprocess(obj):
    """
    Marks a test case that is to be run in its own 'clean' interpreter instance.

    When applied to a TestCase class, each method will be run in a separate
    interpreter instance.

    Usage on a class::

        from tests.subprocesstest import SubprocessTestCase, run_in_subprocess

        @run_in_subprocess
        class PatchTests(SubprocessTestCase):
            # will be run in new interpreter
            def test_patch_before_import(self):
                patch()
                import module

            # will be run in new interpreter as well
            def test_patch_after_import(self):
                import module
                patch()


    Usage on a test method::

        class OtherTests(SubprocessTestCase):
            @run_in_subprocess
            def test_case(self):
                pass


    :param obj: method or class to run in a separate python interpreter.
    :return:
    """
    setattr(obj, SUBPROC_TEST_ATTR, True)
    return obj


class SubprocessTestCase(unittest.TestCase):
    def _full_method_name(self):
        test = getattr(self, self._testMethodName)
        # DEV: we have to use the internal self reference of the bound test
        # method to pull out the class and module since using a mix of `self`
        # and the test attributes will result in inconsistencies when the test
        # method is defined on another class.
        # A concrete case of this is a parent and child TestCase where the child
        # doesn't override a parent test method. The full_method_name we want
        # is that of the child test method (even though it exists on the parent)
        modpath = test.__self__.__class__.__module__
        clsname = test.__self__.__class__.__name__
        testname = test.__name__
        testcase_name = '{}.{}.{}'.format(modpath, clsname, testname)
        return testcase_name

    def _run_test_in_subprocess(self, result):
        full_testcase_name = self._full_method_name()

        # copy the environment and include the special subprocess environment
        # variable for the subprocess to detect
        sp_test_env = os.environ.copy()
        sp_test_env[SUBPROC_ENV_VAR] = 'True'
        sp_test_cmd = ['python', '-m', 'unittest', full_testcase_name]
        sp = subprocess.Popen(
            sp_test_cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env=sp_test_env,
        )
        stdout, stderr = sp.communicate()

        if sp.returncode:
            try:
                cmdf = ' '.join(sp_test_cmd)
                raise Exception('Subprocess Test "{}" Failed'.format(cmdf))
            except Exception:
                exc_info = sys.exc_info()

            # DEV: stderr, stdout are byte sequences so to print them nicely
            #      back out they should be decoded.
            sys.stderr.write(stderr.decode())
            sys.stdout.write(stdout.decode())
            result.addFailure(self, exc_info)
        else:
            result.addSuccess(self)

    def _in_subprocess(self):
        """Determines if the test is being run in a subprocess.

        This is done by checking for an environment variable that we call the
        subprocess test with.

        :return: whether the test is a subprocess test
        """
        return os.getenv(SUBPROC_ENV_VAR, None) is not None

    def _is_subprocess_test(self):
        if hasattr(self, SUBPROC_TEST_ATTR):
            return True

        test = getattr(self, self._testMethodName)
        if hasattr(test, SUBPROC_TEST_ATTR):
            return True

        return False

    def run(self, result=None):
        if not self._is_subprocess_test():
            return super(SubprocessTestCase, self).run(result=result)

        if self._in_subprocess():
            return super(SubprocessTestCase, self).run(result=result)
        else:
            self._run_test_in_subprocess(result)
