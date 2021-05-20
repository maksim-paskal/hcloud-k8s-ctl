# High available Kubernetes cluster on Hetzner Cloud with Autoscaling
## Motivation
Kubernetes Autoscaler start to support [hetzner as cloud provider](https://github.com/kubernetes/autoscaler/releases/tag/cluster-autoscaler-1.21.0) - this gives an opportunity to create kubernetes cluster that will make scaleup and scaledown on requested resources in cluster

## Preparation
- login to https://console.hetzner.cloud/ and create new project
- select project, select in menu Security -> API Tokens and create new "Read & Write" token
- install https://github.com/hetznercloud/cli
- configure cli with new token `hcloud context create k8s-cluster`

## Create kubernetes cluster
this will create 3 instance with 1 load balancer for kubernetes control plane and 1 kubernetes worker node

after successful installation cluster will have:
- [Kubernetes Autoscaler](https://github.com/kubernetes/autoscaler/releases/tag/cluster-autoscaler-1.21.0)
- [Flannel](https://github.com/flannel-io/flannel)
- [Kubernetes Cloud Controller Manager for Hetzner Cloud](https://github.com/hetznercloud/hcloud-cloud-controller-manager)
- [Container Storage Interface driver for Hetzner Cloud](https://github.com/hetznercloud/csi-driver)
- [Kubernetes Metrics Server](https://github.com/kubernetes-sigs/metrics-server)
- [Simple CSR approver for Kubernetes](https://github.com/kontena/kubelet-rubber-stamp)

for HA needs odd number of master nodes (minimum 3) https://etcd.io/docs/v3.4/faq/#why-an-odd-number-of-cluster-members

```bash
# create 3 instance with 1 load balancer
make hcloud-create

# create HA cluster on 3 instance
make create-master hcloud_token=<Hetzner Cloud API token>

# kubernetes cluster will add 1 kubernetes node
```

## Cleanup
```bash
make hcloud-destroy
```

