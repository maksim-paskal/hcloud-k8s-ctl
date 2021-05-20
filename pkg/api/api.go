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
package api

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/maksim-paskal/hcloud-k8s-ctl/pkg/config"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type ApplicationAPI struct {
	config            *config.ApplicationConfig
	hcloudClient      *hcloud.Client
	ctx               context.Context
	masterClusterJoin string
	clusterKubeConfig string
}

func NewApplicationAPI(config *config.ApplicationConfig) *ApplicationAPI {
	api := ApplicationAPI{
		ctx:          context.Background(),
		config:       config,
		hcloudClient: hcloud.NewClient(hcloud.WithToken(config.Get().HetznerToken)),
	}

	return &api
}

func (api *ApplicationAPI) getCommonInstallCommand() string {
	return commonExecCommand + `

/root/scripts/common-install.sh

`
}

func (api *ApplicationAPI) getInitMasterCommand(loadBalancerIP string) string {
	return commonExecCommand + `

export HCLOUD_TOKEN=` + api.config.Get().HetznerToken + `
export MASTER_LB=` + loadBalancerIP + `

/root/scripts/init-master.sh
`
}

func (api *ApplicationAPI) waitForLoadBalancer(loadBalancerName string) (string, error) {
	loadBalancer, _, err := api.hcloudClient.LoadBalancer.Get(api.ctx, loadBalancerName)
	if err != nil {
		return "", errors.Wrap(err, "error in loadBalancer get")
	}

	if loadBalancer == nil {
		return "", errors.Wrap(err, "loadBalancer is nil")
	}

	loadBalancerIP, err := loadBalancer.PublicNet.IPv4.IP.MarshalText()
	if err != nil {
		return "", errors.Wrap(err, "loadBalancer IP get")
	}

	if loadBalancerIP == nil {
		return "", errors.Wrap(err, "loadBalancerIP is nil")
	}

	return string(loadBalancerIP), nil
}

func (api *ApplicationAPI) waitForServer(server string) (string, error) {
	masterServer, _, err := api.hcloudClient.Server.Get(api.ctx, server)
	if err != nil {
		return "", errors.Wrap(err, "error in server get")
	}

	if masterServer == nil {
		return "", errors.Wrap(err, "masterServer is null")
	}

	serverIP, err := masterServer.PublicNet.IPv4.IP.MarshalText()
	if err != nil {
		return "", errors.Wrap(err, "masterServer ip get")
	}

	if serverIP == nil {
		return "", errors.Wrap(err, "serverIP ip null")
	}

	_, _, err = api.execCommand(string(serverIP), "date")
	if err != nil {
		return "", errors.Wrap(err, "error executing command")
	}

	return string(serverIP), nil
}

func (api *ApplicationAPI) joinToMasterNodes(server string) error {
	log.Infof("Join %s server to master nodes...", server)

	// join to cluster
	retryCount := 0

	for {
		if retryCount > api.config.Get().MasterServers.RetryTimeLimit {
			return errRetryLimitReached
		}

		if retryCount > 0 {
			time.Sleep(api.config.Get().MasterServers.WaitTimeInRetry)
		}
		retryCount++

		log.Infof("Waiting for master node... try=%d", retryCount)

		serverIP, err := api.waitForServer(server)
		if err != nil {
			log.WithError(err).Error()

			continue
		}

		stdout, stderr, err := api.execCommand(serverIP, api.masterClusterJoin)
		if err != nil {
			log.WithError(err).Error(stderr)

			continue
		}

		log.Debugf("stdout=%s", stdout)
		log.Debugf("stderr=%s", stderr)

		break
	}

	return nil
}

func (api *ApplicationAPI) initFirstMasterNode() error {
	log.Info("Init first master node...")

	// make first node
	retryCount := 0

	for {
		if retryCount > api.config.Get().MasterServers.RetryTimeLimit {
			return errRetryLimitReached
		}

		if retryCount > 0 {
			time.Sleep(api.config.Get().MasterServers.WaitTimeInRetry)
		}
		retryCount++

		log.Infof("Waiting for master node... try=%d", retryCount)

		serverIP, err := api.waitForServer(fmt.Sprintf(api.config.Get().MasterServers.NamePattern, 1))
		if err != nil {
			log.WithError(err).Error()

			continue
		}

		loadBalancerIP, err := api.waitForLoadBalancer(api.config.Get().ClusterName)
		if err != nil {
			log.WithError(err).Error()

			continue
		}

		stdout, stderr, err := api.execCommand(serverIP, api.getInitMasterCommand(loadBalancerIP))
		if err != nil {
			log.WithError(err).Error(stderr)

			continue
		}

		log.Debugf("stdout=%s", stdout)
		log.Debugf("stderr=%s", stderr)

		stdout, stderr, err = api.execCommand(serverIP, "cat /root/scripts/join-master.sh")
		if err != nil {
			log.WithError(err).Fatal(stderr)
		}

		api.masterClusterJoin = stdout

		stdout, stderr, err = api.execCommand(serverIP, "cat /etc/kubernetes/admin.conf")
		if err != nil {
			log.WithError(err).Fatal(stderr)
		}

		api.clusterKubeConfig = stdout

		break
	}

	return nil
}

func (api *ApplicationAPI) createLoadBalancer() error {
	log.Info("Creating loadbalancer...")

	k8sLoadBalancerType, _, err := api.hcloudClient.LoadBalancerType.Get(
		api.ctx,
		api.config.Get().MasterLoadBalancer.LoadBalancerType,
	)
	if err != nil {
		return err
	}

	k8sLocation, _, err := api.hcloudClient.Location.Get(api.ctx, api.config.Get().MasterLoadBalancer.Location)
	if err != nil {
		return err
	}

	k8sNetwork, _, err := api.hcloudClient.Network.Get(api.ctx, api.config.Get().ClusterName)
	if err != nil {
		return err
	}

	k8sTarget := hcloud.LoadBalancerCreateOptsTarget{
		Type: hcloud.LoadBalancerTargetTypeLabelSelector,
		LabelSelector: hcloud.LoadBalancerCreateOptsTargetLabelSelector{
			Selector: api.config.Get().MasterLoadBalancer.Selector,
		},
	}

	ListenPort := api.config.Get().MasterLoadBalancer.ListenPort
	DestinationPort := api.config.Get().MasterLoadBalancer.DestinationPort

	k8sService := hcloud.LoadBalancerCreateOptsService{
		Protocol:        hcloud.LoadBalancerServiceProtocolTCP,
		ListenPort:      &ListenPort,
		DestinationPort: &DestinationPort,
	}

	_, _, err = api.hcloudClient.LoadBalancer.Create(api.ctx, hcloud.LoadBalancerCreateOpts{
		Name:             api.config.Get().ClusterName,
		LoadBalancerType: k8sLoadBalancerType,
		Location:         k8sLocation,
		Network:          k8sNetwork,
		Targets:          []hcloud.LoadBalancerCreateOptsTarget{k8sTarget},
		Services:         []hcloud.LoadBalancerCreateOptsService{k8sService},
	})

	if err != nil {
		return err
	}

	return nil
}

func (api *ApplicationAPI) createServer() error {
	log.Info("Creating servers...")

	serverType, _, err := api.hcloudClient.ServerType.Get(api.ctx, api.config.Get().MasterServers.ServerType)
	if err != nil {
		return err
	}

	serverImage, _, err := api.hcloudClient.Image.Get(api.ctx, api.config.Get().MasterServers.Image)
	if err != nil {
		return err
	}

	k8sNetwork, _, err := api.hcloudClient.Network.Get(api.ctx, api.config.Get().ClusterName)
	if err != nil {
		return err
	}

	k8sSSHKey, _, err := api.hcloudClient.SSHKey.Get(api.ctx, api.config.Get().ClusterName)
	if err != nil {
		return err
	}

	k8sDatacenter, _, err := api.hcloudClient.Datacenter.Get(api.ctx, api.config.Get().MasterServers.Datacenter)
	if err != nil {
		return err
	}

	startAfterCreate := true

	for i := 1; i <= api.config.Get().MasterCount; i++ {
		prop := hcloud.ServerCreateOpts{
			Name:             fmt.Sprintf(api.config.Get().MasterServers.NamePattern, i),
			ServerType:       serverType,
			Image:            serverImage,
			Networks:         []*hcloud.Network{k8sNetwork},
			SSHKeys:          []*hcloud.SSHKey{k8sSSHKey},
			Labels:           api.config.Get().MasterServers.Labels,
			Datacenter:       k8sDatacenter,
			StartAfterCreate: &startAfterCreate,
		}

		// install kubelet kubeadm on server start
		if i > 1 {
			prop.UserData = api.getCommonInstallCommand()
		}

		_, _, err = api.hcloudClient.Server.Create(api.ctx, prop)
		if err != nil {
			return err
		}
	}

	return nil
}

func (api *ApplicationAPI) createSSHKey() error {
	log.Info("Creating sshKey...")

	publicKey, err := ioutil.ReadFile(api.config.Get().SSHPublicKey)
	if err != nil {
		return err
	}

	_, _, err = api.hcloudClient.SSHKey.Create(api.ctx, hcloud.SSHKeyCreateOpts{
		Name:      api.config.Get().ClusterName,
		PublicKey: string(publicKey),
	})
	if err != nil {
		return err
	}

	return nil
}

func (api *ApplicationAPI) createNetwork() error {
	log.Info("Creating network...")

	k8sNetwork, _, err := api.hcloudClient.Network.Create(api.ctx, hcloud.NetworkCreateOpts{
		Name:    api.config.Get().ClusterName,
		IPRange: api.config.Get().IPRangeNet,
	})
	if err != nil {
		return err
	}

	k8sNetworkSubnet := hcloud.NetworkSubnet{
		Type:        hcloud.NetworkSubnetTypeServer,
		IPRange:     api.config.Get().IPRangeNet,
		NetworkZone: hcloud.NetworkZoneEUCentral,
	}

	_, _, err = api.hcloudClient.Network.AddSubnet(api.ctx, k8sNetwork, hcloud.NetworkAddSubnetOpts{
		Subnet: k8sNetworkSubnet,
	})
	if err != nil {
		return err
	}

	return nil
}

func (api *ApplicationAPI) NewCluster() error {
	log.Info("Creating cluster...")

	err := api.createNetwork()
	if err != nil {
		return errors.Wrap(err, "error in create network")
	}

	err = api.createSSHKey()
	if err != nil {
		return errors.Wrap(err, "error in create sshkey")
	}

	err = api.createLoadBalancer()
	if err != nil {
		return errors.Wrap(err, "error in create loadbalancer")
	}

	err = api.createServer()
	if err != nil {
		return errors.Wrap(err, "error in create server")
	}

	err = api.initFirstMasterNode()
	if err != nil {
		return errors.Wrap(err, "error in init first master nodes")
	}

	for i := 2; i <= api.config.Get().MasterCount; i++ {
		err = api.joinToMasterNodes(fmt.Sprintf(api.config.Get().MasterServers.NamePattern, i))
		if err != nil {
			return errors.Wrap(err, "error in join")
		}
	}

	log.Info("kubeconfig=\n" + api.clusterKubeConfig)
	log.Infof("Saving kubeconfig to %s", api.config.Get().KubeConfigPath)

	err = ioutil.WriteFile(api.config.Get().KubeConfigPath, []byte(api.clusterKubeConfig), 0644) //nolint:gosec
	if err != nil {
		log.WithError(err).Error(err)
	}

	return nil
}

func (api *ApplicationAPI) DeleteCluster() {
	log.Info("Deleting cluster...")

	k8sNetwork, _, _ := api.hcloudClient.Network.Get(api.ctx, api.config.Get().ClusterName)
	if k8sNetwork != nil {
		_, _ = api.hcloudClient.Network.Delete(api.ctx, k8sNetwork)
	}

	k8sSSHKey, _, _ := api.hcloudClient.SSHKey.Get(api.ctx, api.config.Get().ClusterName)
	if k8sSSHKey != nil {
		_, _ = api.hcloudClient.SSHKey.Delete(api.ctx, k8sSSHKey)
	}

	k8sLoadBalancer, _, _ := api.hcloudClient.LoadBalancer.Get(api.ctx, api.config.Get().ClusterName)
	if k8sLoadBalancer != nil {
		_, _ = api.hcloudClient.LoadBalancer.Delete(api.ctx, k8sLoadBalancer)
	}

	for i := 1; i <= api.config.Get().MasterCount; i++ {
		k8sServer, _, _ := api.hcloudClient.Server.Get(api.ctx, fmt.Sprintf(api.config.Get().MasterServers.NamePattern, i))
		if k8sServer != nil {
			_, _ = api.hcloudClient.Server.Delete(api.ctx, k8sServer)
		}
	}

	nodeServers, _, _ := api.hcloudClient.Server.List(api.ctx, hcloud.ServerListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: "hcloud/node-group",
		},
	})

	for _, nodeServer := range nodeServers {
		_, _ = api.hcloudClient.Server.Delete(api.ctx, nodeServer)
	}
}

func (api *ApplicationAPI) execCommand(ipAddress string, command string) (string, string, error) {
	privateKey, err := ioutil.ReadFile(api.config.Get().SSHPrivateKey)
	if err != nil {
		return "", "", err
	}

	key, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return "", "", err
	}

	config := &ssh.ClientConfig{
		User:            "root",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort(ipAddress, "22"), config)
	if err != nil {
		return "", "", err
	}

	session, err := client.NewSession()
	if err != nil {
		return "", "", err
	}

	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(command)
	if err != nil {
		return stdout.String(), stderr.String(), err
	}

	return stdout.String(), stderr.String(), nil
}
