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
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
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
	LogLevel   *string
	ConfigPath *string
	Action     *string
	Command    *string
}

type masterServers struct {
	NamePattern       string
	ServerType        string
	Image             string
	Labels            map[string]string
	WaitTimeInRetry   time.Duration
	RetryTimeLimit    int
	ServersInitParams masterServersInitParams
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
	HetznerToken       string             `yaml:"hetznerToken"`
	IPRange            string             `yaml:"ipRange"`
	SSHPrivateKey      string             `yaml:"sshPrivateKey"`
	SSHPublicKey       string             `yaml:"sshPublicKey"`
	MasterCount        int                `yaml:"masterCount"`
	NetworkZone        hcloud.NetworkZone `yaml:"networkZone"`
	Location           string             `yaml:"location"`
	Datacenter         string             `yaml:"datacenter"`
	MasterServers      masterServers      `yaml:"masterServers"`
	MasterLoadBalancer masterLoadBalancer `yaml:"masterLoadBalancer"`
	CliArgs            cliArgs            `yaml:"cliArgs"`
	DeploymentsConfig  interface{}        `yaml:"deploymentsConfig"` // values.yaml in chart
}

//nolint:gochecknoglobals
var cliArguments = cliArgs{
	LogLevel:   flag.String("log.level", "INFO", "logging level"),
	ConfigPath: flag.String("config", envDefault("CONFIG", "config.yaml"), "config path"),
	Action:     flag.String("action", "", "create|delete|list-configurations|patch-cluster|adhoc"),
	Command:    flag.String("command", "", "command to adhoc action"),
}

func defaultConfig() Type {
	privateKey := "~/.ssh/id_rsa"
	kubeConfigPath := "~/.kube/hcloud"

	serverLabels := make(map[string]string)
	serverLabels["role"] = "master"

	return Type{
		HetznerToken:   os.Getenv("HCLOUD_TOKEN"),
		ClusterName:    "k8s",
		NetworkZone:    hcloud.NetworkZoneEUCentral,
		Location:       defaultLocation,
		Datacenter:     defaultDatacenter,
		KubeConfigPath: kubeConfigPath,
		SSHPrivateKey:  privateKey,
		SSHPublicKey:   fmt.Sprintf("%s.pub", privateKey),
		MasterCount:    masterServersCount,
		CliArgs:        cliArguments,
		MasterServers: masterServers{
			NamePattern:     "master-%d",
			ServerType:      "cx21",
			Image:           "ubuntu-20.04",
			Labels:          serverLabels,
			WaitTimeInRetry: waitTimeInRetry,
			RetryTimeLimit:  retryTimeLimit,
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
		DeploymentsConfig: emptyStruct{},
	}
}

func Load() error {
	configByte, err := ioutil.ReadFile(*cliArguments.ConfigPath)
	if err != nil {
		return err
	}

	config = defaultConfig()

	if len(config.HetznerToken) == 0 {
		auth, err := ioutil.ReadFile(".hcloudauth")
		if err != nil {
			log.Debug(err)
		} else {
			config.HetznerToken = string(auth)
		}
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
	e := os.Getenv(name)
	if len(e) > 0 {
		return e
	}

	return defaultValue
}
