import logging
import time

import requests


def wait_for_event_to_be_indexed_in_splunk():
    logging.info("Wating for event indexing")
    time.sleep(3)


def send_event(index, source, source_type, data):
    print("Sending event")
    json_data = {
        "event": data,
        "index": index,
        "host": "localhost",
        "source": source,
        "sourcetype": source_type,
    }
    requests.post("http://0.0.0.0:8883", json=json_data, verify=False)
    wait_for_event_to_be_indexed_in_splunk()


def send_event_with_time_offset(index, source, source_type, data, time):
    print("Sending event with time offset")
    json_data = {
        "event": data,
        "time": time,
        "index": index,
        "host": "localhost",
        "source": source,
        "sourcetype": source_type,
    }
    requests.post("http://0.0.0.0:8883", json=json_data, verify=False)
    wait_for_event_to_be_indexed_in_splunk()


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
    requests.post("http://127.0.0.1:8884", json=json_data, verify=False)
    wait_for_event_to_be_indexed_in_splunk()


def add_filelog_event(data):
    print("Adding file event")
    f = open("./../../../bin/test_file.json", "a")

    for line in data:
        f.write(line)
    f.write("\n")
    f.close()
    wait_for_event_to_be_indexed_in_splunk()


# if __name__ == "__main__":
# send_event()
# send_metric()
# add_filelog_event()
