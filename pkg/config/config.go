/*
Copyright paskal.maksim@gmail.com
Licensed under the Apache License, Version 2.0 (the "License")
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package config

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/hetznercloud/hcloud-go/hcloud"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type masterServersInitParams struct {
	TarGz  string
	Folder string
}

type cliArgs struct {
	LogLevel                   *string
	ConfigPath                 *string
	Action                     *string
	AdhocCommand               *string
	AdhocCopyNewFile           *bool
	AdhocMasters               *bool
	AdhocWorkers               *bool
	AdhocUser                  *string
	UpgradeControlPlaneVersion *string
	CreateFirewallControlPlane *bool
	CreateFirewallWorkers      *bool
}

type masterServers struct {
	NamePattern        string
	PlacementGroupName string
	ServerType         string
	Image              string
	Labels             map[string]string
	WaitTimeInRetry    time.Duration
	RetryTimeLimit     int
	ServersInitParams  masterServersInitParams
}

type autoscalerWorker struct {
	Location string
	Min      int
	Max      int
	Type     []string
}

type autoscalerDefaults struct {
	Min int
	Max int
}
type autoscaler struct {
	Enabled  bool
	Args     []string
	Defaults autoscalerDefaults
	Workers  []autoscalerWorker
}

type hetznerToken struct {
	Main string
	Csi  string
	Ccm  string
}

type emptyStruct struct{}

type masterLoadBalancer struct {
	LoadBalancerType string
	ListenPort       int
	DestinationPort  int
}

//nolint:gochecknoglobals
var config = Type{}

type Type struct {
	ClusterName        string             `yaml:"clusterName"`
	KubeConfigPath     string             `yaml:"kubeConfigPath"`
	HetznerToken       hetznerToken       `yaml:"hetznerToken"`
	IPRange            string             `yaml:"ipRange"`
	IPRangeSubnet      string             `yaml:"ipRangeSubnet"`
	SSHPrivateKey      string             `yaml:"sshPrivateKey"`
	SSHPublicKey       string             `yaml:"sshPublicKey"`
	MasterCount        int                `yaml:"masterCount"`
	NetworkZone        hcloud.NetworkZone `yaml:"networkZone"`
	Location           string             `yaml:"location"`
	Datacenter         string             `yaml:"datacenter"`
	MasterServers      masterServers      `yaml:"masterServers"`
	MasterLoadBalancer masterLoadBalancer `yaml:"masterLoadBalancer"`
	CliArgs            cliArgs            `yaml:"cliArgs"`
	Deployments        interface{}        `yaml:"deployments"` // values.yaml in chart
	Autoscaler         autoscaler         `yaml:"autoscaler"`
	PreStartScript     string             `yaml:"preStartScript"`
	PostStartScript    string             `yaml:"postStartScript"`
}

//nolint:gochecknoglobals
var cliArguments = cliArgs{
	LogLevel:                   flag.String("log.level", "INFO", "logging level"),
	ConfigPath:                 flag.String("config", envDefault("CONFIG", "config.yaml"), "config path"),
	Action:                     flag.String("action", "", "create|delete|list-configurations|patch-cluster|adhoc|upgrade-controlplane|create-firewall"), //nolint:lll
	AdhocCommand:               flag.String("adhoc.command", "", "command to adhoc action"),
	AdhocCopyNewFile:           flag.Bool("adhoc.copynewfile", false, "copy new files to adhoc action"),
	AdhocMasters:               flag.Bool("adhoc.master", false, "run adhoc also on master servers"),
	AdhocWorkers:               flag.Bool("adhoc.workers", true, "run adhoc also on workers servers"),
	AdhocUser:                  flag.String("adhoc.user", "", "ssh user for adhoc action"),
	UpgradeControlPlaneVersion: flag.String("upgrade-controlplane.version", "", "controlplane version to upgrade"),
	CreateFirewallControlPlane: flag.Bool("create-firewall.controlplane", false, "create firewall for controlplane"),
	CreateFirewallWorkers:      flag.Bool("create-firewall.workers", false, "create firewall for workers"),
}

func defaultConfig() Type { //nolint:funlen
	privateKey := "~/.ssh/id_rsa"
	kubeConfigPath := "~/.kube/hcloud"

	serverLabels := make(map[string]string)
	serverLabels["role"] = "master"

	return Type{
		HetznerToken: hetznerToken{
			Main: os.Getenv("HCLOUD_TOKEN"),
		},
		ClusterName:    "k8s",
		IPRange:        "10.0.0.0/16",
		IPRangeSubnet:  "",
		NetworkZone:    hcloud.NetworkZoneEUCentral,
		Location:       defaultLocation,
		Datacenter:     defaultDatacenter,
		KubeConfigPath: kubeConfigPath,
		SSHPrivateKey:  privateKey,
		SSHPublicKey:   fmt.Sprintf("%s.pub", privateKey),
		MasterCount:    masterServersCount,
		CliArgs:        cliArguments,
		MasterServers: masterServers{
			NamePattern:        "master-%d",
			PlacementGroupName: "master-placement-group",
			ServerType:         "cx21",
			Image:              "ubuntu-20.04",
			Labels:             serverLabels,
			WaitTimeInRetry:    waitTimeInRetry,
			RetryTimeLimit:     retryTimeLimit,
			ServersInitParams: masterServersInitParams{
				TarGz:  "https://github.com/maksim-paskal/hcloud-k8s-ctl/archive/refs/heads/main.tar.gz",
				Folder: "hcloud-k8s-ctl-main",
			},
		},
		MasterLoadBalancer: masterLoadBalancer{
			LoadBalancerType: "lb11",
			ListenPort:       loadBalancerDefaultPort,
			DestinationPort:  loadBalancerDefaultPort,
		},
		Deployments: emptyStruct{},
		Autoscaler: autoscaler{
			Enabled: true,
			Args: []string{
				"--v=4",
				"--cloud-provider=hetzner",
				"--stderrthreshold=info",
				"--expander=least-waste",
				"--scale-down-enabled=true",
				"--skip-nodes-with-local-storage=false",
				"--skip-nodes-with-system-pods=false",
				"--scale-down-utilization-threshold=0.8",
			},
			Defaults: autoscalerDefaults{
				Min: 0,
				Max: workersCount,
			},
			Workers: []autoscalerWorker{
				{
					Location: hcloudLocationEUFalkenstein,
					Min:      0,
					Max:      0,
					Type:     strings.Split(defaultAutoscalerInstances, ","),
				},
				{
					Location: hcloudLocationEUNuremberg,
					Min:      0,
					Max:      0,
					Type:     strings.Split(defaultAutoscalerInstances, ","),
				},
				{
					Location: hcloudLocationEUHelsinki,
					Min:      0,
					Max:      0,
					Type:     strings.Split(defaultAutoscalerInstances, ","),
				},
			},
		},
	}
}

func Load() error { //nolint: cyclop
	configByte, err := os.ReadFile(*cliArguments.ConfigPath)
	if err != nil {
		return err
	}

	config = defaultConfig()

	if len(config.HetznerToken.Main) == 0 {
		auth, err := os.ReadFile(".hcloudauth")
		if err != nil {
			log.Debug(err)
		} else {
			config.HetznerToken.Main = string(auth)
		}
	}

	if len(config.IPRangeSubnet) == 0 {
		config.IPRangeSubnet = config.IPRange
	}

	err = yaml.Unmarshal(configByte, &config)
	if err != nil {
		return err
	}

	config.KubeConfigPath, err = expand(config.KubeConfigPath)
	if err != nil {
		return err
	}

	config.SSHPrivateKey, err = expand(config.SSHPrivateKey)
	if err != nil {
		return err
	}

	config.SSHPublicKey, err = expand(config.SSHPublicKey)
	if err != nil {
		return err
	}

	_, _, err = net.ParseCIDR(config.IPRange)
	if err != nil {
		return err
	}

	_, _, err = net.ParseCIDR(config.IPRangeSubnet)
	if err != nil {
		return err
	}

	return nil
}

func Check() error {
	if len(config.HetznerToken.Main) == 0 {
		return errNoHetznerToken
	}

	return nil
}

func Get() *Type {
	return &config
}

func String() string {
	out, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Sprintf("ERROR: %t", err)
	}

	return string(out)
}

func expand(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	return filepath.Join(usr.HomeDir, path[1:]), nil
}

func envDefault(name string, defaultValue string) string {
	if e := os.Getenv(name); len(e) > 0 {
		return e
	}

	return defaultValue
}
