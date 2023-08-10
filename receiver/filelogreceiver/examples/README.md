# Filelog Receiver Ready to experiment exampler configs

Contains there files to setup filelog receiver example with end to end flow, follow the following steps to get started!

### Step1:
Setup a demo application generating logs in json format, any other application can be used for log generation with any of the supported formats.
<br />Run:
```bash
kubectl apply -f app.yaml
```

### Step 2:
Setup OTeL as an agent running as a `DaemonSet` to scrape container logs from the application, a service account is associated with the agent to collect k8s metadata using `k8sattributes` metadata processor to enrich resource attributes.
Run:
```bash
kubectl apply -f agent.yaml
```

### Step 3:
Finally setup OTeL as collector exposing http and gRPC endpoint to collect logs from agent.
Run:
```bash
kubectl apply -f collector.yaml
```

The above setup would help you achieve `app -> agent -> collector` log collection workflow, modify the configurations as per your needs.