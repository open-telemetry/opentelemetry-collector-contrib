package pytransform

// prefixScript is a python script that is prepended to the user's script.
var prefixScript = `
import json
import os
import urllib.request
import sys

# override print function to send logs back to otel
def print(msg, endpoint=os.environ.get("LOG_URL"), pipeline=os.environ.get("PIPELINE")):
	# Encode string to bytes
	data_bytes = json.dumps({
		"message":str(msg),
		"pipeline": pipeline,
	}).encode('utf-8')

	req = urllib.request.Request(endpoint, data=data_bytes, method='POST')

	with urllib.request.urlopen(req) as resp:
		return resp.read().decode()


def send(event):
	sys.stdout.write(json.dumps(event))

# read input event from env var
try:
	event = json.loads(os.environ.get("INPUT"))
except Exception as e:
	raise Exception(f"error loading input event from env var: {e}")
`
