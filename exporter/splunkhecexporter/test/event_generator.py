import logging
import time

import requests

from config.config_data import (
    METRICS_RECEIVER_URL,
    EVENTS_RECEIVER_URL,
    PATH_TO_OTEL_COLLECTOR_BIN_DIR,
)


def wait_for_event_to_be_indexed_in_splunk(time_to_wait=3):
    logging.info("Waiting for event indexing")
    time.sleep(time_to_wait)


def send_event(index, source, source_type, data, url=EVENTS_RECEIVER_URL):
    print("Sending event")
    json_data = {
        "event": data,
        "index": index,
        "host": "localhost",
        "source": source,
        "sourcetype": source_type,
    }
    response = requests.post(url, json=json_data, verify=False)
    wait_for_event_to_be_indexed_in_splunk()
    return response


def send_event_with_time_offset(index, source, source_type, data, timestamp):
    print("Sending event with time offset")
    json_data = {
        "event": data,
        "time": timestamp,
        "index": index,
        "host": "localhost",
        "source": source,
        "sourcetype": source_type,
    }
    response = requests.post(EVENTS_RECEIVER_URL, json=json_data, verify=False)
    print(response.text)
    print(response.status_code)
    wait_for_event_to_be_indexed_in_splunk(10)


def send_metric(index, source, source_type):
    print("Sending metric event")
    fields = {
        "metric_name:test.metric": 123,
        "k0": "v0",
        "k1": "v1",
        "metric_name:cpu.usr": 11.12,
        "metric_name:cpu.sys": 12.23,
    }
    json_data = {
        "host": "localhost",
        "source": source,
        "sourcetype": source_type,
        "index": index,
        "event": "metric",
        "fields": fields,
    }
    requests.post(METRICS_RECEIVER_URL, json=json_data, verify=False)
    wait_for_event_to_be_indexed_in_splunk()


def add_filelog_event(data):
    print("Adding file event")
    f = open(PATH_TO_OTEL_COLLECTOR_BIN_DIR + "/test_file.json", "a")

    for line in data:
        f.write(line)
    f.write("\n")
    f.close()
    wait_for_event_to_be_indexed_in_splunk()
