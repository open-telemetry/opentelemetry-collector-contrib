# Copyright The OpenTelemetry Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# A default event name to be used for logging events when a better event name
# can't be derived from the event's key-value pairs.
DEFAULT_EVENT_NAME = "log"


def time_seconds_to_ns(time_seconds):
    """Converts a time value in seconds to a time value in nanoseconds.

    `time_seconds` is a `float` as returned by `time.time()` which represents
    the number of seconds since the epoch.

    The returned value is an `int` representing the number of nanoseconds since
    the epoch.
    """

    return int(time_seconds * 1e9)


def time_seconds_from_ns(time_nanoseconds):
    """Converts a time value in nanoseconds to a time value in seconds.

    `time_nanoseconds` is an `int` representing the number of nanoseconds since
    the epoch.

    The returned value is a `float` representing the number of seconds since
    the epoch.
    """

    return time_nanoseconds / 1e9


def event_name_from_kv(key_values):
    """A helper function which returns an event name from the given dict, or a
    default event name.
    """

    if key_values is None or "event" not in key_values:
        return DEFAULT_EVENT_NAME

    return key_values["event"]
