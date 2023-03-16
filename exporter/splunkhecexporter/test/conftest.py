"""
Copyright 2018-2019 Splunk, Inc..

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
"""

import pytest
import config.config_data as config_data
from common import check_events_from_splunk


def pytest_addoption(parser):
    parser.addoption(
        "--splunkd-url",
        help="splunkd url used to send test data to. \
                          Eg: https://localhost:8089",
        default="https://localhost:8089",
    )
    parser.addoption("--splunk-user", help="splunk username", default="admin")
    parser.addoption(
        "--splunk-password", help="splunk user password", default="password"
    )


# Print events ingested in splunk for debugging
def pytest_unconfigure(config):
    indexes = [
        config_data.EVENT_INDEX_1,
        config_data.EVENT_INDEX_2,
        config_data.EVENT_INDEX_FILE_LOG,
        config_data.EVENT_TIMESTAMP,
    ]
    for index in indexes:
        search_query = "index=" + index + "  | fields *"
        events = check_events_from_splunk(
            start_time="-12h@h",
            url=config.getoption("--splunkd-url"),
            user=config.getoption("--splunk-user"),
            query=["search {0}".format(search_query)],
            password=config.getoption("--splunk-password"),
        )
        print("index=" + index + " event count=" + str(len(events)))
        # debug print all events
        for event in events:
            print(event)

    # debug get _internal ERRORS
    internal_index = "_internal"
    search_query = "index=" + internal_index + "  ERROR"
    errors = check_events_from_splunk(
        start_time="-15m@m",
        url=config.getoption("--splunkd-url"),
        user=config.getoption("--splunk-user"),
        query=["search {0}".format(search_query)],
        password=config.getoption("--splunk-password"),
    )
    print("=================")
    print("index=" + internal_index + " event count=" + str(len(errors)))
    # debug print all error events
    for event in errors:
        print(event)


@pytest.fixture(scope="function")
def setup(request):
    config = {}
    config["splunkd_url"] = request.config.getoption("--splunkd-url")
    config["splunk_user"] = request.config.getoption("--splunk-user")
    config["splunk_password"] = request.config.getoption("--splunk-password")

    return config
