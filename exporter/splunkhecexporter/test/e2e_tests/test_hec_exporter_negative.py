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
@pytest.mark.parametrize(
    "hec_receiver_url",
    [config.INVALID_HEC_EVENTS_RECEIVER_URL, config.DISABLED_HEC_EVENTS_RECEIVER_URL],
)
def test_splunk_event_metadata_as_string(setup, hec_receiver_url):
    """
    What is this test doing
    - check text event received via event endpoint
    """
    logger.info("-- Starting: check text event received via event endpoint --")
    index = config.EVENT_INDEX_1
    source = "source_test_1"
    sourcetype = "sourcetype_test_1"
    test_data = "Invalid test case" + hec_receiver_url
    response = send_event(index, source, sourcetype, test_data, hec_receiver_url)
    assert 200 == response.status_code
    search_query = "index=" + index + " sourcetype=" + sourcetype + " source=" + source
    events = check_events_from_splunk(
        start_time="-1m@m",
        url=setup["splunkd_url"],
        user=setup["splunk_user"],
        query=["search {0}".format(search_query)],
        password=setup["splunk_password"],
    )
    logger.info("Splunk received %s events in the last minute", len(events))
    assert len(events) == 0

    logger.info("Test Finished")
