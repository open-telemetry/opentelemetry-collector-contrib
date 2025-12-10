from flask import Flask, jsonify

app = Flask(__name__)

@app.route('/', methods=['GET'])
def home():
    return jsonify({
  "DockerId": "196a0e6abfce1e31ee24b65e97875f089878dd7d1d7e9f15155d6094c8b908f5",
  "Name": "cadvisor",
  "DockerName": "ecs-cadvisor-task-definition-7-cadvisor-bae592b5e4c1a3bb3800",
  "Image": "gcr.io/cadvisor/cadvisor:latest",
  "ImageID": "sha256:68c29634fe49724f94ed34f18224316f776392f7a5a4014969ac5798a2ec96dc",
  "Ports": [
    {
      "ContainerPort": 8080,
      "Protocol": "tcp",
      "HostPort": 32911,
      "HostIp": "0.0.0.0"
    },
    {
      "ContainerPort": 8080,
      "Protocol": "tcp",
      "HostPort": 32911,
      "HostIp": "::"
    }
  ],
  "Labels": {
    "ECS_PROMETHEUS_EXPORTER_PORT": "8080",
    "ECS_PROMETHEUS_METRICS_PATH": "/metrics",
    "com.amazonaws.ecs.cluster": "cds-305",
    "com.amazonaws.ecs.container-name": "cadvisor",
    "com.amazonaws.ecs.task-arn": "arn:aws:ecs:eu-west-1:035955823196:task/cds-305/ec7ff82b7a3a44a5bbbe9bcf11daee33",
    "com.amazonaws.ecs.task-definition-family": "cadvisor-task-definition",
    "com.amazonaws.ecs.task-definition-version": "7"
  },
  "DesiredStatus": "RUNNING",
  "KnownStatus": "RUNNING",
  "Limits": {
    "CPU": 10,
    "Memory": 300
  },
  "CreatedAt": "2023-06-22T12:41:18.315883278Z",
  "StartedAt": "2023-06-22T12:41:18.713571182Z",
  "Type": "NORMAL",
  "Volumes": [
    {
      "Source": "/var",
      "Destination": "/var"
    },
    {
      "Source": "/etc",
      "Destination": "/etc"
    }
  ],
  "ContainerARN": "arn:aws:ecs:eu-west-1:035955823196:container/cds-305/ec7ff82b7a3a44a5bbbe9bcf11daee33/cc1c133f-bd1f-4006-8dae-4cd8a3f54f19",
  "Networks": [
    {
      "NetworkMode": "bridge",
      "IPv4Addresses": [
        "172.17.0.2"
      ]
    }
  ]
})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8080)
