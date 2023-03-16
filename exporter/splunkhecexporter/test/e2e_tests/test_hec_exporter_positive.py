import time

import pytest
import logging
import sys
import config.config_data as config
from ..event_generator import (
    send_event,
    send_metric,
    send_event_with_time_offset,
    add_filelog_event,
)
from ..common import check_events_from_splunk, check_metrics_from_splunk

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)
formatter = logging.Formatter("%(message)s")
handler = logging.StreamHandler(sys.stdout)
handler.setFormatter(formatter)
logger.addHandler(handler)


# @pytest.mark.skip
@pytest.mark.parametrize("test_data,expected", [("test_data", 1)])
def test_splunk_event_metadata_as_string(setup, test_data, expected):
    """
    What is this test doing
    - check text event received via event endpoint
    """
    logger.info("-- Starting: check text event received via event endpoint --")
    index = config.EVENT_INDEX_1
    source = "source_test_1"
    sourcetype = "sourcetype_test_1"
    send_event(index, source, sourcetype, test_data)
    search_query = "index=" + index + " sourcetype=" + sourcetype + " source=" + source
    events = check_events_from_splunk(
        start_time="-1m@m",
        url=setup["splunkd_url"],
        user=setup["splunk_user"],
        query=["search {0}".format(search_query)],
        password=setup["splunk_password"],
    )
    logger.info("Splunk received %s events in the last minute", len(events))
    assert len(events) >= expected

    logger.info("Test Finished")


# @pytest.mark.skip
def test_splunk_event_json_object(setup):
    """
    What is this test doing
    - check json event received via event endpoint
    """
    logger.info("-- Starting: check json event received via event endpoint --")
    index = config.EVENT_INDEX_2
    source = "source_test_2"
    sourcetype = "sourcetype_test_2"
    json_data = {
        "test_name": "JSON Object",
        "field1": "test",
        "field2": 155,
        "index": index,
    }

    send_event(index, source, sourcetype, json_data)
    search_query = "index=" + index + " sourcetype=" + sourcetype + " source=" + source
    events = check_events_from_splunk(
        start_time="-1m@m",
        url=setup["splunkd_url"],
        user=setup["splunk_user"],
        query=["search {0}".format(search_query)],
        password=setup["splunk_password"],
    )
    logger.info("Splunk received %s events in the last minute", len(events))
    assert len(events) >= 1
    logger.info("Test Finished")


# @pytest.mark.skip
def test_splunk_event_timestamp(setup):
    """
    What is this test doing
    - check event timestamp test
    """
    logger.info("-- Starting: check event timestamp test --")
    index = config.EVENT_TIMESTAMP
    source = "source_test_3"
    sourcetype = "sourcetype_test_3"
    time_base = time.time()
    event_timestamp = time_base - 1800
    json_data = {
        "time": event_timestamp,
        "test_name": "Test timestamp",
        "index": index,
    }
    logger.info("Timestamp event body: {}".format(json_data))
    send_event_with_time_offset(index, source, sourcetype, json_data, event_timestamp)
    search_query = "index=" + index + " sourcetype=" + sourcetype + " source=" + source
    events = check_events_from_splunk(
        start_time="-1m@m",
        url=setup["splunkd_url"],
        user=setup["splunk_user"],
        query=["search {0}".format(search_query)],
        password=setup["splunk_password"],
    )
    past_events = check_events_from_splunk(
        start_time="-31m@m",
        end_time="-29m@m",
        url=setup["splunkd_url"],
        user=setup["splunk_user"],
        query=["search {0}".format(search_query)],
        password=setup["splunk_password"],
    )
    logger.info("Splunk received %s events in the last minute", len(events))
    logger.info(
        "Splunk received %s events in the selected timeframe (~30 mins ago)",
        len(past_events),
    )
    logger.info("--------------------")
    logger.info("Timestamp {}".format(event_timestamp))
    logger.info("Time base {}".format(time_base))
    logger.info("Event {}".format(past_events))
    logger.info("--------------------")
    assert len(events) == 0
    assert len(past_events) >= 1
    logger.info("Test Finished")


# @pytest.mark.skip
@pytest.mark.parametrize(
    "data,expected_event_count",
    [
        ("This is the event!", 1),
        ({"test_name": "json_event", "source": "file_log"}, 2),
    ],
)
def test_events_source_file_log_receiver_text(setup, data, expected_event_count):
    """
    What is this test doing
    - check events received via file log receiver
    """
    logger.info(" -- Starting: file log receiver test! --")
    index = config.EVENT_INDEX_FILE_LOG
    sourcetype = "filelog_sourcetype"

    search_query = "index=" + index + " sourcetype=" + sourcetype
    events = check_events_from_splunk(
        start_time="-1m@m",
        url=setup["splunkd_url"],
        user=setup["splunk_user"],
        query=["search {0}".format(search_query)],
        password=setup["splunk_password"],
    )
    logger.info("Splunk received %s events in the last minute", len(events))
    assert len(events) == expected_event_count - 1
    add_filelog_event(data)

    events = check_events_from_splunk(
        start_time="-1m@m",
        url=setup["splunkd_url"],
        user=setup["splunk_user"],
        query=["search {0}".format(search_query)],
        password=setup["splunk_password"],
    )
    logger.info("Splunk received %s events in the last minute", len(events))
    assert len(events) >= expected_event_count
    logger.info("Test Finished")


# @pytest.mark.skip
@pytest.mark.parametrize(
    "metric_name",
    [
        "system.filesystem.inodes.usage",
        "system.filesystem.usage",
        "system.memory.usage",
        "system.network.connections",
        "system.network.dropped",
        "system.network.errors",
        "system.network.io",
        "system.network.packets",
        "system.cpu.load_average.1m",
        "system.cpu.load_average.5m",
        "system.cpu.load_average.15m",
    ],
)
def test_splunk_host_metric_events(setup, metric_name):
    """
    What is this test doing
    - host metrics receiver test
    """
    logger.info("\n-- Starting: host metrics test! --")
    index = config.METRICS_INDEX_HOST_METRICS
    events = check_metrics_from_splunk(
        start_time="-1m@m",
        url=setup["splunkd_url"],
        user=setup["splunk_user"],
        metric_name=metric_name,
        index=index,
        password=setup["splunk_password"],
    )
    logger.info("Splunk received %s events in the last minute", len(events))
    assert len(events) >= 1
    logger.info("Test Finished")


# @pytest.mark.skip
def test_splunk_metric_events(setup):
    """
    What is this test doing
    - check metrics via metrics endpoint test
    """
    logger.info("\n-- Starting: check metrics via metrics endpoint test --")
    index = config.METRICS_INDEX_METRICS_ENDPOINT
    source = "source_test_metric"
    sourcetype = "sourcetype_test_metric"

    send_metric(index, source, sourcetype)
    events = check_metrics_from_splunk(
        start_time="-1m@m",
        url=setup["splunkd_url"],
        user=setup["splunk_user"],
        metric_name="test.metric",
        index=index,
        password=setup["splunk_password"],
    )
    logger.info("Splunk received %s events in the last minute", len(events))
    assert len(events) >= 1
    logger.info("Test Finished")
