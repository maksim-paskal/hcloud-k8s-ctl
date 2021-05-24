# High available Kubernetes cluster on Hetzner Cloud with Autoscaling
## Motivation
AWS has eksctl tool for creating kubernetes cluster - Hetzner Cloud have no official tool for creating kubernetes cluster. This tool will create new production ready kubernetes cluster on Hetzner Cloud with minium user interaction. New cluster will be avaible in High availability mode with automatic cluster autoscaling and automatic volume creation.

## Preparation
- login to https://console.hetzner.cloud/ and create new project
- select project, select in menu Security -> API Tokens and create new "Read & Write" token
- save token to `.hcloudauth` file in current dirrectory

## Install binnary
MacOS
```bash
brew install maksim-paskal/tap/hcloud-k8s-ctl
```
for other OS download binnary from [release pages](https://github.com/maksim-paskal/hcloud-k8s-ctl/releases)
## Create kubernetes cluster
This will create 3 instance with 1 load balancer for kubernetes control plane and 1 kubernetes worker node,after successful installation cluster will have:
- [Kubernetes Autoscaler](https://github.com/kubernetes/autoscaler/releases/tag/cluster-autoscaler-1.21.0)
- [Flannel](https://github.com/flannel-io/flannel)
- [Kubernetes Cloud Controller Manager for Hetzner Cloud](https://github.com/hetznercloud/hcloud-cloud-controller-manager)
- [Container Storage Interface driver for Hetzner Cloud](https://github.com/hetznercloud/csi-driver)
- [Kubernetes Metrics Server](https://github.com/kubernetes-sigs/metrics-server)
- [Simple CSR approver for Kubernetes](https://github.com/kontena/kubelet-rubber-stamp)

for HA needs odd number of master nodes (minimum 3) https://etcd.io/docs/v3.4/faq/#why-an-odd-number-of-cluster-members

create simle configuration file `config.yaml` full configuration example [here](https://github.com/maksim-paskal/hcloud-k8s-ctl/blob/main/examples/config-full.yaml)
```yaml
# Hetzner cloud internal network CIDR
ipRange: "10.0.0.0/16"
# servers for kuberntes master (recomended 3)
masterCount: 3
```
and start application
```bash
# create 3 instance with 1 load balancer
# kubernetes autoscaler will create 1 worker node
hcloud-k8s-ctl -action=create
```
all nodes in cluster initialized with official kubeadm - for all nodes use this [script](https://github.com/maksim-paskal/hcloud-k8s-ctl/blob/main/scripts/common-install.sh) for master initializing this [script](https://github.com/maksim-paskal/hcloud-k8s-ctl/blob/main/scripts/init-master.sh) 

## Cleanup
```bash
hcloud-k8s-ctl -action=delete
```

