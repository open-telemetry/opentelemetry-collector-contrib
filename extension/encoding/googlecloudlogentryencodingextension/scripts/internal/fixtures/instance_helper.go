// instance_helper discovers instances in a regional managed instance group,
// resolves each instance's zone and primary internal IP, assigns a peer IP in a
// circular fashion (used by the shell script to set metadata for traffic
// generation), and emits the result as JSON:
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
	maxAttempts         = 12
	waitBetweenAttempts = 10 * time.Second
)

func main() {
	var migName string
	var region string
	var expectedSize int

	flag.StringVar(&migName, "mig-name", "", "Managed instance group name")
	flag.StringVar(&region, "region", "", "Managed instance group region")
	flag.IntVar(&expectedSize, "expected-size", defaultExpectedSize, "Minimum number of instances required")
	flag.Parse()

	if migName == "" {
		exitErr(errors.New("missing required flag --mig-name"))
	}
	if region == "" {
		exitErr(errors.New("missing required flag --region"))
	}
	if expectedSize < 2 {
		expectedSize = 2
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		entries, err := collectEntries(migName, region, expectedSize)
		if err == nil {
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
	fmt.Fprintf(os.Stderr, "instance_helper: %v\n", err)
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
