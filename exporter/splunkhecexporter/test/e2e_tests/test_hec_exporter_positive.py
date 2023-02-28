import time

import pytest
import logging
import sys
import os
from ..event_generator import send_event, send_metric, send_event_with_timeoffset, add_filelog_event
from ..common import check_events_from_splunk, check_metrics_from_splunk

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)
formatter = logging.Formatter('%(message)s')
handler = logging.StreamHandler(sys.stdout)
handler.setFormatter(formatter)
logger.addHandler(handler)


# @pytest.mark.skip
@pytest.mark.parametrize("test_data,expected", [
    ("test_data", 1)
])
def test_splunk_event_metadata_as_string(setup, test_data, expected):
    '''
    What is this test doing
    '''
    logger.info("\n----------")
    logger.info("testing test_splunk_index input={0} expected={1} event(s)".format(
        test_data, expected))
    index = "main"
    source = "source_test_1"
    sourcetype = "sourcetype_test_1"
    send_event(index, source, sourcetype, test_data)
    search_query = "index=" + index + " sourcetype=" + sourcetype + " source=" + source
    logger.info(f"SPL: {search_query}")
    events = check_events_from_splunk(start_time="-1m@m",
                                      url=setup["splunkd_url"],
                                      user=setup["splunk_user"],
                                      query=["search {0}".format(
                                          search_query)],
                                      password=setup["splunk_password"])
    logger.info("Splunk received %s events in the last minute",
                len(events))
    assert len(events) >= expected

    logger.info("Test Finished")


# @pytest.mark.skip
def test_splunk_event_json_object(setup):
    '''
     What is this test doing
     '''
    logger.info("Starting second test!")
    logger.info("\n----------")
    index = "sck-otel"
    source = "source_test_2"
    sourcetype = "sourcetype_test_2"
    json_data = {
        "test_name": "Test 2b timestamp",
        "field1": "test",
        "field2": 155,
        "index": index
    }

    send_event(index, source, sourcetype, json_data)
    search_query = "index=" + index + " sourcetype=" + sourcetype + " source=" + source
    logger.info(f"SPL: {search_query}")
    events = check_events_from_splunk(start_time="-1m@m",
                                      url=setup["splunkd_url"],
                                      user=setup["splunk_user"],
                                      query=["search {0}".format(
                                          search_query)],
                                      password=setup["splunk_password"])
    logger.info("Splunk received %s events in the last minute",
                len(events))
    assert len(events) >= 1
    logger.info("Test Finished")


# @pytest.mark.skip
def test_splunk_event_timestamp(setup):
    '''
     What is this test doing
     '''
    logger.info("Starting third test!")
    logger.info("\n----------")
    index = "sck-otel"
    source = "source_test_3"
    sourcetype = "sourcetype_test_3"
    event_timestamp = time.time() - 1800
    json_data = {
        "time": event_timestamp,
        "test_name": "Test  timestamp",
        "index": index
    }
    send_event_with_timeoffset(index, source, sourcetype, json_data, event_timestamp)
    search_query = "index=" + index + " sourcetype=" + sourcetype + " source=" + source
    logger.info(f"SPL: {search_query}")
    events = check_events_from_splunk(start_time="-1m@m",
                                      url=setup["splunkd_url"],
                                      user=setup["splunk_user"],
                                      query=["search {0}".format(
                                          search_query)],
                                      password=setup["splunk_password"])
    past_events = check_events_from_splunk(start_time="-30m@m",
                                           end_time="-29m@m",
                                           url=setup["splunkd_url"],
                                           user=setup["splunk_user"],
                                           query=["search {0}".format(
                                               search_query)],
                                           password=setup["splunk_password"])
    logger.info("Splunk received %s events in the last minute",
                len(events))
    assert len(events) == 0
    assert len(past_events) >= 1
    logger.info("Test Finished")


# @pytest.mark.skip
@pytest.mark.parametrize("data,expected_event_count", [("This is the event!", 1),
                                                       ('{"test_name": "json_event","source": "file_log"}', 2)])
def test_events_source_file_log_receiver_text(setup, data, expected_event_count):
    '''
     What is this test doing
     '''
    logger.info("Starting file log receiver test!")
    logger.info("\n----------")
    index = "file_logs"
    sourcetype = "filelog_sourcetype"

    search_query = "index=" + index + " sourcetype=" + sourcetype
    logger.info(f"SPL: {search_query}")
    events = check_events_from_splunk(start_time="-1m@m",
                                      url=setup["splunkd_url"],
                                      user=setup["splunk_user"],
                                      query=["search {0}".format(
                                          search_query)],
                                      password=setup["splunk_password"])
    logger.info("Splunk received %s events in the last minute",
                len(events))
    assert len(events) == expected_event_count - 1
    add_filelog_event(data)

    events = check_events_from_splunk(start_time="-1m@m",
                                      url=setup["splunkd_url"],
                                      user=setup["splunk_user"],
                                      query=["search {0}".format(
                                          search_query)],
                                      password=setup["splunk_password"])
    logger.info("Splunk received %s events in the last minute",
                len(events))
    assert len(events) >= expected_event_count
    logger.info("Test Finished")


# @pytest.mark.skip
@pytest.mark.parametrize("metric_name", [
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
    "system.cpu.load_average.15m"

])
def test_splunk_hostmetric_events(setup, metric_name):
    '''
     What is this test doing
     '''
    logger.info("\n----------")
    index = "metrics"
    source = "source_test_2"
    sourcetype = "sourcetype_test_2"

    send_metric(index, source, sourcetype)
    events = check_metrics_from_splunk(start_time="-1m@m",
                                       url=setup["splunkd_url"],
                                       user=setup["splunk_user"],
                                       metric_name=metric_name,
                                       index=index,
                                       password=setup["splunk_password"])
    logger.info("Splunk received %s events in the last minute",
                len(events))
    assert len(events) >= 1
    logger.info("Test Finished")


# @pytest.mark.skip
def test_splunk_metric_events(setup):
    '''
     What is this test doing
     '''
    logger.info("\n----------")
    index = "sck-metrics"
    source = "source_test_metric"
    sourcetype = "sourcetype_test_metric"

    send_metric(index, source, sourcetype)
    events = check_metrics_from_splunk(start_time="-1m@m",
                                       url=setup["splunkd_url"],
                                       user=setup["splunk_user"],
                                       metric_name="test.metric",
                                       index=index,
                                       password=setup["splunk_password"])
    logger.info("Splunk received %s events in the last minute",
                len(events))
    assert len(events) >= 1
    logger.info("Test Finished")
