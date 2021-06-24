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
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/maksim-paskal/hcloud-k8s-ctl/pkg/config"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

type ApplicationAPI struct {
	config            *config.ApplicationConfig
	hcloudClient      *hcloud.Client
	ctx               context.Context
	masterClusterJoin string
	clusterKubeConfig string
}

func NewApplicationAPI(config *config.ApplicationConfig) (*ApplicationAPI, error) {
	api := ApplicationAPI{
		ctx:          context.Background(),
		config:       config,
		hcloudClient: hcloud.NewClient(hcloud.WithToken(config.Get().HetznerToken)),
	}

	if err := api.validateConfig(); err != nil {
		return &api, err
	}

	return &api, nil
}

func (api *ApplicationAPI) validateConfig() error {
	location, _, err := api.hcloudClient.Location.Get(api.ctx, api.config.Get().Location)
	if err != nil {
		return errors.Wrap(err, "hcloudClient.Location.Get")
	}

	if location == nil {
		return errors.Wrap(errLocationNotFound, api.config.Get().Location)
	}

	datacenter, _, err := api.hcloudClient.Datacenter.Get(api.ctx, api.config.Get().Datacenter)
	if err != nil {
		return errors.Wrap(err, "hcloudClient.Datacenter.Get")
	}

	if datacenter == nil {
		return errors.Wrap(errDatacenterNotFound, api.config.Get().Datacenter)
	}

	return nil
}

func (api *ApplicationAPI) saveKubeconfig() error {
	log.Info("kubeconfig=\n" + api.clusterKubeConfig)
	log.Infof("Saving kubeconfig to %s", api.config.Get().KubeConfigPath)

	err := ioutil.WriteFile(
		api.config.Get().KubeConfigPath,
		[]byte(api.clusterKubeConfig),
		kubeconfigFileMode,
	)
	if err != nil {
		return err
	}

	return nil
}

func (api *ApplicationAPI) getCommonExecCommand() string {
	return fmt.Sprintf(commonExecCommand,
		api.config.Get().MasterServers.ServersInitParams.TarGz,
		api.config.Get().MasterServers.ServersInitParams.Folder,
	)
}

func (api *ApplicationAPI) getCommonInstallCommand() string {
	return api.getCommonExecCommand() + `

/root/scripts/common-install.sh

`
}

func (api *ApplicationAPI) getInitMasterCommand(loadBalancerIP string) string {
	return api.getCommonExecCommand() + `

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
	log := log.WithField("server", server)

	log.Infof("Join server to master nodes...")

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

		log.Infof("Waiting for server... try=%d", retryCount)

		serverIP, err := api.waitForServer(server)
		if err != nil {
			log.WithError(err).Error()

			continue
		}

		log.Info("Executing command...")

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

func (api *ApplicationAPI) postInstall(copyNewScripts bool) error {
	log.Info("Executing postInstall...")

	serverName := fmt.Sprintf(api.config.Get().MasterServers.NamePattern, 1)

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

		serverIP, err := api.waitForServer(serverName)
		if err != nil {
			log.WithError(err).Error()

			continue
		}

		if copyNewScripts {
			log.Info("Clear current root directory - and loading new scripts")

			_, stderr, err := api.execCommand(serverIP, api.getCommonExecCommand())
			if err != nil {
				log.WithError(err).Fatal(stderr)
			}
		}

		log.Info("Executing command...")

		postInstallCommand := fmt.Sprintf("VALUES=%s /root/scripts/post-install.sh", api.getDeploymentValues())

		stdout, stderr, err := api.execCommand(serverIP, postInstallCommand)
		if err != nil {
			log.WithError(err).Fatal(stderr)
		}

		log.Debugf("stdout=%s", stdout)
		log.Debugf("stderr=%s", stderr)

		if api.config.Get().MasterCount == 1 {
			stdout, stderr, err := api.execCommand(serverIP, "/root/scripts/one-master-mode.sh")
			if err != nil {
				log.WithError(err).Fatal(stderr)
			}

			log.Debugf("stdout=%s", stdout)
			log.Debugf("stderr=%s", stderr)
		}

		break
	}

	return nil
}

func (api *ApplicationAPI) initFirstMasterNode() error { //nolint:funlen
	log.Info("Init first master node...")

	serverName := fmt.Sprintf(api.config.Get().MasterServers.NamePattern, 1)

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

		serverIP, err := api.waitForServer(serverName)
		if err != nil {
			log.WithError(err).Debug()

			continue
		}

		loadBalancerIP := serverIP

		if api.config.Get().MasterCount > 1 {
			log.Info("Waiting for loadBalancer...")

			loadBalancerIP, err = api.waitForLoadBalancer(api.config.Get().ClusterName)
			if err != nil {
				log.WithError(err).Error()

				continue
			}
		}

		log.Info("Executing command...")

		stdout, stderr, err := api.execCommand(serverIP, api.getInitMasterCommand(loadBalancerIP))
		if err != nil {
			log.WithError(err).Error(stderr)

			continue
		}

		log.Debugf("stdout=%s", stdout)
		log.Debugf("stderr=%s", stderr)

		log.Info("Get join command...")

		stdout, stderr, err = api.execCommand(serverIP, "cat /root/scripts/join-master.sh")
		if err != nil {
			log.WithError(err).Fatal(stderr)
		}

		api.masterClusterJoin = stdout

		log.Info("Get kubeconfig..")

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

	k8sLocation, _, err := api.hcloudClient.Location.Get(api.ctx, api.config.Get().Location)
	if err != nil {
		return err
	}

	k8sNetwork, _, err := api.hcloudClient.Network.Get(api.ctx, api.config.Get().ClusterName)
	if err != nil {
		return err
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
		Services:         []hcloud.LoadBalancerCreateOptsService{k8sService},
	})

	if err != nil {
		return err
	}

	return nil
}

func (api *ApplicationAPI) attachToBalancer(server hcloud.ServerCreateResult, balancer *hcloud.LoadBalancer) error {
	usePrivateIP := true
	k8sTargetServer := hcloud.LoadBalancerAddServerTargetOpts{
		Server:       server.Server,
		UsePrivateIP: &usePrivateIP,
	}

	_, _, err := api.hcloudClient.LoadBalancer.AddServerTarget(api.ctx, balancer, k8sTargetServer)
	if err != nil {
		return err
	}

	return nil
}

func (api *ApplicationAPI) createServer() error { //nolint:funlen,cyclop
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

	k8sDatacenter, _, err := api.hcloudClient.Datacenter.Get(api.ctx, api.config.Get().Datacenter)
	if err != nil {
		return err
	}

	startAfterCreate := true

	k8sLoadBalancer, _, err := api.hcloudClient.LoadBalancer.Get(api.ctx, api.config.Get().ClusterName)
	if err != nil {
		return err
	}

	for i := 1; i <= api.config.Get().MasterCount; i++ {
		serverName := fmt.Sprintf(api.config.Get().MasterServers.NamePattern, i)

		log := log.WithField("server", serverName)

		prop := hcloud.ServerCreateOpts{
			Name:             serverName,
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

		serverResults, _, err := api.hcloudClient.Server.Create(api.ctx, prop)
		if err != nil {
			return err
		}

		retryCount := 0

		for {
			if retryCount > api.config.Get().MasterServers.RetryTimeLimit {
				return errRetryLimitReached
			}

			if api.config.Get().MasterCount > 1 {
				err = api.attachToBalancer(serverResults, k8sLoadBalancer)
				if err != nil {
					log.WithError(err).Debug()
					time.Sleep(api.config.Get().MasterServers.WaitTimeInRetry)

					continue
				}
			}

			break
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

	_, IPRangeNet, err := net.ParseCIDR(api.config.Get().IPRange)
	if err != nil {
		return err
	}

	k8sNetwork, _, err := api.hcloudClient.Network.Create(api.ctx, hcloud.NetworkCreateOpts{
		Name:    api.config.Get().ClusterName,
		IPRange: IPRangeNet,
	})
	if err != nil {
		return err
	}

	k8sNetworkSubnet := hcloud.NetworkSubnet{
		Type:        hcloud.NetworkSubnetTypeServer,
		IPRange:     IPRangeNet,
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

func (api *ApplicationAPI) NewCluster() error { //nolint:cyclop
	log.Info("Creating cluster...")

	err := api.createNetwork()
	if err != nil {
		return errors.Wrap(err, "error in create network")
	}

	err = api.createSSHKey()
	if err != nil {
		return errors.Wrap(err, "error in create sshkey")
	}

	if api.config.Get().MasterCount > 1 {
		err = api.createLoadBalancer()
		if err != nil {
			return errors.Wrap(err, "error in create loadbalancer")
		}
	}

	err = api.createServer()
	if err != nil {
		return errors.Wrap(err, "error in create server")
	}

	err = api.initFirstMasterNode()
	if err != nil {
		return errors.Wrap(err, "error in init first master nodes")
	}

	err = api.saveKubeconfig()
	if err != nil {
		log.WithError(err).Warn("error in saving kubeconfig")
	}

	for i := 2; i <= api.config.Get().MasterCount; i++ {
		serverName := fmt.Sprintf(api.config.Get().MasterServers.NamePattern, i)

		log := log.WithField("server", serverName)

		err = api.joinToMasterNodes(serverName)
		if err != nil {
			log.WithError(err).Error(err, "error in join")
		}
	}

	err = api.postInstall(false)
	if err != nil {
		return errors.Wrap(err, "error in postInstall")
	}

	log.Info("Cluster created!")

	return nil
}

func (api *ApplicationAPI) DeleteCluster() { //nolint:cyclop
	log.Info("Deleting cluster...")

	k8sNetwork, _, _ := api.hcloudClient.Network.Get(api.ctx, api.config.Get().ClusterName)
	if k8sNetwork != nil {
		_, err := api.hcloudClient.Network.Delete(api.ctx, k8sNetwork)
		if err != nil {
			log.WithError(err).Warn("error deleting Network")
		}
	}

	k8sSSHKey, _, _ := api.hcloudClient.SSHKey.Get(api.ctx, api.config.Get().ClusterName)
	if k8sSSHKey != nil {
		_, err := api.hcloudClient.SSHKey.Delete(api.ctx, k8sSSHKey)
		if err != nil {
			log.WithError(err).Warn("error deleting SSHKey")
		}
	}

	k8sLoadBalancer, _, _ := api.hcloudClient.LoadBalancer.Get(api.ctx, api.config.Get().ClusterName)
	if k8sLoadBalancer != nil {
		_, err := api.hcloudClient.LoadBalancer.Delete(api.ctx, k8sLoadBalancer)
		if err != nil {
			log.WithError(err).Warn("error deleting LoadBalancer")
		}
	}

	// delete master nodes
	nodeServers, _, _ := api.hcloudClient.Server.List(api.ctx, hcloud.ServerListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: "role=master",
		},
	})

	for _, nodeServer := range nodeServers {
		_, err := api.hcloudClient.Server.Delete(api.ctx, nodeServer)
		if err != nil {
			log.WithError(err).Warnf("error deleting Server=%s", nodeServer.Name)
		}
	}

	// delete worker nodes
	nodeServers, _, _ = api.hcloudClient.Server.List(api.ctx, hcloud.ServerListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: "hcloud/node-group",
		},
	})

	for _, nodeServer := range nodeServers {
		_, err := api.hcloudClient.Server.Delete(api.ctx, nodeServer)
		if err != nil {
			log.WithError(err).Warnf("error deleting Server=%s", nodeServer.Name)
		}
	}
}

func (api *ApplicationAPI) execCommand(ipAddress string, command string) (string, string, error) {
	log.Debugf("ipAddress=%s,command=%s", ipAddress, command)

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

func (api *ApplicationAPI) ListConfigurations() {
	type DatacentersType struct {
		Location string
		Name     string
	}

	type ResultType struct {
		Locations   []string
		Datacenters []DatacentersType
		ServerType  []string
	}

	result := ResultType{}

	var err error

	locations, err := api.hcloudClient.Location.All(api.ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, location := range locations {
		result.Locations = append(result.Locations, location.Name)
	}

	datacenters, err := api.hcloudClient.Datacenter.All(api.ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, datacenter := range datacenters {
		result.Datacenters = append(result.Datacenters, DatacentersType{
			Name:     datacenter.Name,
			Location: datacenter.Location.Name,
		})
	}

	servertypes, err := api.hcloudClient.ServerType.All(api.ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, servertype := range servertypes {
		result.ServerType = append(result.ServerType, servertype.Name)
	}

	resultYAML, err := yaml.Marshal(result)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("\n%s", string(resultYAML))
}

func (api *ApplicationAPI) getDeploymentValues() string {
	resultYAML, err := yaml.Marshal(api.config.Get())
	if err != nil {
		log.Fatal(err)
	}

	log.Debug(string(resultYAML))

	return base64.StdEncoding.EncodeToString(resultYAML)
}

func (api *ApplicationAPI) PatchClusterDeployment() {
	if err := api.postInstall(true); err != nil {
		log.Fatal(err)
	}

	log.Info("Cluster pached!")
}
