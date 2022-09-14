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
	"net"
	"os"
	"sync"
	"time"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/maksim-paskal/hcloud-k8s-ctl/pkg/config"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

var ctx = context.Background() //nolint:gochecknoglobals

type ApplicationAPI struct {
	hcloudClient      *hcloud.Client
	masterClusterJoin string
	clusterKubeConfig string
	sshRootUser       string
}

func NewApplicationAPI() (*ApplicationAPI, error) {
	log.Info("Connecting to Hetzner Cloud API...")

	api := ApplicationAPI{
		hcloudClient: hcloud.NewClient(hcloud.WithToken(config.Get().HetznerToken.Main)),
	}

	if err := api.validateConfig(); err != nil {
		return &api, err
	}

	api.sshRootUser = "hcloud-user"

	return &api, nil
}

func (api *ApplicationAPI) validateConfig() error {
	log.Info("Validating config...")

	location, _, err := api.hcloudClient.Location.Get(ctx, config.Get().Location)
	if err != nil {
		return errors.Wrap(err, "hcloudClient.Location.Get")
	}

	if location == nil {
		return errors.Wrap(errLocationNotFound, config.Get().Location)
	}

	datacenter, _, err := api.hcloudClient.Datacenter.Get(ctx, config.Get().Datacenter)
	if err != nil {
		return errors.Wrap(err, "hcloudClient.Datacenter.Get")
	}

	if datacenter == nil {
		return errors.Wrap(errDatacenterNotFound, config.Get().Datacenter)
	}

	return nil
}

func (api *ApplicationAPI) saveKubeconfig() error {
	log.Info("kubeconfig=\n" + api.clusterKubeConfig)
	log.Infof("Saving kubeconfig to %s", config.Get().KubeConfigPath)

	err := os.WriteFile(
		config.Get().KubeConfigPath,
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
		config.Get().MasterServers.ServersInitParams.TarGz,
		config.Get().MasterServers.ServersInitParams.Folder,
		api.getDeploymentValues(),
	)
}

func (api *ApplicationAPI) getCommonInstallCommand() string {
	return api.getCommonExecCommand() + `

/root/scripts/common-install.sh

`
}

func (api *ApplicationAPI) getInitMasterCommand(loadBalancerIP string) string {
	return api.getCommonExecCommand() + `

export MASTER_LB=` + loadBalancerIP + `

/root/scripts/init-master.sh
`
}

func (api *ApplicationAPI) waitForLoadBalancer(loadBalancerName string) (string, error) {
	loadBalancer, _, err := api.hcloudClient.LoadBalancer.Get(ctx, loadBalancerName)
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
	masterServer, _, err := api.hcloudClient.Server.Get(ctx, server)
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
		if retryCount > config.Get().MasterServers.RetryTimeLimit {
			return errRetryLimitReached
		}

		if retryCount > 0 {
			time.Sleep(config.Get().MasterServers.WaitTimeInRetry)
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

	serverName := fmt.Sprintf(config.Get().MasterServers.NamePattern, 1)

	retryCount := 0

	for {
		if retryCount > config.Get().MasterServers.RetryTimeLimit {
			return errRetryLimitReached
		}

		if retryCount > 0 {
			time.Sleep(config.Get().MasterServers.WaitTimeInRetry)
		}
		retryCount++

		log.Infof("Waiting for master node... try=%d", retryCount)

		serverIP, err := api.waitForServer(serverName)
		if err != nil {
			log.WithError(err).Error()

			continue
		}

		if copyNewScripts {
			if err = api.downloadNewScripts(serverName, serverIP); err != nil {
				log.WithError(err).Fatal()
			}
		}

		log.Info("Executing command...")

		stdout, stderr, err := api.execCommand(serverIP, "/root/scripts/post-install.sh")
		if err != nil {
			log.WithError(err).Fatal(stderr)
		}

		log.Debugf("stdout=%s", stdout)
		log.Debugf("stderr=%s", stderr)

		if config.Get().MasterCount == 1 {
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

	// hetzner cloud default user is root
	api.sshRootUser = "root"

	serverName := fmt.Sprintf(config.Get().MasterServers.NamePattern, 1)

	retryCount := 0

	for {
		if retryCount > config.Get().MasterServers.RetryTimeLimit {
			return errRetryLimitReached
		}

		if retryCount > 0 {
			time.Sleep(config.Get().MasterServers.WaitTimeInRetry)
		}
		retryCount++

		log.Infof("Waiting for master node... try=%d", retryCount)

		serverIP, err := api.waitForServer(serverName)
		if err != nil {
			log.WithError(err).Debug()

			continue
		}

		log.Info("Waiting for loadBalancer...")

		loadBalancerIP, err := api.waitForLoadBalancer(config.Get().ClusterName)
		if err != nil {
			log.WithError(err).Error()

			continue
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

		api.sshRootUser = "hcloud-user"

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
		ctx,
		config.Get().MasterLoadBalancer.LoadBalancerType,
	)
	if err != nil {
		return err
	}

	k8sLocation, _, err := api.hcloudClient.Location.Get(ctx, config.Get().Location)
	if err != nil {
		return err
	}

	k8sNetwork, _, err := api.hcloudClient.Network.Get(ctx, config.Get().ClusterName)
	if err != nil {
		return err
	}

	ListenPort := config.Get().MasterLoadBalancer.ListenPort
	DestinationPort := config.Get().MasterLoadBalancer.DestinationPort

	k8sService := hcloud.LoadBalancerCreateOptsService{
		Protocol:        hcloud.LoadBalancerServiceProtocolTCP,
		ListenPort:      &ListenPort,
		DestinationPort: &DestinationPort,
		HealthCheck: &hcloud.LoadBalancerCreateOptsServiceHealthCheck{
			Protocol: hcloud.LoadBalancerServiceProtocolHTTP,
			Port:     &ListenPort,
			Interval: hcloud.Duration(hcloudLoadBalancerInterval),
			Timeout:  hcloud.Duration(hcloudLoadBalancerTimeout),
			Retries:  hcloud.Int(hcloudLoadBalancerRetries),
			HTTP: &hcloud.LoadBalancerCreateOptsServiceHealthCheckHTTP{
				Path:        hcloud.String("/healthz"),
				StatusCodes: []string{"2??"},
				TLS:         hcloud.Bool(true),
			},
		},
	}

	_, _, err = api.hcloudClient.LoadBalancer.Create(ctx, hcloud.LoadBalancerCreateOpts{
		Name:             config.Get().ClusterName,
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

	_, _, err := api.hcloudClient.LoadBalancer.AddServerTarget(ctx, balancer, k8sTargetServer)
	if err != nil {
		return err
	}

	return nil
}

func (api *ApplicationAPI) createServer() error { //nolint:funlen,cyclop
	log.Info("Creating servers...")

	serverType, _, err := api.hcloudClient.ServerType.Get(ctx, config.Get().MasterServers.ServerType)
	if err != nil {
		return err
	}

	serverImage, _, err := api.hcloudClient.Image.Get(ctx, config.Get().MasterServers.Image)
	if err != nil {
		return err
	}

	k8sNetwork, _, err := api.hcloudClient.Network.Get(ctx, config.Get().ClusterName)
	if err != nil {
		return err
	}

	k8sSSHKey, _, err := api.hcloudClient.SSHKey.Get(ctx, config.Get().ClusterName)
	if err != nil {
		return err
	}

	k8sDatacenter, _, err := api.hcloudClient.Datacenter.Get(ctx, config.Get().Datacenter)
	if err != nil {
		return err
	}

	startAfterCreate := true

	k8sLoadBalancer, _, err := api.hcloudClient.LoadBalancer.Get(ctx, config.Get().ClusterName)
	if err != nil {
		return err
	}

	var placementGroupResults hcloud.PlacementGroupCreateResult

	placementGroupResults, _, err = api.hcloudClient.PlacementGroup.Create(ctx, hcloud.PlacementGroupCreateOpts{
		Name: config.Get().MasterServers.PlacementGroupName,
		Type: hcloud.PlacementGroupTypeSpread,
	})
	if err != nil {
		return err
	}

	for i := 1; i <= config.Get().MasterCount; i++ {
		serverName := fmt.Sprintf(config.Get().MasterServers.NamePattern, i)

		log := log.WithField("server", serverName)

		prop := hcloud.ServerCreateOpts{
			Name:             serverName,
			ServerType:       serverType,
			Image:            serverImage,
			Networks:         []*hcloud.Network{k8sNetwork},
			SSHKeys:          []*hcloud.SSHKey{k8sSSHKey},
			Labels:           config.Get().MasterServers.Labels,
			Datacenter:       k8sDatacenter,
			StartAfterCreate: &startAfterCreate,
		}

		prop.PlacementGroup = placementGroupResults.PlacementGroup

		// install kubelet kubeadm on server start
		if i > 1 {
			prop.UserData = api.getCommonInstallCommand()
		}

		serverResults, _, err := api.hcloudClient.Server.Create(ctx, prop)
		if err != nil {
			return err
		}

		retryCount := 0

		for {
			if retryCount > config.Get().MasterServers.RetryTimeLimit {
				return errRetryLimitReached
			}

			err = api.attachToBalancer(serverResults, k8sLoadBalancer)
			if err != nil {
				log.WithError(err).Debug()
				time.Sleep(config.Get().MasterServers.WaitTimeInRetry)

				continue
			}

			break
		}
	}

	return nil
}

func (api *ApplicationAPI) createSSHKey() error {
	log.Info("Creating sshKey...")

	publicKey, err := os.ReadFile(config.Get().SSHPublicKey)
	if err != nil {
		return err
	}

	_, _, err = api.hcloudClient.SSHKey.Create(ctx, hcloud.SSHKeyCreateOpts{
		Name:      config.Get().ClusterName,
		PublicKey: string(publicKey),
	})
	if err != nil {
		return err
	}

	return nil
}

func (api *ApplicationAPI) createNetwork() error {
	log.Info("Creating network...")

	_, IPRangeNet, err := net.ParseCIDR(config.Get().IPRange)
	if err != nil {
		return err
	}

	k8sNetwork, _, err := api.hcloudClient.Network.Create(ctx, hcloud.NetworkCreateOpts{
		Name:    config.Get().ClusterName,
		IPRange: IPRangeNet,
	})
	if err != nil {
		return err
	}

	k8sNetworkSubnet := hcloud.NetworkSubnet{
		Type:        hcloud.NetworkSubnetTypeServer,
		IPRange:     IPRangeNet,
		NetworkZone: config.Get().NetworkZone,
	}

	_, _, err = api.hcloudClient.Network.AddSubnet(ctx, k8sNetwork, hcloud.NetworkAddSubnetOpts{
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

	err = api.CreateFirewall(true, true)
	if err != nil {
		return errors.Wrap(err, "error in create firewall")
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

	err = api.saveKubeconfig()
	if err != nil {
		log.WithError(err).Warn("error in saving kubeconfig")
	}

	for i := 2; i <= config.Get().MasterCount; i++ {
		serverName := fmt.Sprintf(config.Get().MasterServers.NamePattern, i)

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

func (api *ApplicationAPI) DeleteCluster() { //nolint: cyclop,funlen
	log.Info("Deleting cluster...")

	k8sNetwork, _, _ := api.hcloudClient.Network.Get(ctx, config.Get().ClusterName)
	if k8sNetwork != nil {
		_, err := api.hcloudClient.Network.Delete(ctx, k8sNetwork)
		if err != nil {
			log.WithError(err).Warn("error deleting Network")
		}
	}

	k8sSSHKey, _, _ := api.hcloudClient.SSHKey.Get(ctx, config.Get().ClusterName)
	if k8sSSHKey != nil {
		_, err := api.hcloudClient.SSHKey.Delete(ctx, k8sSSHKey)
		if err != nil {
			log.WithError(err).Warn("error deleting SSHKey")
		}
	}

	k8sLoadBalancer, _, _ := api.hcloudClient.LoadBalancer.Get(ctx, config.Get().ClusterName)
	if k8sLoadBalancer != nil {
		_, err := api.hcloudClient.LoadBalancer.Delete(ctx, k8sLoadBalancer)
		if err != nil {
			log.WithError(err).Warn("error deleting LoadBalancer")
		}
	}

	// get master nodes
	allServers, _, _ := api.hcloudClient.Server.List(ctx, hcloud.ServerListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: api.getMasterLabels(),
		},
	})

	// get worker nodes
	nodeServers, _, _ := api.hcloudClient.Server.List(ctx, hcloud.ServerListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: "hcloud/node-group",
		},
	})

	allServers = append(allServers, nodeServers...)

	for _, nodeServer := range allServers {
		_, err := api.hcloudClient.Server.Delete(ctx, nodeServer)
		if err != nil {
			log.WithError(err).Warnf("error deleting Server=%s", nodeServer.Name)
		}
	}

	placementGroup, _, _ := api.hcloudClient.PlacementGroup.Get(ctx, config.Get().MasterServers.PlacementGroupName)
	if placementGroup != nil {
		_, err := api.hcloudClient.PlacementGroup.Delete(ctx, placementGroup)
		if err != nil {
			log.WithError(err).Warnf("error deleting PlacementGroup=%s", placementGroup.Name)
		}
	}

	log.Debug("Wait some time for deleting firewall...")
	time.Sleep(3 * time.Second) //nolint:gomnd

	k8sFirewalls, _, _ := api.hcloudClient.Firewall.List(ctx, hcloud.FirewallListOpts{
		ListOpts: hcloud.ListOpts{
			LabelSelector: "cluster=" + config.Get().ClusterName,
		},
	})
	for _, k8sFirewall := range k8sFirewalls {
		_, err := api.hcloudClient.Firewall.Delete(ctx, k8sFirewall)
		if err != nil {
			log.WithError(err).Warn("error deleting Firewall")
		}
	}
}

func (api *ApplicationAPI) execCommand(ipAddress string, command string) (string, string, error) {
	log.Debugf("user=%s,ipAddress=%s,command=%s", api.sshRootUser, ipAddress, command)

	privateKey, err := os.ReadFile(config.Get().SSHPrivateKey)
	if err != nil {
		return "", "", err
	}

	key, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return "", "", err
	}

	config := &ssh.ClientConfig{
		User:            api.sshRootUser,
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

	sshCommand := fmt.Sprintf(`echo "%s" | base64 -d | sudo bash`, base64.StdEncoding.EncodeToString([]byte(command)))

	log.Debug(sshCommand)

	err = session.Run(sshCommand)
	if err != nil {
		log.Error(stdout.String(), stderr.String())

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

	locations, err := api.hcloudClient.Location.All(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, location := range locations {
		result.Locations = append(result.Locations, location.Name)
	}

	datacenters, err := api.hcloudClient.Datacenter.All(ctx)
	if err != nil {
		log.Fatal(err)
	}

	for _, datacenter := range datacenters {
		result.Datacenters = append(result.Datacenters, DatacentersType{
			Name:     datacenter.Name,
			Location: datacenter.Location.Name,
		})
	}

	servertypes, err := api.hcloudClient.ServerType.All(ctx)
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
	resultYAML, err := yaml.Marshal(config.Get())
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

func (api *ApplicationAPI) ExecuteAdHoc(user string, command string, runOnMasters bool, runOnWorkers bool, copyNewScripts bool) { //nolint:funlen,lll,cyclop
	log.Info("Executing adhoc...")

	if len(user) > 0 {
		api.sshRootUser = user
	}

	var allServers []*hcloud.Server

	if runOnWorkers {
		log.Info("Get worker nodes...")

		workerServers, _, _ := api.hcloudClient.Server.List(ctx, hcloud.ServerListOpts{
			ListOpts: hcloud.ListOpts{
				LabelSelector: "hcloud/node-group",
			},
		})

		allServers = append(allServers, workerServers...)
	}

	if runOnMasters {
		log.Info("Get master nodes...")

		masterServers, _, _ := api.hcloudClient.Server.List(ctx, hcloud.ServerListOpts{
			ListOpts: hcloud.ListOpts{
				LabelSelector: api.getMasterLabels(),
			},
		})

		allServers = append(allServers, masterServers...)
	}

	if len(allServers) == 0 {
		log.Error("No servers found")

		return
	}

	adhocStatus := make(map[string]string)

	var (
		wg    sync.WaitGroup
		mutex sync.Mutex
	)

	wg.Add(len(allServers))

	log.Info("Start executing command on selected nodes")

	for _, server := range allServers {
		go func(server *hcloud.Server) {
			defer wg.Done()

			log := log.WithField("server", server.Name)

			serverIP, err := server.PublicNet.IPv4.IP.MarshalText()
			if err != nil {
				log.WithError(err).Error("can not get server IP")
				adhocStatus[server.Name] = err.Error()

				return
			}

			if copyNewScripts {
				if err = api.downloadNewScripts(server.Name, string(serverIP)); err != nil {
					log.WithError(err).Fatal()
				}
			}

			stdout, stderr, err := api.execCommand(string(serverIP), command)
			if err != nil {
				log.WithError(err).Error(stderr)
				adhocStatus[server.Name] = err.Error()

				return
			}

			log.Infof("stdout=%s,stderr=%s", stdout, stderr)

			mutex.Lock()
			adhocStatus[server.Name] = "ok"
			mutex.Unlock()
		}(server)
	}

	wg.Wait()

	for name, status := range adhocStatus {
		if status == "ok" {
			log.Infof("%s -> %s", name, status)
		} else {
			log.Errorf("%s -> %s", name, status)
		}
	}
}

func (api *ApplicationAPI) getMasterLabels() string {
	result := ""

	for key, val := range config.Get().MasterServers.Labels {
		if len(result) == 0 {
			result = fmt.Sprintf("%s=%s", key, val)
		} else {
			result = fmt.Sprintf("%s,%s=%s", result, key, val)
		}
	}

	return result
}

func (api *ApplicationAPI) downloadNewScripts(serverName string, serverIP string) error {
	log := log.WithField("server", serverName)

	log.Info("Clear current root directory, loading new scripts...")

	_, stderr, err := api.execCommand(serverIP, api.getCommonExecCommand())
	if err != nil {
		return errors.Wrap(err, stderr)
	}

	return nil
}

func (api *ApplicationAPI) UpgradeControlPlane(version string) {
	log.Info("Executing controlplane upgrade...")

	for i := 1; i <= config.Get().MasterCount; i++ {
		serverName := fmt.Sprintf(config.Get().MasterServers.NamePattern, i)

		log := log.WithField("master", serverName)

		serverIP, err := api.waitForServer(serverName)
		if err != nil {
			log.WithError(err).Fatal()
		}

		if err = api.downloadNewScripts(serverName, serverIP); err != nil {
			log.WithError(err).Fatal()
		}

		log.Info("Upgrade controlplane...")

		cmd := "/root/scripts/upgrade-controlplane.sh"

		if len(version) > 0 {
			cmd = fmt.Sprintf("KUBERNETES_VERSION=%s %s", version, cmd)
		}

		stdout, stderr, err := api.execCommand(serverIP, cmd)
		if err != nil {
			log.WithError(err).Fatal(stderr)
		}

		log.Debugf("stdout=%s", stdout)
		log.Debugf("stderr=%s", stderr)
	}

	log.Info("Cluster upgraded!")
}

func (api *ApplicationAPI) CreateFirewall(createControlPlane, createWorker bool) error { //nolint:funlen
	log.Info("Creating firewall...")

	if !createControlPlane && !createWorker {
		return errors.New("nothing to create, please specify at least one firewall type")
	}

	_, anyIPv4, _ := net.ParseCIDR("0.0.0.0/0")
	_, anyIPv6, _ := net.ParseCIDR("::/0")
	_, clusterNetwork, _ := net.ParseCIDR(config.Get().IPRange)

	sharedRules := []hcloud.FirewallRule{
		{
			Direction:   hcloud.FirewallRuleDirectionIn,
			SourceIPs:   []net.IPNet{*anyIPv4, *anyIPv6},
			Protocol:    "tcp",
			Port:        hcloud.String("22"),
			Description: hcloud.String("SSH to server"),
		},
		{
			Direction:   hcloud.FirewallRuleDirectionIn,
			SourceIPs:   []net.IPNet{*clusterNetwork},
			Protocol:    "udp",
			Port:        hcloud.String("8285"),
			Description: hcloud.String("flannel overlay network - udp backend"),
		},
		{
			Direction:   hcloud.FirewallRuleDirectionIn,
			SourceIPs:   []net.IPNet{*clusterNetwork},
			Protocol:    "udp",
			Port:        hcloud.String("8472"),
			Description: hcloud.String("flannel overlay network - vxlan backend"),
		},
	}

	// https://kubernetes.io/docs/reference/ports-and-protocols/
	controlPlane := hcloud.FirewallCreateOpts{
		Name: config.Get().ClusterName + "-controlplane",
		Labels: map[string]string{
			"cluster": config.Get().ClusterName,
		},
		ApplyTo: []hcloud.FirewallResource{
			{
				Type: hcloud.FirewallResourceTypeLabelSelector,
				LabelSelector: &hcloud.FirewallResourceLabelSelector{
					Selector: "role=master",
				},
			},
		},
		Rules: append(sharedRules, []hcloud.FirewallRule{
			{
				Direction:   hcloud.FirewallRuleDirectionIn,
				SourceIPs:   []net.IPNet{*anyIPv4, *anyIPv6}, // flannel do not start if only clusternetwork
				Protocol:    "tcp",
				Port:        hcloud.String("6443"),
				Description: hcloud.String("Kubernetes API server"),
			},
			{
				Direction:   hcloud.FirewallRuleDirectionIn,
				SourceIPs:   []net.IPNet{*anyIPv4, *anyIPv6}, // other master nodes can not connect if only clusternetwork
				Protocol:    "tcp",
				Port:        hcloud.String("2379-2380"),
				Description: hcloud.String("etcd server client API"),
			},
			{
				Direction:   hcloud.FirewallRuleDirectionIn,
				SourceIPs:   []net.IPNet{*clusterNetwork},
				Protocol:    "tcp",
				Port:        hcloud.String("10250"),
				Description: hcloud.String("Kubelet API"),
			},
			{
				Direction:   hcloud.FirewallRuleDirectionIn,
				SourceIPs:   []net.IPNet{*clusterNetwork},
				Protocol:    "tcp",
				Port:        hcloud.String("10259"),
				Description: hcloud.String("kube-scheduler"),
			},
			{
				Direction:   hcloud.FirewallRuleDirectionIn,
				SourceIPs:   []net.IPNet{*clusterNetwork},
				Protocol:    "tcp",
				Port:        hcloud.String("10257"),
				Description: hcloud.String("kube-controller-manager"),
			},
		}...),
	}

	workers := hcloud.FirewallCreateOpts{
		Name: config.Get().ClusterName + "-workers",
		Labels: map[string]string{
			"cluster": config.Get().ClusterName,
		},
		ApplyTo: []hcloud.FirewallResource{
			{
				Type: hcloud.FirewallResourceTypeLabelSelector,
				LabelSelector: &hcloud.FirewallResourceLabelSelector{
					Selector: "hcloud/node-group",
				},
			},
		},
		Rules: append(sharedRules, []hcloud.FirewallRule{
			{
				Direction:   hcloud.FirewallRuleDirectionIn,
				SourceIPs:   []net.IPNet{*clusterNetwork},
				Protocol:    "tcp",
				Port:        hcloud.String("10250"),
				Description: hcloud.String("Kubelet API"),
			},
			{
				Direction:   hcloud.FirewallRuleDirectionIn,
				SourceIPs:   []net.IPNet{*clusterNetwork},
				Protocol:    "tcp",
				Port:        hcloud.String("30000-32767"),
				Description: hcloud.String("NodePort Services"),
			},
		}...),
	}

	if createControlPlane {
		if _, _, err := api.hcloudClient.Firewall.Create(ctx, controlPlane); err != nil {
			return errors.Wrap(err, "can not create controlplane firewall")
		}
	}

	if createWorker {
		if _, _, err := api.hcloudClient.Firewall.Create(ctx, workers); err != nil {
			return errors.Wrap(err, "can not create workers firewall")
		}
	}

	return nil
}
