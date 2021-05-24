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

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type cliArgs struct {
	LogLevel             *string
	ConfigPath           *string
	Action               *string
	DeleteBeforeCreation *bool
}

type masterServers struct {
	NamePattern     string
	ServerType      string
	Image           string
	Datacenter      string
	Labels          map[string]string
	WaitTimeInRetry time.Duration
	RetryTimeLimit  int
}

type masterLoadBalancer struct {
	LoadBalancerType string
	Location         string
	ListenPort       int
	DestinationPort  int
	Selector         string
}

type Type struct {
	ClusterName        string             `yaml:"clusterName"`
	KubeConfigPath     string             `yaml:"kubeConfigPath"`
	HetznerToken       string             `yaml:"hetznerToken"`
	IPRange            string             `yaml:"ipRange"`
	IPRangeNet         *net.IPNet         `yaml:"ipRangeNet"`
	SSHPrivateKey      string             `yaml:"sshPrivateKey"`
	SSHPublicKey       string             `yaml:"sshPublicKey"`
	MasterCount        int                `yaml:"masterCount"`
	MasterServers      masterServers      `yaml:"masterServers"`
	MasterLoadBalancer masterLoadBalancer `yaml:"masterLoadBalancer"`
	CliArgs            cliArgs            `yaml:"cliArgs"`
}

type ApplicationConfig struct {
	config Type
}

//nolint:gochecknoglobals
var cliArguments = cliArgs{
	LogLevel:             flag.String("log.level", "INFO", "logging level"),
	ConfigPath:           flag.String("config", "config.yaml", "config path"),
	Action:               flag.String("action", "create", "create|delete"),
	DeleteBeforeCreation: flag.Bool("deleteBeforeCreation", false, "delete cluster before creation"),
}

func NewApplicationConfig() *ApplicationConfig {
	return &ApplicationConfig{}
}

func (c *ApplicationConfig) defaultConfig() Type {
	privateKey := "~/.ssh/id_rsa"
	kubeConfigPath := "~/.kube/hcloud"

	serverLabels := make(map[string]string)
	serverLabels["role"] = "master"

	return Type{
		HetznerToken:   os.Getenv("HCLOUD_TOKEN"),
		ClusterName:    "k8s",
		KubeConfigPath: kubeConfigPath,
		SSHPrivateKey:  privateKey,
		SSHPublicKey:   fmt.Sprintf("%s.pub", privateKey),
		MasterCount:    masterServersCount,
		CliArgs:        cliArguments,
		MasterServers: masterServers{
			NamePattern:     "master-%d",
			ServerType:      "cx21",
			Image:           "ubuntu-20.04",
			Datacenter:      "fsn1-dc14",
			Labels:          serverLabels,
			WaitTimeInRetry: waitTimeInRetry,
			RetryTimeLimit:  retryTimeLimit,
		},
		MasterLoadBalancer: masterLoadBalancer{
			LoadBalancerType: "lb11",
			Location:         "fsn1",
			ListenPort:       loadBalancerDefaultPort,
			DestinationPort:  loadBalancerDefaultPort,
			Selector:         "role=master",
		},
	}
}

func (c *ApplicationConfig) Load() error {
	configByte, err := ioutil.ReadFile(*cliArguments.ConfigPath)
	if err != nil {
		return err
	}

	c.config = c.defaultConfig()

	if len(c.config.HetznerToken) == 0 {
		auth, err := ioutil.ReadFile(".hcloudauth")
		if err != nil {
			log.Debug(err)
		} else {
			c.config.HetznerToken = string(auth)
		}
	}

	err = yaml.Unmarshal(configByte, &c.config)
	if err != nil {
		return err
	}

	c.config.KubeConfigPath, err = c.expand(c.config.KubeConfigPath)
	if err != nil {
		return err
	}

	c.config.SSHPrivateKey, err = c.expand(c.config.SSHPrivateKey)
	if err != nil {
		return err
	}

	c.config.SSHPublicKey, err = c.expand(c.config.SSHPublicKey)
	if err != nil {
		return err
	}

	_, c.config.IPRangeNet, err = net.ParseCIDR(c.config.IPRange)
	if err != nil {
		return err
	}

	return nil
}

func (c *ApplicationConfig) Get() Type {
	return c.config
}

func (c *ApplicationConfig) String() string {
	out, err := yaml.Marshal(c.config)
	if err != nil {
		return fmt.Sprintf("ERROR: %t", err)
	}

	return string(out)
}

func (c *ApplicationConfig) expand(path string) (string, error) {
	if len(path) == 0 || path[0] != '~' {
		return path, nil
	}

	usr, err := user.Current()
	if err != nil {
		return "", err
	}

	return filepath.Join(usr.HomeDir, path[1:]), nil
}
