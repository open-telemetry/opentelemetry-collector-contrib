import itertools

from ddtrace.internal.runtime.container import CGroupInfo


def cgroup_line_valid_test_cases():
    controllers = [
        ['name=systemd'],
        ['pids'],
        ['cpu', 'cpuacct'],
        ['perf_event'],
        ['net_cls', 'net_prio'],
    ]

    ids = [str(i) for i in range(10)]

    container_ids = [
        '3726184226f5d3147c25fdeab5b60097e378e8a720503a5e19ecfdf29f869860',
        '37261842-26f5-d314-7c25-fdeab5b60097',
        '37261842_26f5_d314_7c25_fdeab5b60097',
    ]

    pod_ids = [
        '3d274242-8ee0-11e9-a8a6-1e68d864ef1a',
        '3d274242_8ee0_11e9_a8a6_1e68d864ef1a',
    ]

    paths = [
        # Docker
        '/docker/{0}',
        '/docker/{0}.scope',

        # k8s
        '/kubepods/test/pod{1}/{0}',
        '/kubepods/test/pod{1}.slice/{0}',
        '/kubepods/test/pod{1}/{0}.scope',
        '/kubepods/test/pod{1}.slice/{0}.scope',

        # ECS
        '/ecs/test-ecs-classic/5a0d5ceddf6c44c1928d367a815d890f/{0}',
        '/ecs/test-ecs-classic/5a0d5ceddf6c44c1928d367a815d890f/{0}.scope',

        # Fargate
        '/ecs/55091c13-b8cf-4801-b527-f4601742204d/{0}',
        '/ecs/55091c13-b8cf-4801-b527-f4601742204d/{0}.scope',

        # Linux non-containerized
        '/user.slice/user-0.slice/session-83.scope',
    ]

    valid_test_cases = dict(
        (
            ':'.join([id, ','.join(groups), path.format(container_id, pod_id)]),
            CGroupInfo(
                id=id,
                groups=','.join(groups),
                path=path.format(container_id, pod_id),
                controllers=groups,
                container_id=container_id if '{0}' in path else None,
                pod_id=pod_id if '{1}' in path else None,
            )
        )
        for path, id, groups, container_id, pod_id
        in itertools.product(paths, ids, controllers, container_ids, pod_ids)
    )
    # Dedupe test cases
    valid_test_cases = list(valid_test_cases.items())

    # Assert here to ensure we are always testing the number of cases we expect
    assert len(valid_test_cases) == 2150

    return valid_test_cases
