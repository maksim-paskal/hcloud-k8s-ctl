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
package drain

import (
	"context"
	"time"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/maksim-paskal/hcloud-k8s-ctl/pkg/config"
	"github.com/maksim-paskal/hcloud-k8s-ctl/pkg/utils"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// ClusterDrainer is a struct for drain cluster.
type ClusterDrainer struct {
	hcloudClient      *hcloud.Client
	WaitTime          time.Duration
	MasterSelector    string
	NodeGroupSelector string
}

func NewClusterDrainer(hcloudClient *hcloud.Client) *ClusterDrainer {
	return &ClusterDrainer{
		hcloudClient: hcloudClient,
		WaitTime:     3 * time.Second, //nolint:gomnd
	}
}

func (api *ClusterDrainer) DeleteCluster(ctx context.Context) {
	for ctx.Err() == nil {
		if err := api.processDeleteCluster(ctx); err != nil {
			log.WithError(err).Error("delete cluster")
		} else {
			break
		}

		utils.SleepContext(ctx, api.WaitTime)
	}
}

func (api *ClusterDrainer) processDeleteCluster(ctx context.Context) error {
	if err := api.deleteNetworks(ctx); err != nil {
		return err
	}

	if err := api.deleteSSHKeys(ctx); err != nil {
		return err
	}

	if err := api.deleteLoadBalancer(ctx); err != nil {
		return err
	}

	if err := api.deleteServers(ctx); err != nil {
		return err
	}

	if err := api.deletePlacementGroup(ctx); err != nil {
		return err
	}

	if err := api.deleteFirewalls(ctx); err != nil {
		return err
	}

	if err := api.deleteVolumes(ctx); err != nil {
		return err
	}

	return nil
}

func (api *ClusterDrainer) deleteNetworks(ctx context.Context) error {
	for ctx.Err() == nil {
		k8sNetwork, _, _ := api.hcloudClient.Network.Get(ctx, config.Get().ClusterName)
		if k8sNetwork == nil {
			return nil
		}

		_, err := api.hcloudClient.Network.Delete(ctx, k8sNetwork)
		if err != nil {
			return errors.Wrap(err, "delete network")
		}
	}

	return errors.Wrap(ctx.Err(), "delete network")
}

func (api *ClusterDrainer) deleteSSHKeys(ctx context.Context) error {
	for ctx.Err() == nil {
		k8sSSHKey, _, _ := api.hcloudClient.SSHKey.Get(ctx, config.Get().ClusterName)
		if k8sSSHKey == nil {
			return nil
		}

		_, err := api.hcloudClient.SSHKey.Delete(ctx, k8sSSHKey)
		if err != nil {
			return errors.Wrap(err, "error deleting SSHKey")
		}
	}

	return errors.Wrap(ctx.Err(), "error deleting SSHKey")
}

func (api *ClusterDrainer) deleteLoadBalancer(ctx context.Context) error {
	for ctx.Err() == nil {
		allBalancers := make([]*hcloud.LoadBalancer, 0)

		k8sLoadBalancer, _, _ := api.hcloudClient.LoadBalancer.Get(ctx, config.Get().ClusterName)
		if k8sLoadBalancer != nil {
			allBalancers = append(allBalancers, k8sLoadBalancer)
		}

		loadBalancers, _, _ := api.hcloudClient.LoadBalancer.List(ctx, hcloud.LoadBalancerListOpts{
			ListOpts: hcloud.ListOpts{
				LabelSelector: "hcloud-ccm/service-uid",
			},
		})

		allBalancers = append(allBalancers, loadBalancers...)

		if len(allBalancers) == 0 {
			return nil
		}

		for _, loadBalancer := range allBalancers {
			_, err := api.hcloudClient.LoadBalancer.Delete(ctx, loadBalancer)
			if err != nil {
				return errors.Wrap(err, "error deleting LoadBalancer")
			}
		}
	}

	return errors.Wrap(ctx.Err(), "error deleting LoadBalancer")
}

func (api *ClusterDrainer) deleteServers(ctx context.Context) error {
	for ctx.Err() == nil {
		// get master nodes
		allServers, _, _ := api.hcloudClient.Server.List(ctx, hcloud.ServerListOpts{
			ListOpts: hcloud.ListOpts{
				LabelSelector: api.MasterSelector,
			},
		})

		// get worker nodes
		nodeServers, _, _ := api.hcloudClient.Server.List(ctx, hcloud.ServerListOpts{
			ListOpts: hcloud.ListOpts{
				LabelSelector: api.NodeGroupSelector,
			},
		})

		allServers = append(allServers, nodeServers...)

		if len(allServers) == 0 {
			return nil
		}

		for _, nodeServer := range allServers {
			_, _, err := api.hcloudClient.Server.DeleteWithResult(ctx, nodeServer)
			if err != nil {
				return errors.Wrapf(err, "error deleting Server=%s", nodeServer.Name)
			}
		}
	}

	return errors.Wrap(ctx.Err(), "error deleting Server")
}

func (api *ClusterDrainer) deletePlacementGroup(ctx context.Context) error {
	for ctx.Err() == nil {
		placementGroup, _, _ := api.hcloudClient.PlacementGroup.Get(ctx, config.Get().MasterServers.PlacementGroupName)
		if placementGroup == nil {
			return nil
		}

		_, err := api.hcloudClient.PlacementGroup.Delete(ctx, placementGroup)
		if err != nil {
			return errors.Wrapf(err, "error deleting PlacementGroup=%s", placementGroup.Name)
		}
	}

	return errors.Wrap(ctx.Err(), "error deleting PlacementGroup")
}

func (api *ClusterDrainer) deleteFirewalls(ctx context.Context) error {
	for ctx.Err() == nil {
		k8sFirewalls, _, _ := api.hcloudClient.Firewall.List(ctx, hcloud.FirewallListOpts{
			ListOpts: hcloud.ListOpts{
				LabelSelector: "cluster=" + config.Get().ClusterName,
			},
		})

		if len(k8sFirewalls) == 0 {
			return nil
		}

		for _, k8sFirewall := range k8sFirewalls {
			_, err := api.hcloudClient.Firewall.Delete(ctx, k8sFirewall)
			if err != nil {
				return errors.Wrap(err, "error deleting Firewall")
			}
		}
	}

	return errors.Wrap(ctx.Err(), "error deleting Firewall")
}

func (api *ClusterDrainer) deleteVolumes(ctx context.Context) error {
	for ctx.Err() == nil {
		k8sVolumes, _, _ := api.hcloudClient.Volume.List(ctx, hcloud.VolumeListOpts{})

		if len(k8sVolumes) == 0 {
			return nil
		}

		for _, k8sVolume := range k8sVolumes {
			_, err := api.hcloudClient.Volume.Delete(ctx, k8sVolume)
			if err != nil {
				return errors.Wrap(err, "error deleting Volume")
			}
		}
	}

	return errors.Wrap(ctx.Err(), "error deleting Volume")
}
