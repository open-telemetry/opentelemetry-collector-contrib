from typing import Collection, Optional

from pkg_resources import (
    Distribution,
    DistributionNotFound,
    VersionConflict,
    get_distribution,
)


class DependencyConflict:
    required: str = None
    found: Optional[str] = None

    def __init__(self, required, found=None):
        self.required = required
        self.found = found

    def __str__(self):
        return 'DependencyConflict: requested: "{0}" but found: "{1}"'.format(
            self.required, self.found
        )


def get_dist_dependency_conflicts(
    dist: Distribution,
) -> Optional[DependencyConflict]:
    deps = [
        dep
        for dep in dist.requires(("instruments",))
        if dep not in dist.requires()
    ]
    return get_dependency_conflicts(deps)


def get_dependency_conflicts(
    deps: Collection[str],
) -> Optional[DependencyConflict]:
    for dep in deps:
        try:
            get_distribution(str(dep))
        except VersionConflict as exc:
            return DependencyConflict(dep, exc.dist)
        except DistributionNotFound:
            return DependencyConflict(dep)
    return None
