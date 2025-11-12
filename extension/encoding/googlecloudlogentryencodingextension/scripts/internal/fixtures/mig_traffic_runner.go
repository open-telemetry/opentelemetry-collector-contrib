// mig_traffic_runner discovers instances in a regional managed instance group,
// resolves each instance's zone and primary internal IP, assigns a peer IP in a
// circular fashion, and optionally connects to each instance over SSH to run
// traffic generation commands. When traffic generation is not requested, the
// tool emits the discovered instances as JSON:
//
//	{
//	  "instances": [
//	    {"name": "vm-1", "zone": "us-central1-a", "ip": "10.10.0.2", "peer_ip": "10.10.0.3"},
//	    ...
//	  ]
//	}
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type migInstance struct {
	Instance string `json:"instance"`
}

type instanceOutput struct {
	Entries []instanceEntry `json:"instances"`
}

type instanceEntry struct {
	Name   string `json:"name"`
	Zone   string `json:"zone"`
	IP     string `json:"ip"`
	PeerIP string `json:"peer_ip"`
}

const (
	defaultExpectedSize = 2
	maxAttempts         = 18
	waitBetweenAttempts = 10 * time.Second
)

func main() {
	var migName string
	var region string
	var expectedSize int
	var projectID string
	var generateTraffic bool

	flag.StringVar(&migName, "mig-name", "", "Managed instance group name")
	flag.StringVar(&region, "region", "", "Managed instance group region")
	flag.IntVar(&expectedSize, "expected-size", defaultExpectedSize, "Minimum number of instances required")
	flag.StringVar(&projectID, "project-id", "", "Google Cloud project ID (required when --generate-traffic is set)")
	flag.BoolVar(&generateTraffic, "generate-traffic", false, "When set, run traffic generation commands on each instance via SSH")
	flag.Parse()

	if migName == "" {
		exitErr(errors.New("missing required flag --mig-name"))
	}
	if region == "" {
		exitErr(errors.New("missing required flag --region"))
	}
	if generateTraffic && projectID == "" {
		exitErr(errors.New("--project-id is required when --generate-traffic is set"))
	}
	if expectedSize < 2 {
		expectedSize = 2
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		entries, err := collectEntries(migName, region, expectedSize)
		if err == nil {
			if generateTraffic {
				if err := executeTrafficGeneration(entries, projectID); err != nil {
					exitErr(err)
				}
				return
			}
			output(entries)
			return
		}
		if !errors.Is(err, errNotReady) {
			exitErr(err)
		}
		if attempt == maxAttempts {
			exitErr(fmt.Errorf("instances not ready after %d attempts: %w", attempt, err))
		}
		time.Sleep(waitBetweenAttempts)
	}
}

var errNotReady = errors.New("instances not ready")

func collectEntries(migName, region string, expectedSize int) ([]instanceEntry, error) {
	instances, err := listInstances(migName, region)
	if err != nil {
		return nil, err
	}
	if len(instances) < expectedSize {
		return nil, errNotReady
	}

	entries := make([]instanceEntry, len(instances))
	for i, instURL := range instances {
		name, zone := parseNameZone(instURL)
		if name == "" || zone == "" {
			return nil, fmt.Errorf("unable to parse instance or zone from %q", instURL)
		}
		ip, err := instanceInternalIP(name, zone)
		if err != nil {
			if errors.Is(err, errNotReady) {
				return nil, errNotReady
			}
			return nil, err
		}
		entries[i] = instanceEntry{Name: name, Zone: zone, IP: ip}
	}

	ready := make([]instanceEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IP == "" {
			return nil, errNotReady
		}
		ready = append(ready, entry)
	}
	if len(ready) < expectedSize {
		return nil, errNotReady
	}

	for i := range ready {
		peer := ready[(i+1)%len(ready)]
		ready[i].PeerIP = peer.IP
	}
	return ready, nil
}

func output(entries []instanceEntry) {
	out := instanceOutput{Entries: entries}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		exitErr(err)
	}
}

func exitErr(err error) {
	fmt.Fprintf(os.Stderr, "mig_traffic_runner: %v\n", err)
	os.Exit(1)
}

func listInstances(migName, region string) ([]string, error) {
	cmd := exec.Command("gcloud", "compute", "instance-groups", "managed", "list-instances", migName,
		"--region", region, "--format", "json(instance)")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing instances: %w", err)
	}

	var payload []migInstance
	if err := json.Unmarshal(output, &payload); err != nil {
		return nil, fmt.Errorf("parsing gcloud output: %w", err)
	}

	result := make([]string, 0, len(payload))
	for _, entry := range payload {
		if entry.Instance != "" {
			result = append(result, entry.Instance)
		}
	}
	return result, nil
}

func instanceInternalIP(name, zone string) (string, error) {
	cmd := exec.Command("gcloud", "compute", "instances", "describe", name, "--zone", zone, "--format", "value(networkInterfaces[0].networkIP)")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("describe instance %s: %w", name, err)
	}
	ip := strings.TrimSpace(string(output))
	if ip == "" {
		return "", errNotReady
	}
	return ip, nil
}

func parseNameZone(instanceURL string) (string, string) {
	parts := strings.Split(instanceURL, "/")
	if len(parts) < 2 {
		return "", ""
	}
	name := parts[len(parts)-1]
	zone := ""
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == "zones" && i+1 < len(parts) {
			zone = parts[i+1]
			break
		}
	}
	return name, zone
}

func executeTrafficGeneration(entries []instanceEntry, projectID string) error {
	total := len(entries)
	if total == 0 {
		return errors.New("no instances discovered for traffic generation")
	}

	successes := 0
	for _, inst := range entries {
		fmt.Printf("mig_traffic_runner: running traffic commands on %s (%s)\n", inst.Name, inst.Zone)
		if err := runTrafficForInstance(inst, projectID); err != nil {
			fmt.Fprintf(os.Stderr, "mig_traffic_runner: traffic generation failed for %s: %v\n", inst.Name, err)
			continue
		}
		successes++
	}

	fmt.Printf("mig_traffic_runner: traffic generation completed (%d/%d succeeded)\n", successes, total)
	if successes == 0 {
		return errors.New("traffic generation failed for all instances")
	}
	return nil
}

func runTrafficForInstance(inst instanceEntry, projectID string) error {
	script := fmt.Sprintf(`set -euo pipefail
export DEBIAN_FRONTEND=noninteractive
apt-get -o Acquire::ForceIPv4=true update
apt-get -o Acquire::ForceIPv4=true install -y -q iperf3 curl jq

peer_ip="%s"
if [ -n "$peer_ip" ]; then
  ping -c 40 "$peer_ip" || true
  iperf3 -c "$peer_ip" -t 45 -P 4 || true
fi

curl -sv https://storage.googleapis.com/ -o /tmp/storage_index.html || true
curl -sv https://www.googleapis.com/discovery/v1/apis -o /tmp/discovery.json || true

project_id="$(curl -s -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/project/project-id")"
access_token="$(curl -s -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token" | jq -r '.access_token')"
if [ -n "$access_token" ] && [ "$access_token" != "null" ]; then
  curl -sv -H "Authorization: Bearer $access_token" "https://storage.googleapis.com/storage/v1/b?project=$project_id&maxResults=1" -o /tmp/storage_buckets.json || true
  curl -sv -H "Authorization: Bearer $access_token" "https://compute.googleapis.com/compute/v1/projects/$project_id/regions" -o /tmp/compute_regions.json || true
fi
`, inst.PeerIP)

	return sshRunCommand(inst.Name, inst.Zone, projectID, script)
}

func sshRunCommand(instanceName, zone, projectID, script string) error {
	args := []string{
		"compute", "ssh", instanceName,
		"--project", projectID,
		"--zone", zone,
		"--command", "sudo bash -s",
		"--",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
	}

	cmd := exec.Command("gcloud", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = strings.NewReader(script)
	return cmd.Run()
}
