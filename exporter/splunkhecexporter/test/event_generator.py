import logging
import time

import requests


def wait_for_event_to_be_indexed_in_splunk():
    logging.info("Wating for event indexing")
    time.sleep(3)


def send_event_old():
    print("sending event")
    json_data = {"event": "Pony 8 has left the barn", "index": "main"}
    requests.post("http://0.0.0.0:8883", json=json_data, verify=False)
    wait_for_event_to_be_indexed_in_splunk()


def send_event(index, source, source_type, data):
    print("sending event")
    json_data = {"event": data,
                 # "time": 1677241967.317544,
                 "index": index,
                 "host": "localhost",
                 "source": source,
                 "sourcetype": source_type,
                 }
    requests.post("http://0.0.0.0:8883", json=json_data, verify=False)
    wait_for_event_to_be_indexed_in_splunk()


def send_event_with_timeoffset(index, source, source_type, data, time):
    print("sending event")
    json_data = {"event": data,
                 "time": time,
                 "index": index,
                 "host": "localhost",
                 "source": source,
                 "sourcetype": source_type,
                 }
    requests.post("http://0.0.0.0:8883", json=json_data, verify=False)
    wait_for_event_to_be_indexed_in_splunk()


def send_metric(index, source, source_type):
    print("sending metric")
    fields_1 = {"metric_name:test.metric": 123,
                "k0": "v0",
                "k1": "v1"}

    fields_2 = {"region": "us-west-1", "datacenter": "dc1", "rack": "63", "os": "Ubuntu16.10", "arch": "x64",
                "team": "LON", "service": "6", "service_version": "0", "service_environment": "test",
                "path": "/dev/sda1",
                "fstype": "ext3", "metric_name:cpu.usr": 11.12, "metric_name:cpu.sys": 12.23,
                "metric_name:cpu.idle": 13.34}

    json_data = {
        "host": "localhost",
        "source": source,
        "sourcetype": source_type,
        "index": index,
        "event": "metric",
        "fields": fields_1
    }
    requests.post("http://127.0.0.1:8884", json=json_data, verify=False)
    wait_for_event_to_be_indexed_in_splunk()


def add_filelog_event(data):
    print("adding file event")
    f = open("./../../../bin/test_file.json", "a")
    # f.write(data)
    # f.close()

    for line in data:
        f.write(line)
    f.write("\n")
    f.close()
    wait_for_event_to_be_indexed_in_splunk()


# if __name__ == "__main__":
    # send_event()
    # send_metric()
    # add_filelog_event()
