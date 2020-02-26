import re

from ..logger import get_logger

log = get_logger(__name__)


class CGroupInfo(object):
    """
    CGroup class for container information parsed from a group cgroup file
    """

    __slots__ = ("id", "groups", "path", "container_id", "controllers", "pod_id")

    UUID_SOURCE_PATTERN = r"[0-9a-f]{8}[-_][0-9a-f]{4}[-_][0-9a-f]{4}[-_][0-9a-f]{4}[-_][0-9a-f]{12}"
    CONTAINER_SOURCE_PATTERN = r"[0-9a-f]{64}"

    LINE_RE = re.compile(r"^(\d+):([^:]*):(.+)$")
    POD_RE = re.compile(r"pod({0})(?:\.slice)?$".format(UUID_SOURCE_PATTERN))
    CONTAINER_RE = re.compile(r"({0}|{1})(?:\.scope)?$".format(UUID_SOURCE_PATTERN, CONTAINER_SOURCE_PATTERN))

    def __init__(self, **kwargs):
        # Initialize all attributes in __slots__ to `None`
        # DEV: Otherwise we'll get `AttributeError` when trying to access if they are unset
        for attr in self.__slots__:
            setattr(self, attr, kwargs.get(attr))

    @classmethod
    def from_line(cls, line):
        """
        Parse a new :class:`CGroupInfo` from the provided line

        :param line: A line from a cgroup file (e.g. /proc/self/cgroup) to parse information from
        :type line: str
        :returns: A :class:`CGroupInfo` object with all parsed data, if the line is valid, otherwise `None`
        :rtype: :class:`CGroupInfo` | None

        """
        # Clean up the line
        line = line.strip()

        # Ensure the line is valid
        match = cls.LINE_RE.match(line)
        if not match:
            return None

        # Create our new `CGroupInfo` and set attributes from the line
        info = cls()
        info.id, info.groups, info.path = match.groups()

        # Parse the controllers from the groups
        info.controllers = [c.strip() for c in info.groups.split(",") if c.strip()]

        # Break up the path to grab container_id and pod_id if available
        # e.g. /docker/<container_id>
        # e.g. /kubepods/test/pod<pod_id>/<container_id>
        parts = [p for p in info.path.split("/")]

        # Grab the container id from the path if a valid id is present
        if len(parts):
            match = cls.CONTAINER_RE.match(parts.pop())
            if match:
                info.container_id = match.group(1)

        # Grab the pod id from the path if a valid id is present
        if len(parts):
            match = cls.POD_RE.match(parts.pop())
            if match:
                info.pod_id = match.group(1)

        return info

    def __str__(self):
        return self.__repr__()

    def __repr__(self):
        return "{}(id={!r}, groups={!r}, path={!r}, container_id={!r}, controllers={!r}, pod_id={!r})".format(
            self.__class__.__name__, self.id, self.groups, self.path, self.container_id, self.controllers, self.pod_id,
        )


def get_container_info(pid="self"):
    """
    Helper to fetch the current container id, if we are running in a container

    We will parse `/proc/{pid}/cgroup` to determine our container id.

    The results of calling this function are cached

    :param pid: The pid of the cgroup file to parse (default: 'self')
    :type pid: str | int
    :returns: The cgroup file info if found, or else None
    :rtype: :class:`CGroupInfo` | None
    """
    try:
        cgroup_file = "/proc/{0}/cgroup".format(pid)
        with open(cgroup_file, mode="r") as fp:
            for line in fp:
                info = CGroupInfo.from_line(line)
                if info and info.container_id:
                    return info
    except Exception:
        log.debug("Failed to parse cgroup file for pid %r", pid, exc_info=True)

    return None
