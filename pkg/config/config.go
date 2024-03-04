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
	"bytes"
	"flag"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/pkg/errors"
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
	SaveConfigPath             *string
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
	Labels             map[string]string
	WaitTimeInRetry    time.Duration
	RetryTimeLimit     int
	ServersInitParams  masterServersInitParams
}

type emptyStruct struct{}

type masterLoadBalancer struct {
	LoadBalancerType string
	ListenPort       int
	DestinationPort  int
}

type serverComponentContainerd struct {
	Version        string
	PauseContainer string
}

type serverComponentDocker struct {
	Version string
}
type serverComponentKubernetes struct {
	Version string
}
type serverComponentUbuntu struct {
	Version      string
	UserName     string
	Architecture hcloud.Architecture
}
type serverComponents struct {
	Ubuntu     serverComponentUbuntu
	Kubernetes serverComponentKubernetes
	Docker     serverComponentDocker
	Containerd serverComponentContainerd
}

type clusterAutoscalingGroup struct {
	Name    string `yaml:"name"`
	MinSize int    `yaml:"minSize"`
	MaxSize int    `yaml:"maxSize"`
}

func getDefaultMasterServersInitParams(branch string) masterServersInitParams {
	return masterServersInitParams{
		TarGz:  fmt.Sprintf("https://github.com/maksim-paskal/hcloud-k8s-ctl/archive/refs/heads/%s.tar.gz", branch),
		Folder: "hcloud-k8s-ctl-" + branch,
	}
}

func getDefaultClusterAutoscaler() map[interface{}]interface{} {
	result := make([]*clusterAutoscalingGroup, 0)

	defaultLocations := []string{
		hcloudLocationEUFalkenstein,
		hcloudLocationEUNuremberg,
		hcloudLocationEUHelsinki,
	}
	defaultServers := strings.Split(defaultAutoscalerInstances, ",")

	for _, location := range defaultLocations {
		for _, server := range defaultServers {
			result = append(result, &clusterAutoscalingGroup{
				Name: fmt.Sprintf(
					"%s:%s:%s-%s",
					strings.ToUpper(server),
					strings.ToUpper(location),
					server,
					location,
				),
				MinSize: 0,
				MaxSize: workersCount,
			})
		}
	}

	return map[interface{}]interface{}{
		"autoscalingGroups": result,
	}
}

//nolint:gochecknoglobals
var config = Type{}

type Type struct {
	ClusterName        string             `yaml:"clusterName"`
	KubeConfigPath     string             `yaml:"kubeConfigPath"`
	HetznerToken       string             `yaml:"hetznerToken"`
	ServerComponents   serverComponents   `yaml:"serverComponents"`
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
	PreStartScript     string             `yaml:"preStartScript"`
	PostStartScript    string             `yaml:"postStartScript"`

	// helm dependencies values.yaml
	ClusterAutoscaler            map[interface{}]interface{} `yaml:"cluster-autoscaler,omitempty"`              //nolint:tagliatelle,lll
	NfsSubdirExternalProvisioner map[interface{}]interface{} `yaml:"nfs-subdir-external-provisioner,omitempty"` //nolint:tagliatelle,lll
	KubeletCSRApprover           map[interface{}]interface{} `yaml:"kubelet-csr-approver,omitempty"`            //nolint:tagliatelle,lll
	HCloudCSI                    map[interface{}]interface{} `yaml:"hcloud-csi,omitempty"`                      //nolint:tagliatelle,lll
	HCloudCCM                    map[interface{}]interface{} `yaml:"hcloud-cloud-controller-manager,omitempty"` //nolint:tagliatelle,lll
	MetricsServer                map[interface{}]interface{} `yaml:"metrics-server,omitempty"`                  //nolint:tagliatelle,lll
	Flannel                      map[interface{}]interface{} `yaml:"flannel,omitempty"`                         //nolint:tagliatelle,lll
}

//nolint:gochecknoglobals
var cliArguments = cliArgs{
	LogLevel:                   flag.String("log.level", "INFO", "logging level"),
	SaveConfigPath:             flag.String("save-config-path", "", "save full config path"),
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

func SetBranch(branch string) {
	config.MasterServers.ServersInitParams = getDefaultMasterServersInitParams(branch)
}

func defaultConfig() Type {
	privateKey := "~/.ssh/id_rsa"
	kubeConfigPath := "~/.kube/hcloud"

	serverLabels := make(map[string]string)
	serverLabels["role"] = "master"

	return Type{
		HetznerToken: os.Getenv("HCLOUD_TOKEN"),
		ServerComponents: serverComponents{
			Ubuntu: serverComponentUbuntu{
				Version:      "ubuntu-20.04",
				UserName:     "hcloud-user",
				Architecture: hcloud.ArchitectureX86, // x86 or arm
			},
			Kubernetes: serverComponentKubernetes{
				Version: "1.28.2",
			},
			Docker: serverComponentDocker{
				Version: "5:24.0.6-1~ubuntu.20.04~focal",
			},
			Containerd: serverComponentContainerd{
				Version:        "1.6.24-1",
				PauseContainer: "registry.k8s.io/pause:3.2",
			},
		},
		ClusterName:    "k8s",
		IPRange:        "10.0.0.0/16",
		IPRangeSubnet:  "",
		NetworkZone:    hcloud.NetworkZoneEUCentral,
		Location:       defaultLocation,
		Datacenter:     defaultDatacenter,
		KubeConfigPath: kubeConfigPath,
		SSHPrivateKey:  privateKey,
		SSHPublicKey:   privateKey + ".pub",
		MasterCount:    masterServersCount,
		CliArgs:        cliArguments,
		MasterServers: masterServers{
			NamePattern:        "master-%d",
			PlacementGroupName: "master-placement-group",
			ServerType:         "cx21",
			Labels:             serverLabels,
			WaitTimeInRetry:    waitTimeInRetry,
			RetryTimeLimit:     retryTimeLimit,
			ServersInitParams:  getDefaultMasterServersInitParams("main"),
		},
		MasterLoadBalancer: masterLoadBalancer{
			LoadBalancerType: "lb11",
			ListenPort:       loadBalancerDefaultPort,
			DestinationPort:  loadBalancerDefaultPort,
		},
		Deployments:       emptyStruct{},
		ClusterAutoscaler: getDefaultClusterAutoscaler(),
	}
}

func Load() error { //nolint: cyclop
	configByte, err := os.ReadFile(*cliArguments.ConfigPath)
	if err != nil {
		return errors.Wrap(err, "failed to read config file")
	}

	config = defaultConfig()

	if len(config.HetznerToken) == 0 {
		auth, err := os.ReadFile(".hcloudauth")
		if err != nil {
			log.Debug(err)
		} else {
			config.HetznerToken = string(auth)
		}
	}

	if len(config.IPRangeSubnet) == 0 {
		config.IPRangeSubnet = config.IPRange
	}

	err = yaml.Unmarshal(configByte, &config)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal config file")
	}

	config.KubeConfigPath, err = expand(config.KubeConfigPath)
	if err != nil {
		return errors.Wrap(err, "failed to expand kube config path")
	}

	config.SSHPrivateKey, err = expand(config.SSHPrivateKey)
	if err != nil {
		return errors.Wrap(err, "failed to expand ssh private key path")
	}

	config.SSHPublicKey, err = expand(config.SSHPublicKey)
	if err != nil {
		return errors.Wrap(err, "failed to expand ssh public key path")
	}

	_, _, err = net.ParseCIDR(config.IPRange)
	if err != nil {
		return errors.Wrap(err, "failed to parse ip range")
	}

	_, _, err = net.ParseCIDR(config.IPRangeSubnet)
	if err != nil {
		return errors.Wrap(err, "failed to parse ip range subnet")
	}

	return nil
}

func Check() error {
	if len(config.HetznerToken) == 0 {
		return errNoHetznerToken
	}

	return nil
}

func Get() *Type {
	return &config
}

func hideSensitiveData(out []byte, sensitive string) []byte {
	if len(sensitive) == 0 {
		return out
	}

	return bytes.ReplaceAll(out, []byte(sensitive), []byte(secretString))
}

func String() string {
	var b bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&b)
	yamlEncoder.SetIndent(2) //nolint:gomnd

	if err := yamlEncoder.Encode(&config); err != nil {
		return fmt.Sprintf("ERROR %s", err)
	}

	out := b.Bytes()
	// remove sensitive data
	out = hideSensitiveData(out, config.HetznerToken)

	return string(out)
}

func expand(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", errors.Wrap(err, "failed to get current user")
	}

	return filepath.Join(usr.HomeDir, path[1:]), nil
}

func envDefault(name string, defaultValue string) string {
	if e := os.Getenv(name); len(e) > 0 {
		return e
	}

	return defaultValue
}

func SaveConfig(filePath string) error {
	const configPermissions = 0o600

	re := regexp.MustCompile("(?m)[\r\n]+^.*(kubeConfigPath|hetznerToken|sshPrivateKey|sshPublicKey).*$")

	content := re.ReplaceAllString(String(), "")

	err := os.WriteFile(filePath, []byte(content), configPermissions)
	if err != nil {
		return errors.Wrap(err, "failed to write config file")
	}

	return nil
}
