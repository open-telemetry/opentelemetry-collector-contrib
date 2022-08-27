// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !windows
// +build !windows

package podmanreceiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func tmpSock(t *testing.T) (net.Listener, string) {
	f, err := os.CreateTemp(os.TempDir(), "testsock")
	if err != nil {
		t.Fatal(err)
	}
	addr := f.Name()
	os.Remove(addr)

	listener, err := net.Listen("unix", addr)
	if err != nil {
		t.Fatal(err)
	}

	return listener, addr
}

func TestStats(t *testing.T) {
	// stats sample
	statsExample := `{"Error":null,"Stats":[{"AvgCPU":42.04781177856639,"ContainerID":"e6af5805edae6c950003abd5451808b277b67077e400f0a6f69d01af116ef014","Name":"charming_sutherland","PerCPU":null,"CPU":42.04781177856639,"CPUNano":309165846000,"CPUSystemNano":54515674,"SystemNano":1650912926385978706,"MemUsage":27717632,"MemLimit":7942234112,"MemPerc":0.34899036730888044,"NetInput":430,"NetOutput":330,"BlockInput":0,"BlockOutput":0,"PIDs":118,"UpTime":309165846000,"Duration":309165846000}]}`

	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stats") {
			_, err := w.Write([]byte(statsExample))
			assert.NoError(t, err)
		} else {
			_, err := w.Write([]byte{})
			assert.NoError(t, err)
		}
	}))
	srv.Listener = listener
	srv.Start()
	defer srv.Close()

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		// default timeout
		Timeout: 5 * time.Second,
	}

	cli, err := newLibpodClient(zap.NewNop(), config)
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	expectedStats := containerStats{
		AvgCPU:        42.04781177856639,
		ContainerID:   "e6af5805edae6c950003abd5451808b277b67077e400f0a6f69d01af116ef014",
		Name:          "charming_sutherland",
		PerCPU:        nil,
		CPU:           42.04781177856639,
		CPUNano:       309165846000,
		CPUSystemNano: 54515674,
		SystemNano:    1650912926385978706,
		MemUsage:      27717632,
		MemLimit:      7942234112,
		MemPerc:       0.34899036730888044,
		NetInput:      430,
		NetOutput:     330,
		BlockInput:    0,
		BlockOutput:   0,
		PIDs:          118,
		UpTime:        309165846000 * time.Nanosecond,
		Duration:      309165846000,
	}

	stats, err := cli.stats(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedStats, stats[0])
}

func TestStatsError(t *testing.T) {
	// If the stats request fails, the API returns the following Error structure: https://docs.podman.io/en/latest/_static/api.html#operation/ContainersStatsAllLibpod
	// For example, if we query the stats with an invalid container ID, the API returns the following message
	statsError := `{"Error":{},"Stats":null}`

	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/stats") {
			_, err := w.Write([]byte(statsError))
			assert.NoError(t, err)
		} else {
			_, err := w.Write([]byte{})
			assert.NoError(t, err)
		}
	}))
	srv.Listener = listener
	srv.Start()
	defer srv.Close()

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		// default timeout
		Timeout: 5 * time.Second,
	}

	cli, err := newLibpodClient(zap.NewNop(), config)
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	stats, err := cli.stats(context.Background(), nil)
	assert.Nil(t, stats)
	assert.EqualError(t, err, errNoStatsFound.Error())
}

func TestList(t *testing.T) {
	// list sample
	listExample := `[{"AutoRemove":false,"Command":["nginx","-g","daemon off;"],"Created":"2022-05-28T11:25:35.999277074+02:00","CreatedAt":"","Exited":false,"ExitedAt":-62135596800,"ExitCode":0,"Id":"aa3e2040dee22a369d2c8f0b712a5ff045e8f1ce47f5e943426ce6664e3ef379","Image":"library/nginxy:latest","ImageID":"12766a6745eea133de9fdcd03ff720fa971fdaf21113d4bc72b417c123b15619","IsInfra":false,"Labels":{"maintainer":"someone"},"Mounts":[],"Names":["sharp_curran"],"Namespaces":{},"Networks":[],"Pid":7892,"Pod":"","PodName":"","Ports":null,"Size":null,"StartedAt":1653729936,"State":"running","Status":""}]`

	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/containers/json") {
			_, err := w.Write([]byte(listExample))
			assert.NoError(t, err)
		} else {
			_, err := w.Write([]byte{})
			assert.NoError(t, err)
		}
	}))
	srv.Listener = listener
	srv.Start()
	defer srv.Close()

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		// default timeout
		Timeout: 5 * time.Second,
	}

	cli, err := newLibpodClient(zap.NewNop(), config)
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	expectedContainer := containerList{
		{
			ID: "aa3e2040dee22a369d2c8f0b712a5ff045e8f1ce47f5e943426ce6664e3ef379",
		},
	}

	containers, err := cli.list(context.Background(), nil)
	assert.NoError(t, err)
	assert.Equal(t, expectedContainer, containers)
}

func TestInspect(t *testing.T) {
	// list sample
	listExample := `{"Id":"d1e70d0b660b122f40f4cf8876f0f0212d6619efbb1d7f1e7b8b1b1922edf51e","Created":"2022-08-27T20:45:14.476846069+03:00","Path":"httpd-foreground","Args":["httpd-foreground"],"State":{"OciVersion":"1.0.2-dev","Status":"running","Running":true,"Paused":false,"Restarting":false,"OOMKilled":false,"Dead":false,"Pid":1147,"ConmonPid":1144,"ExitCode":0,"Error":"","StartedAt":"2022-08-27T20:45:14.545966481+03:00","FinishedAt":"0001-01-01T00:00:00Z","Health":{"Status":"","FailingStreak":0,"Log":null},"CgroupPath":"/user.slice/user-1000.slice/user@1000.service/user.slice/libpod-d1e70d0b660b122f40f4cf8876f0f0212d6619efbb1d7f1e7b8b1b1922edf51e.scope","CheckpointedAt":"0001-01-01T00:00:00Z","RestoredAt":"0001-01-01T00:00:00Z"},"Image":"a981c8992512d65c9b450a9ecabb1cb9d35bb6b03f3640f86471032d5800d825","ImageName":"docker.io/library/httpd:latest","Rootfs":"","Pod":"","ResolvConfPath":"/run/user/1000/containers/overlay-containers/d1e70d0b660b122f40f4cf8876f0f0212d6619efbb1d7f1e7b8b1b1922edf51e/userdata/resolv.conf","HostnamePath":"/run/user/1000/containers/overlay-containers/d1e70d0b660b122f40f4cf8876f0f0212d6619efbb1d7f1e7b8b1b1922edf51e/userdata/hostname","HostsPath":"/run/user/1000/containers/overlay-containers/d1e70d0b660b122f40f4cf8876f0f0212d6619efbb1d7f1e7b8b1b1922edf51e/userdata/hosts","StaticDir":"/home/test/.local/share/containers/storage/overlay-containers/d1e70d0b660b122f40f4cf8876f0f0212d6619efbb1d7f1e7b8b1b1922edf51e/userdata","OCIConfigPath":"/home/test/.local/share/containers/storage/overlay-containers/d1e70d0b660b122f40f4cf8876f0f0212d6619efbb1d7f1e7b8b1b1922edf51e/userdata/config.json","OCIRuntime":"crun","ConmonPidFile":"/run/user/1000/containers/overlay-containers/d1e70d0b660b122f40f4cf8876f0f0212d6619efbb1d7f1e7b8b1b1922edf51e/userdata/conmon.pid","PidFile":"/run/user/1000/containers/overlay-containers/d1e70d0b660b122f40f4cf8876f0f0212d6619efbb1d7f1e7b8b1b1922edf51e/userdata/pidfile","Name":"pedantic_albattani","RestartCount":0,"Driver":"overlay","MountLabel":"system_u:object_r:container_file_t:s0:c186,c799","ProcessLabel":"system_u:system_r:container_t:s0:c186,c799","AppArmorProfile":"","EffectiveCaps":["CAP_CHOWN","CAP_DAC_OVERRIDE","CAP_FOWNER","CAP_FSETID","CAP_KILL","CAP_NET_BIND_SERVICE","CAP_SETFCAP","CAP_SETGID","CAP_SETPCAP","CAP_SETUID","CAP_SYS_CHROOT"],"BoundingCaps":["CAP_CHOWN","CAP_DAC_OVERRIDE","CAP_FOWNER","CAP_FSETID","CAP_KILL","CAP_NET_BIND_SERVICE","CAP_SETFCAP","CAP_SETGID","CAP_SETPCAP","CAP_SETUID","CAP_SYS_CHROOT"],"ExecIDs":[],"GraphDriver":{"Name":"overlay","Data":{"LowerDir":"/home/test/.local/share/containers/storage/overlay/7eee2c97dd0632f3796ea2900ce78ea3f753f4452920b90c583b2154dec981ef/diff:/home/test/.local/share/containers/storage/overlay/d1718bbc0d2e38153e0600e62744e8891159d12b3886dff93231aa508d16aa3d/diff:/home/test/.local/share/containers/storage/overlay/618087d4fdfee2c5bcb985aa520fef400c5bd02e538b2fd60a70583cb00f1d82/diff:/home/test/.local/share/containers/storage/overlay/6f32ad5bfd7e339cd6f0460664fb95655c8d0c00a784128c1c743ba81537144c/diff:/home/test/.local/share/containers/storage/overlay/6485bed636274e42b47028c43ad5f9c036dd7cf2b40194bd556ddad2a98eea63/diff","MergedDir":"/home/test/.local/share/containers/storage/overlay/4dd4e40606d88c144e27ef1c2363b8f5ab6081b4e4f32201a0599d41c6a5d3bd/merged","UpperDir":"/home/test/.local/share/containers/storage/overlay/4dd4e40606d88c144e27ef1c2363b8f5ab6081b4e4f32201a0599d41c6a5d3bd/diff","WorkDir":"/home/test/.local/share/containers/storage/overlay/4dd4e40606d88c144e27ef1c2363b8f5ab6081b4e4f32201a0599d41c6a5d3bd/work"}},"Mounts":[],"Dependencies":[],"NetworkSettings":{"EndpointID":"","Gateway":"","IPAddress":"","IPPrefixLen":0,"IPv6Gateway":"","GlobalIPv6Address":"","GlobalIPv6PrefixLen":0,"MacAddress":"","Bridge":"","SandboxID":"","HairpinMode":false,"LinkLocalIPv6Address":"","LinkLocalIPv6PrefixLen":0,"Ports":{"80/tcp":[{"HostIp":"","HostPort":"8080"}]},"SandboxKey":"/run/user/1000/netns/netns-d7fc8056-726c-b01c-3fab-a2a3bdbae7b4"},"Namespace":"","IsInfra":false,"Config":{"Hostname":"d1e70d0b660b","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":true,"OpenStdin":false,"StdinOnce":false,"Env":["PATH=/usr/local/apache2/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin","container=podman","MY_ENV_1=env_metric_value_1","TERM=xterm","HTTPD_VERSION=2.4.54","HTTPD_SHA256=eb397feeefccaf254f8d45de3768d9d68e8e73851c49afd5b7176d1ecf80c340","HTTPD_PATCHES=","HTTPD_PREFIX=/usr/local/apache2","MY_ENV_2=env_metric_value_2","HOME=/root","HOSTNAME=d1e70d0b660b"],"Cmd":["httpd-foreground"],"Image":"docker.io/library/httpd:latest","Volumes":null,"WorkingDir":"/usr/local/apache2","Entrypoint":"","OnBuild":null,"Labels":{"label_1":"label_metric_value_1","label_2":"label_metric_value_2"},"Annotations":{"io.container.manager":"libpod","io.kubernetes.cri-o.Created":"2022-08-27T20:45:14.476846069+03:00","io.kubernetes.cri-o.TTY":"true","io.podman.annotations.autoremove":"FALSE","io.podman.annotations.init":"FALSE","io.podman.annotations.privileged":"FALSE","io.podman.annotations.publish-all":"FALSE","org.opencontainers.image.stopSignal":"28"},"StopSignal":28,"CreateCommand":["podman","run","-dt","-p","8080:80/tcp","-l","label_1=label_metric_value_1","-l","label_2=label_metric_value_2","--env","MY_ENV_1=env_metric_value_1","--env","MY_ENV_2=env_metric_value_2","docker.io/library/httpd"],"Umask":"0022","Timeout":0,"StopTimeout":10,"Passwd":true},"HostConfig":{"Binds":[],"CgroupManager":"systemd","CgroupMode":"private","ContainerIDFile":"","LogConfig":{"Type":"journald","Config":null,"Path":"","Tag":"","Size":"0B"},"NetworkMode":"slirp4netns","PortBindings":{"80/tcp":[{"HostIp":"","HostPort":"8080"}]},"RestartPolicy":{"Name":"","MaximumRetryCount":0},"AutoRemove":false,"VolumeDriver":"","VolumesFrom":null,"CapAdd":[],"CapDrop":["CAP_AUDIT_WRITE","CAP_MKNOD","CAP_NET_RAW"],"Dns":[],"DnsOptions":[],"DnsSearch":[],"ExtraHosts":[],"GroupAdd":[],"IpcMode":"private","Cgroup":"","Cgroups":"default","Links":null,"OomScoreAdj":0,"PidMode":"private","Privileged":false,"PublishAllPorts":false,"ReadonlyRootfs":false,"SecurityOpt":[],"Tmpfs":{},"UTSMode":"private","UsernsMode":"","ShmSize":65536000,"Runtime":"oci","ConsoleSize":[0,0],"Isolation":"","CpuShares":0,"Memory":0,"NanoCpus":0,"CgroupParent":"user.slice","BlkioWeight":0,"BlkioWeightDevice":null,"BlkioDeviceReadBps":null,"BlkioDeviceWriteBps":null,"BlkioDeviceReadIOps":null,"BlkioDeviceWriteIOps":null,"CpuPeriod":0,"CpuQuota":0,"CpuRealtimePeriod":0,"CpuRealtimeRuntime":0,"CpusetCpus":"","CpusetMems":"","Devices":[],"DiskQuota":0,"KernelMemory":0,"MemoryReservation":0,"MemorySwap":0,"MemorySwappiness":0,"OomKillDisable":false,"PidsLimit":2048,"Ulimits":[],"CpuCount":0,"CpuPercent":0,"IOMaximumIOps":0,"IOMaximumBandwidth":0,"CgroupConf":null}}`

	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/containers/d1e70d0b660b122f40f4cf8876f0f0212d6619efbb1d7f1e7b8b1b1922edf51e/json") {
			_, err := w.Write([]byte(listExample))
			assert.NoError(t, err)
		} else {
			_, err := w.Write([]byte{})
			assert.NoError(t, err)
		}
	}))
	srv.Listener = listener
	srv.Start()
	defer srv.Close()

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		// default timeout
		Timeout: 5 * time.Second,
	}

	cli, err := newLibpodClient(zap.NewNop(), config)
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	expectedContainer := container{
		ID: "d1e70d0b660b122f40f4cf8876f0f0212d6619efbb1d7f1e7b8b1b1922edf51e",
		Config: containerConfig{
			Env: map[string]string{
				"HOME":          "/root",
				"HOSTNAME":      "d1e70d0b660b",
				"HTTPD_PATCHES": "",
				"HTTPD_PREFIX":  "/usr/local/apache2",
				"HTTPD_SHA256":  "eb397feeefccaf254f8d45de3768d9d68e8e73851c49afd5b7176d1ecf80c340",
				"HTTPD_VERSION": "2.4.54",
				"MY_ENV_1":      "env_metric_value_1",
				"MY_ENV_2":      "env_metric_value_2",
				"PATH":          "/usr/local/apache2/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"TERM":          "xterm",
				"container":     "podman",
			},
			Image: "docker.io/library/httpd:latest",
			Labels: map[string]string{
				"label_1": "label_metric_value_1",
				"label_2": "label_metric_value_2",
			},
		},
	}

	containerDto, err := cli.inspect(context.Background(), "d1e70d0b660b122f40f4cf8876f0f0212d6619efbb1d7f1e7b8b1b1922edf51e")
	assert.NoError(t, err)
	assert.Equal(t, expectedContainer, containerDto)
}

func TestEvents(t *testing.T) {
	// event samples
	eventsExample := []string{
		`{"status":"start","id":"49a4c52afb06e6b36b2941422a0adf47421dbfbf40503dbe17bd56b4570b6681","from":"docker.io/library/httpd:latest","Type":"container","Action":"start","Actor":{"ID":"49a4c52afb06e6b36b2941422a0adf47421dbfbf40503dbe17bd56b4570b6681","Attributes":{"containerExitCode":"0","image":"docker.io/library/httpd:latest","name":"vigilant_jennings"}},"scope":"local","time":1655230086,"timeNano":1655230086294801585}`,
		`{"status":"died","id":"d5c43c6954e4bfe62170c75f9f18f81da644bd35bfd22dbfafda349192d4940a","from":"docker.io/library/nginx:latest","Type":"container","Action":"died","Actor":{"ID":"d5c43c6954e4bfe62170c75f9f18f81da644bd35bfd22dbfafda349192d4940a","Attributes":{"containerExitCode":"0","image":"docker.io/library/nginx:latest","name":"relaxed_mccarthy"}},"scope":"local","time":1655653026,"timeNano":1655653026340832435}`,
	}

	listener, addr := tmpSock(t)
	defer listener.Close()
	defer os.Remove(addr)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "events") {
			// write stream events
			for _, event := range eventsExample {
				_, err := w.Write([]byte(event))
				assert.NoError(t, err)
			}
			// empty write to generate io.EOF error
			_, err := w.Write([]byte{})
			assert.NoError(t, err)
		} else {
			_, err := w.Write([]byte{})
			assert.NoError(t, err)
		}
	}))
	srv.Listener = listener
	srv.Start()
	defer srv.Close()

	config := &Config{
		Endpoint: fmt.Sprintf("unix://%s", addr),
		// default timeout
		Timeout: 5 * time.Second,
	}

	cli, err := newLibpodClient(zap.NewNop(), config)
	assert.NotNil(t, cli)
	assert.Nil(t, err)

	expectedEvents := []event{
		{ID: "49a4c52afb06e6b36b2941422a0adf47421dbfbf40503dbe17bd56b4570b6681", Status: "start"},
		{ID: "d5c43c6954e4bfe62170c75f9f18f81da644bd35bfd22dbfafda349192d4940a", Status: "died"},
	}

	events, errs := cli.events(context.Background(), nil)
	var actualEvents []event

loop:
	for {

		select {
		case err := <-errs:
			if err != nil && !errors.Is(err, io.EOF) {
				t.Fatal(err)
			}
			assert.Equal(t, io.EOF, err)
			break loop
		case e := <-events:
			actualEvents = append(actualEvents, e)
		}
	}

	assert.Equal(t, expectedEvents, actualEvents)

}
