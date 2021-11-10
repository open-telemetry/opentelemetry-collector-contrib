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
#
"""
Some utils used by the redis integration
"""
from opentelemetry.semconv.trace import (
    DbSystemValues,
    NetTransportValues,
    SpanAttributes,
)


def _extract_conn_attributes(conn_kwargs):
    """Transform redis conn info into dict"""
    attributes = {
        SpanAttributes.DB_SYSTEM: DbSystemValues.REDIS.value,
    }
    db = conn_kwargs.get("db", 0)
    attributes[SpanAttributes.DB_NAME] = db
    attributes[SpanAttributes.DB_REDIS_DATABASE_INDEX] = db
    try:
        attributes[SpanAttributes.NET_PEER_NAME] = conn_kwargs.get(
            "host", "localhost"
        )
        attributes[SpanAttributes.NET_PEER_PORT] = conn_kwargs.get(
            "port", 6379
        )
        attributes[
            SpanAttributes.NET_TRANSPORT
        ] = NetTransportValues.IP_TCP.value
    except KeyError:
        attributes[SpanAttributes.NET_PEER_NAME] = conn_kwargs.get("path", "")
        attributes[
            SpanAttributes.NET_TRANSPORT
        ] = NetTransportValues.UNIX.value

    return attributes


def _format_command_args(args):
    """Format command arguments and trim them as needed"""
    value_max_len = 100
    value_too_long_mark = "..."
    cmd_max_len = 1000
    length = 0
    out = []
    for arg in args:
        cmd = str(arg)

        if len(cmd) > value_max_len:
            cmd = cmd[:value_max_len] + value_too_long_mark

        if length + len(cmd) > cmd_max_len:
            prefix = cmd[: cmd_max_len - length]
            out.append(f"{prefix}{value_too_long_mark}")
            break

        out.append(cmd)
        length += len(cmd)

    return " ".join(out)
