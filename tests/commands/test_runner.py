import os
import subprocess
import sys

from ..base import BaseTestCase


def inject_sitecustomize(path):
    """Creates a new environment, injecting a ``sitecustomize.py`` module in
    the current PYTHONPATH.

    :param path: package path containing ``sitecustomize.py`` module, starting
                 from the ddtrace root folder
    :returns: a cloned environment that includes an altered PYTHONPATH with
              the given `sitecustomize.py`
    """
    from ddtrace import __file__ as root_file
    root_folder = os.path.dirname(root_file)
    # Copy the current environment and replace the PYTHONPATH. This is
    # required otherwise `ddtrace` scripts are not found when `env` kwarg is
    # passed
    env = os.environ.copy()
    sitecustomize = os.path.join(root_folder, '..', path)

    # Add `boostrap` module so that `sitecustomize.py` is at the bottom
    # of the PYTHONPATH
    python_path = list(sys.path) + [sitecustomize]
    env['PYTHONPATH'] = ':'.join(python_path)[1:]
    return env


class DdtraceRunTest(BaseTestCase):
    def test_service_name_passthrough(self):
        """
        $DATADOG_SERVICE_NAME gets passed through to the program
        """
        with self.override_env(dict(DATADOG_SERVICE_NAME='my_test_service')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_service.py']
            )
            assert out.startswith(b'Test success')

    def test_env_name_passthrough(self):
        """
        $DATADOG_ENV gets passed through to the global tracer as an 'env' tag
        """
        with self.override_env(dict(DATADOG_ENV='test')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_env.py']
            )
            assert out.startswith(b'Test success')

    def test_env_enabling(self):
        """
        DATADOG_TRACE_ENABLED=false allows disabling of the global tracer
        """
        with self.override_env(dict(DATADOG_TRACE_ENABLED='false')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_disabled.py']
            )
            assert out.startswith(b'Test success')

        with self.override_env(dict(DATADOG_TRACE_ENABLED='true')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_enabled.py']
            )
            assert out.startswith(b'Test success')

    def test_patched_modules(self):
        """
        Using `ddtrace-run` registers some generic patched modules
        """
        out = subprocess.check_output(
            ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_patched_modules.py']
        )
        assert out.startswith(b'Test success')

    def test_integration(self):
        out = subprocess.check_output(
            ['ddtrace-run', 'python', '-m', 'tests.commands.ddtrace_run_integration']
        )
        assert out.startswith(b'Test success')

    def test_debug_enabling(self):
        """
        DATADOG_TRACE_DEBUG=true allows setting debug logging of the global tracer
        """
        with self.override_env(dict(DATADOG_TRACE_DEBUG='false')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_no_debug.py']
            )
            assert out.startswith(b'Test success')

        with self.override_env(dict(DATADOG_TRACE_DEBUG='true')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_debug.py']
            )
            assert out.startswith(b'Test success')

    def test_host_port_from_env(self):
        """
        DATADOG_TRACE_AGENT_HOSTNAME|PORT point to the tracer
        to the correct host/port for submission
        """
        with self.override_env(dict(DATADOG_TRACE_AGENT_HOSTNAME='172.10.0.1',
                                    DATADOG_TRACE_AGENT_PORT='8120')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_hostname.py']
            )
            assert out.startswith(b'Test success')

    def test_host_port_from_env_dd(self):
        """
        DD_AGENT_HOST|DD_TRACE_AGENT_PORT point to the tracer
        to the correct host/port for submission
        """
        with self.override_env(dict(DD_AGENT_HOST='172.10.0.1',
                                    DD_TRACE_AGENT_PORT='8120')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_hostname.py']
            )
            assert out.startswith(b'Test success')

            # Do we get the same results without `ddtrace-run`?
            out = subprocess.check_output(
                ['python', 'tests/commands/ddtrace_run_hostname.py']
            )
            assert out.startswith(b'Test success')

    def test_dogstatsd_client_env_host_and_port(self):
        """
        DD_AGENT_HOST and DD_DOGSTATSD_PORT used to configure dogstatsd with udp in tracer
        """
        with self.override_env(dict(DD_AGENT_HOST='172.10.0.1',
                                    DD_DOGSTATSD_PORT='8120')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_dogstatsd.py']
            )
            assert out.startswith(b'Test success')

    def test_dogstatsd_client_env_url_host_and_port(self):
        """
        DD_DOGSTATSD_URL=<host>:<port> used to configure dogstatsd with udp in tracer
        """
        with self.override_env(dict(DD_DOGSTATSD_URL='172.10.0.1:8120')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_dogstatsd.py']
            )
            assert out.startswith(b'Test success')

    def test_dogstatsd_client_env_url_udp(self):
        """
        DD_DOGSTATSD_URL=udp://<host>:<port> used to configure dogstatsd with udp in tracer
        """
        with self.override_env(dict(DD_DOGSTATSD_URL='udp://172.10.0.1:8120')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_dogstatsd.py']
            )
            assert out.startswith(b'Test success')

    def test_dogstatsd_client_env_url_unix(self):
        """
        DD_DOGSTATSD_URL=unix://<path> used to configure dogstatsd with socket path in tracer
        """
        with self.override_env(dict(DD_DOGSTATSD_URL='unix:///dogstatsd.sock')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_dogstatsd.py']
            )
            assert out.startswith(b'Test success')

    def test_dogstatsd_client_env_url_path(self):
        """
        DD_DOGSTATSD_URL=<path> used to configure dogstatsd with socket path in tracer
        """
        with self.override_env(dict(DD_DOGSTATSD_URL='/dogstatsd.sock')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_dogstatsd.py']
            )
            assert out.startswith(b'Test success')

    def test_priority_sampling_from_env(self):
        """
        DATADOG_PRIORITY_SAMPLING enables Distributed Sampling
        """
        with self.override_env(dict(DATADOG_PRIORITY_SAMPLING='True')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_priority_sampling.py']
            )
            assert out.startswith(b'Test success')

    def test_patch_modules_from_env(self):
        """
        DATADOG_PATCH_MODULES overrides the defaults for patch_all()
        """
        from ddtrace.bootstrap.sitecustomize import EXTRA_PATCHED_MODULES, update_patched_modules
        orig = EXTRA_PATCHED_MODULES.copy()

        # empty / malformed strings are no-ops
        with self.override_env(dict(DATADOG_PATCH_MODULES='')):
            update_patched_modules()
            assert orig == EXTRA_PATCHED_MODULES

        with self.override_env(dict(DATADOG_PATCH_MODULES=':')):
            update_patched_modules()
            assert orig == EXTRA_PATCHED_MODULES

        with self.override_env(dict(DATADOG_PATCH_MODULES=',')):
            update_patched_modules()
            assert orig == EXTRA_PATCHED_MODULES

        with self.override_env(dict(DATADOG_PATCH_MODULES=',:')):
            update_patched_modules()
            assert orig == EXTRA_PATCHED_MODULES

        # overrides work in either direction
        with self.override_env(dict(DATADOG_PATCH_MODULES='django:false')):
            update_patched_modules()
            assert EXTRA_PATCHED_MODULES['django'] is False

        with self.override_env(dict(DATADOG_PATCH_MODULES='boto:true')):
            update_patched_modules()
            assert EXTRA_PATCHED_MODULES['boto'] is True

        with self.override_env(dict(DATADOG_PATCH_MODULES='django:true,boto:false')):
            update_patched_modules()
            assert EXTRA_PATCHED_MODULES['boto'] is False
            assert EXTRA_PATCHED_MODULES['django'] is True

        with self.override_env(dict(DATADOG_PATCH_MODULES='django:false,boto:true')):
            update_patched_modules()
            assert EXTRA_PATCHED_MODULES['boto'] is True
            assert EXTRA_PATCHED_MODULES['django'] is False

    def test_sitecustomize_without_ddtrace_run_command(self):
        # [Regression test]: ensure `sitecustomize` path is removed only if it's
        # present otherwise it will cause:
        #   ValueError: list.remove(x): x not in list
        # as mentioned here: https://github.com/DataDog/dd-trace-py/pull/516
        env = inject_sitecustomize('')
        out = subprocess.check_output(
            ['python', 'tests/commands/ddtrace_minimal.py'],
            env=env,
        )
        # `out` contains the `loaded` status of the module
        result = out[:-1] == b'True'
        self.assertTrue(result)

    def test_sitecustomize_run(self):
        # [Regression test]: ensure users `sitecustomize.py` is properly loaded,
        # so that our `bootstrap/sitecustomize.py` doesn't override the one
        # defined in users' PYTHONPATH.
        env = inject_sitecustomize('tests/commands/bootstrap')
        out = subprocess.check_output(
            ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_sitecustomize.py'],
            env=env,
        )
        assert out.startswith(b'Test success')

    def test_sitecustomize_run_suppressed(self):
        # ensure `sitecustomize.py` is not loaded if `-S` is used
        env = inject_sitecustomize('tests/commands/bootstrap')
        out = subprocess.check_output(
            ['ddtrace-run', 'python', '-S', 'tests/commands/ddtrace_run_sitecustomize.py', '-S'],
            env=env,
        )
        assert out.startswith(b'Test success')

    def test_argv_passed(self):
        out = subprocess.check_output(
            ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_argv.py', 'foo', 'bar']
        )
        assert out.startswith(b'Test success')

    def test_got_app_name(self):
        """
        apps run with ddtrace-run have a proper app name
        """
        out = subprocess.check_output(
            ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_app_name.py']
        )
        assert out.startswith(b'ddtrace_run_app_name.py')

    def test_global_trace_tags(self):
        """ Ensure global tags are passed in from environment
        """
        with self.override_env(dict(DD_TRACE_GLOBAL_TAGS='a:True,b:0,c:C')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_global_tags.py']
            )
            assert out.startswith(b'Test success')

    def test_logs_injection(self):
        """ Ensure logs injection works
        """
        with self.override_env(dict(DD_LOGS_INJECTION='true')):
            out = subprocess.check_output(
                ['ddtrace-run', 'python', 'tests/commands/ddtrace_run_logs_injection.py']
            )
            assert out.startswith(b'Test success')
