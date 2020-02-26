import mock
from ddtrace.internal.runtime.collector import ValueCollector

from ...base import BaseTestCase


def mocked_collector(mock_collect, **kwargs):
    collector = ValueCollector(**kwargs)
    collector.collect_fn = mock_collect
    return collector


class TestValueCollector(BaseTestCase):

    def test_default_usage(self):
        mock_collect = mock.MagicMock()
        mock_collect.side_effect = lambda k: [
            ('key1', 'value1'),
            ('key2', 'value2'),
        ]

        vc = mocked_collector(mock_collect)

        self.assertEqual(vc.collect(keys=set(['key1'])), [
            ('key1', 'value1'),
        ])
        mock_collect.assert_called_once()
        mock_collect.assert_called_with(set(['key1']))

        self.assertEqual(mock_collect.call_count, 1,
                         'Collector is not periodic by default')

    def test_enabled(self):
        collect = mock.MagicMock()
        vc = mocked_collector(collect, enabled=False)
        collect.assert_not_called()
        vc.collect()
        collect.assert_not_called()

    def test_periodic(self):
        collect = mock.MagicMock()
        vc = mocked_collector(collect, periodic=True)
        vc.collect()
        self.assertEqual(collect.call_count, 1)
        vc.collect()
        self.assertEqual(collect.call_count, 2)

    def test_not_periodic(self):
        collect = mock.MagicMock()
        vc = mocked_collector(collect)
        collect.assert_not_called()
        vc.collect()
        self.assertEqual(collect.call_count, 1)
        vc.collect()
        self.assertEqual(collect.call_count, 1)
        vc.collect()
        self.assertEqual(collect.call_count, 1)

    def test_required_module(self):
        mock_module = mock.MagicMock()
        mock_module.fn.side_effect = lambda: 'test'
        with self.override_sys_modules(dict(A=mock_module)):
            class AVC(ValueCollector):
                required_modules = ['A']

                def collect_fn(self, keys):
                    a = self.modules.get('A')
                    a.fn()

            vc = AVC()
            vc.collect()
            mock_module.fn.assert_called_once()

    def test_required_module_not_installed(self):
        collect = mock.MagicMock()
        with mock.patch('ddtrace.internal.runtime.collector.log') as log_mock:
            # Should log a warning (tested below)
            vc = mocked_collector(collect, required_modules=['moduleshouldnotexist'])

            # Collect should not be called as the collector should be disabled.
            collect.assert_not_called()
            vc.collect()
            collect.assert_not_called()

        calls = [
            mock.call(
                'Could not import module "%s" for %s. Disabling collector.',
                'moduleshouldnotexist', vc,
            )
        ]
        log_mock.warning.assert_has_calls(calls)

    def test_collected_values(self):
        class V(ValueCollector):
            i = 0

            def collect_fn(self, keys):
                self.i += 1
                return [('i', self.i)]

        vc = V()
        self.assertEqual(vc.collect(), [('i', 1)])
        self.assertEqual(vc.collect(), [('i', 1)])
        self.assertEqual(vc.collect(), [('i', 1)])

    def test_collected_values_periodic(self):
        class V(ValueCollector):
            periodic = True
            i = 0

            def collect_fn(self, keys):
                self.i += 1
                return [('i', self.i)]

        vc = V()
        self.assertEqual(vc.collect(), [('i', 1)])
        self.assertEqual(vc.collect(), [('i', 2)])
        self.assertEqual(vc.collect(), [('i', 3)])
