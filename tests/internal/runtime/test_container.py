import mock

import pytest

from ddtrace.compat import PY2
from ddtrace.internal.runtime.container import CGroupInfo, get_container_info

from .utils import cgroup_line_valid_test_cases

# Map expected Py2 exception to Py3 name
if PY2:
    FileNotFoundError = IOError  # noqa: A001


def get_mock_open(read_data=None):
    mock_open = mock.mock_open(read_data=read_data)
    return mock.patch('ddtrace.internal.runtime.container.open', mock_open)


def test_cgroup_info_init():
    # Assert default all attributes to `None`
    info = CGroupInfo()
    for attr in ('id', 'groups', 'path', 'container_id', 'controllers', 'pod_id'):
        assert getattr(info, attr) is None

    # Assert init with property sets property
    info = CGroupInfo(container_id='test-container-id')
    assert info.container_id == 'test-container-id'


@pytest.mark.parametrize(
    'line,expected_info',

    # Valid generated cases + one off cases
    cgroup_line_valid_test_cases() + [
        # Valid, extra spaces
        (
            '    13:name=systemd:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860    ',
            CGroupInfo(
                id='13',
                groups='name=systemd',
                controllers=['name=systemd'],
                path='/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860',
                container_id='3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860',
                pod_id=None,
            ),
        ),
        # Valid, bookended newlines
        (
            '\r\n13:name=systemd:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860\r\n',
            CGroupInfo(
                id='13',
                groups='name=systemd',
                controllers=['name=systemd'],
                path='/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860',
                container_id='3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860',
                pod_id=None,
            ),
        ),

        # Invalid container_ids
        (
            # One character too short
            '13:name=systemd:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f86986',
            CGroupInfo(
                id='13',
                groups='name=systemd',
                controllers=['name=systemd'],
                path='/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f86986',
                container_id=None,
                pod_id=None,
            ),
        ),
        (
            # One character too long
            '13:name=systemd:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f8698600',
            CGroupInfo(
                id='13',
                groups='name=systemd',
                controllers=['name=systemd'],
                path='/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f8698600',
                container_id=None,
                pod_id=None,
            ),
        ),
        (
            # Non-hex
            '13:name=systemd:/docker/3726184226f5d3147c25fzyxw5b60097e378e8a720503a5e19ecfdf29f869860',
            CGroupInfo(
                id='13',
                groups='name=systemd',
                controllers=['name=systemd'],
                path='/docker/3726184226f5d3147c25fzyxw5b60097e378e8a720503a5e19ecfdf29f869860',
                container_id=None,
                pod_id=None,
            ),
        ),

        # Invalid id
        (
            # non-digit
            'a:name=systemd:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860',
            None,
        ),
        (
            # missing
            ':name=systemd:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860',
            None,
        ),

        # Missing group
        (
            # empty
            '13::/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860',
            CGroupInfo(
                id='13',
                groups='',
                controllers=[],
                path='/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860',
                container_id='3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860',
                pod_id=None,
            ),
        ),
        (
            # missing
            '13:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860',
            None,
        ),


        # Empty line
        (
            '',
            None,
        ),
    ],
)
def test_cgroup_info_from_line(line, expected_info):
    info = CGroupInfo.from_line(line)

    if expected_info is None:
        assert info is None, line
    else:
        for attr in ('id', 'groups', 'path', 'container_id', 'controllers', 'pod_id'):
            assert getattr(info, attr) == getattr(expected_info, attr), line


@pytest.mark.parametrize(
    'file_contents,container_id',
    (
        # Docker file
        (
            """
13:name=systemd:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860
12:pids:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860
11:hugetlb:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860
10:net_prio:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860
9:perf_event:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860
8:net_cls:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860
7:freezer:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860
6:devices:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860
5:memory:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860
4:blkio:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860
3:cpuacct:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860
2:cpu:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860
1:cpuset:/docker/3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860
            """,
            '3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860',
        ),

        # k8s file
        (
            """
11:perf_event:/kubepods/test/pod3d274242-8ee0-11e9-a8a6-1e68d864ef1a/3e74d3fd9db4c9dd921ae05c2502fb984d0cde1b36e581b13f79c639da4518a1
10:pids:/kubepods/test/pod3d274242-8ee0-11e9-a8a6-1e68d864ef1a/3e74d3fd9db4c9dd921ae05c2502fb984d0cde1b36e581b13f79c639da4518a1
9:memory:/kubepods/test/pod3d274242-8ee0-11e9-a8a6-1e68d864ef1a/3e74d3fd9db4c9dd921ae05c2502fb984d0cde1b36e581b13f79c639da4518a1
8:cpu,cpuacct:/kubepods/test/pod3d274242-8ee0-11e9-a8a6-1e68d864ef1a/3e74d3fd9db4c9dd921ae05c2502fb984d0cde1b36e581b13f79c639da4518a1
7:blkio:/kubepods/test/pod3d274242-8ee0-11e9-a8a6-1e68d864ef1a/3e74d3fd9db4c9dd921ae05c2502fb984d0cde1b36e581b13f79c639da4518a1
6:cpuset:/kubepods/test/pod3d274242-8ee0-11e9-a8a6-1e68d864ef1a/3e74d3fd9db4c9dd921ae05c2502fb984d0cde1b36e581b13f79c639da4518a1
5:devices:/kubepods/test/pod3d274242-8ee0-11e9-a8a6-1e68d864ef1a/3e74d3fd9db4c9dd921ae05c2502fb984d0cde1b36e581b13f79c639da4518a1
4:freezer:/kubepods/test/pod3d274242-8ee0-11e9-a8a6-1e68d864ef1a/3e74d3fd9db4c9dd921ae05c2502fb984d0cde1b36e581b13f79c639da4518a1
3:net_cls,net_prio:/kubepods/test/pod3d274242-8ee0-11e9-a8a6-1e68d864ef1a/3e74d3fd9db4c9dd921ae05c2502fb984d0cde1b36e581b13f79c639da4518a1
2:hugetlb:/kubepods/test/pod3d274242-8ee0-11e9-a8a6-1e68d864ef1a/3e74d3fd9db4c9dd921ae05c2502fb984d0cde1b36e581b13f79c639da4518a1
1:name=systemd:/kubepods/test/pod3d274242-8ee0-11e9-a8a6-1e68d864ef1a/3e74d3fd9db4c9dd921ae05c2502fb984d0cde1b36e581b13f79c639da4518a1
            """,
            '3e74d3fd9db4c9dd921ae05c2502fb984d0cde1b36e581b13f79c639da4518a1',
        ),

        # ECS file
        (
            """
9:perf_event:/ecs/test-ecs-classic/5a0d5ceddf6c44c1928d367a815d890f/38fac3e99302b3622be089dd41e7ccf38aff368a86cc339972075136ee2710ce
8:memory:/ecs/test-ecs-classic/5a0d5ceddf6c44c1928d367a815d890f/38fac3e99302b3622be089dd41e7ccf38aff368a86cc339972075136ee2710ce
7:hugetlb:/ecs/test-ecs-classic/5a0d5ceddf6c44c1928d367a815d890f/38fac3e99302b3622be089dd41e7ccf38aff368a86cc339972075136ee2710ce
6:freezer:/ecs/test-ecs-classic/5a0d5ceddf6c44c1928d367a815d890f/38fac3e99302b3622be089dd41e7ccf38aff368a86cc339972075136ee2710ce
5:devices:/ecs/test-ecs-classic/5a0d5ceddf6c44c1928d367a815d890f/38fac3e99302b3622be089dd41e7ccf38aff368a86cc339972075136ee2710ce
4:cpuset:/ecs/test-ecs-classic/5a0d5ceddf6c44c1928d367a815d890f/38fac3e99302b3622be089dd41e7ccf38aff368a86cc339972075136ee2710ce
3:cpuacct:/ecs/test-ecs-classic/5a0d5ceddf6c44c1928d367a815d890f/38fac3e99302b3622be089dd41e7ccf38aff368a86cc339972075136ee2710ce
2:cpu:/ecs/test-ecs-classic/5a0d5ceddf6c44c1928d367a815d890f/38fac3e99302b3622be089dd41e7ccf38aff368a86cc339972075136ee2710ce
1:blkio:/ecs/test-ecs-classic/5a0d5ceddf6c44c1928d367a815d890f/38fac3e99302b3622be089dd41e7ccf38aff368a86cc339972075136ee2710ce
            """,
            '38fac3e99302b3622be089dd41e7ccf38aff368a86cc339972075136ee2710ce',
        ),

        # Fargate file
        (
            """
11:hugetlb:/ecs/55091c13-b8cf-4801-b527-f4601742204d/432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da
10:pids:/ecs/55091c13-b8cf-4801-b527-f4601742204d/432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da
9:cpuset:/ecs/55091c13-b8cf-4801-b527-f4601742204d/432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da
8:net_cls,net_prio:/ecs/55091c13-b8cf-4801-b527-f4601742204d/432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da
7:cpu,cpuacct:/ecs/55091c13-b8cf-4801-b527-f4601742204d/432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da
6:perf_event:/ecs/55091c13-b8cf-4801-b527-f4601742204d/432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da
5:freezer:/ecs/55091c13-b8cf-4801-b527-f4601742204d/432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da
4:devices:/ecs/55091c13-b8cf-4801-b527-f4601742204d/432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da
3:blkio:/ecs/55091c13-b8cf-4801-b527-f4601742204d/432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da
2:memory:/ecs/55091c13-b8cf-4801-b527-f4601742204d/432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da
1:name=systemd:/ecs/55091c13-b8cf-4801-b527-f4601742204d/432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da
            """,
            '432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da',
        ),

        # Linux non-containerized file
        (
            """
11:blkio:/user.slice/user-0.slice/session-14.scope
10:memory:/user.slice/user-0.slice/session-14.scope
9:hugetlb:/
8:cpuset:/
7:pids:/user.slice/user-0.slice/session-14.scope
6:freezer:/
5:net_cls,net_prio:/
4:perf_event:/
3:cpu,cpuacct:/user.slice/user-0.slice/session-14.scope
2:devices:/user.slice/user-0.slice/session-14.scope
1:name=systemd:/user.slice/user-0.slice/session-14.scope
            """,
            None,
        ),

        # Empty file
        (
            '',
            None,
        ),

        # Missing file
        (
            None,
            None,
        )
    )
)
def test_get_container_info(file_contents, container_id):
    with get_mock_open(read_data=file_contents) as mock_open:
        # simulate the file not being found
        if file_contents is None:
            mock_open.side_effect = FileNotFoundError

        info = get_container_info()

        if container_id is None:
            assert info is None
        else:
            assert info.container_id == container_id

        mock_open.assert_called_once_with('/proc/self/cgroup', mode='r')


@pytest.mark.parametrize(
    'pid,file_name',
    (
        ('13', '/proc/13/cgroup'),
        (13, '/proc/13/cgroup'),
        ('self', '/proc/self/cgroup'),
    )
)
def test_get_container_info_with_pid(pid, file_name):
    # DEV: We need at least 1 line for the loop to call `CGroupInfo.from_line`
    with get_mock_open(read_data='\r\n') as mock_open:
        assert get_container_info(pid=pid) is None

        mock_open.assert_called_once_with(file_name, mode='r')


@mock.patch('ddtrace.internal.runtime.container.CGroupInfo.from_line')
@mock.patch('ddtrace.internal.runtime.container.log')
def test_get_container_info_exception(mock_log, mock_from_line):
    exception = Exception()
    mock_from_line.side_effect = exception

    # DEV: We need at least 1 line for the loop to call `CGroupInfo.from_line`
    with get_mock_open(read_data='\r\n') as mock_open:
        # Assert calling `get_container_info()` does not bubble up the exception
        assert get_container_info() is None

        # Assert we called everything we expected
        mock_from_line.assert_called_once_with('\r\n')
        mock_open.assert_called_once_with('/proc/self/cgroup', mode='r')

        # Ensure we logged the exception
        mock_log.debug.assert_called_once_with('Failed to parse cgroup file for pid %r', 'self', exc_info=True)
