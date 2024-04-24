# High available Kubernetes cluster on Hetzner Cloud with Autoscaling

## Motivation

AWS has eksctl tool for creating kubernetes cluster - Hetzner Cloud have no official tool for creating kubernetes cluster. This tool will create new production ready kubernetes clusters on Hetzner Cloud with minimum user interaction. New cluster will be available in High availability mode with automatic cluster autoscaling and automatic volume creation

## Preparation

- login to <https://console.hetzner.cloud> and create new project
- select project, select in menu Security -> API Tokens and create new "Read & Write" token
- save token to `.hcloudauth` file in current directory

## Install binnary

MacOS

```bash
brew install maksim-paskal/tap/hcloud-k8s-ctl
```

for other OS download binnary from [release pages](https://github.com/maksim-paskal/hcloud-k8s-ctl/releases)

## Create kubernetes cluster

This will create kubernetes cluster in Hetzner Cloud Europe region with 3 instances, 1 load balancer for the kubernetes control plane and 1 kubernetes worker node, after successful installation the cluster will have:

- [Kubernetes v1.28](https://github.com/kubernetes/kubernetes)
- [Kubernetes Autoscaler](https://github.com/kubernetes/autoscaler)
- [Flannel](https://github.com/flannel-io/flannel)
- [Kubernetes Cloud Controller Manager for Hetzner Cloud](https://github.com/hetznercloud/hcloud-cloud-controller-manager)
- [Container Storage Interface driver for Hetzner Cloud](https://github.com/hetznercloud/csi-driver)
- [Kubernetes Metrics Server](https://github.com/kubernetes-sigs/metrics-server)
- [Simple CSR approver for Kubernetes](https://github.com/postfinance/kubelet-csr-approver)
- [Docker registry (optional)](https://github.com/distribution/distribution)
- [NFS Provisioner for Kubernetes (optional)](https://github.com/kubernetes-sigs/nfs-subdir-external-provisioner)

for HA needs odd number of master nodes (minimum 3) <https://etcd.io/docs/v3.4/faq/#why-an-odd-number-of-cluster-members>

Create a simple configuration file `config.yaml` full configuration example [here](https://github.com/maksim-paskal/hcloud-k8s-ctl/blob/main/e2e/configs/full.yaml)

```yaml
# kubeconfig path
kubeConfigPath: ~/.kube/hcloud
# Hetzner cloud internal network CIDR
ipRange: "10.0.0.0/16"
# servers for kubernetes master (recommended 3)
# for development purposes cluster can have 1 master node  
# in this case cluster will be created without load balancer and pods can schedule on master
masterCount: 3
```

customize configuration file for your needs

```yaml
# kubeconfig path
kubeConfigPath: ~/.kube/hcloud
# Hetzner cloud internal network CIDR
ipRange: "10.0.0.0/16"
# servers for kubernetes master (recommended 3)
# for development purposes cluster can have 1 master node  
# in this case cluster will be created without load balancer and pods can schedule on master
masterCount: 3
# server components for all nodes in cluster
serverComponents:
  kubernetes:
    # customize kubertenes version
    version: 1.25.14-1.1
  docker:
    # customize apt package version for docker install
    # apt-cache madison docker-ce
    version: 5:24.0.6-1~ubuntu.$(lsb_release -rs)~$(lsb_release -cs)
  containerd:
    # customize apt package version for containerd install
    # apt-cache madison containerd.io
    version: 1.6.24-1
# add autoscaler chart extra values
cluster-autoscaler:
  replicaCount: 3
  resources:
    requests:
      cpu: 200m
      memory: 300Mi
# add some custom script for all nodes in cluster
preStartScript: |
  # add some custom cron job on node
  crontab <<EOF
  0 0 * * * /usr/bin/docker system prune -af
  EOF

  # add containerd config for some registries
  mkdir -p /etc/containerd/certs.d/some-registry.io
  cat > /etc/containerd/certs.d/some-registry.io/hosts.toml <<EOF
  server = "https://some-registry.io"

  [host."http://10.10.10.10:5000"]
  capabilities = ["pull", "resolve"]
  EOF
```

<!--- move_e2e_details_start -->
<details><summary>Kubernetes v1.25 in Europe</summary>

```yaml
ipRange: "10.0.0.0/16"
masterCount: 3
serverComponents:
  kubernetes:
    version: 1.25.14-1.1
  docker:
    version: 5:24.0.6-1~ubuntu.$(lsb_release -rs)~$(lsb_release -cs)
  containerd:
    version: 1.6.24-1
cluster-autoscaler:
  replicaCount: 3
  resources:
    requests:
      cpu: 100m
      memory: 300Mi
preStartScript: |
  # add some custom cron job on node
  crontab <<EOF
  0 0 * * * /usr/bin/docker system prune -af
  EOF

  # add containerd config for some registries
  mkdir -p /etc/containerd/certs.d/some-registry.io
  cat > /etc/containerd/certs.d/some-registry.io/hosts.toml <<EOF
  server = "https://some-registry.io"

  [host."http://10.10.10.10:5000"]
  capabilities = ["pull", "resolve"]
  EOF
```
</details>
<details><summary>Kubernetes v1.26 in Europe</summary>

```yaml
ipRange: "10.0.0.0/16"
masterCount: 3
serverComponents:
  kubernetes:
    version: 1.26.9-1.1
  docker:
    version: 5:24.0.6-1~ubuntu.$(lsb_release -rs)~$(lsb_release -cs)
  containerd:
    version: 1.6.24-1

```
</details>
<details><summary>Kubernetes v1.27 in Europe</summary>

```yaml
ipRange: "10.0.0.0/16"
masterCount: 3
serverComponents:
  kubernetes:
    version: 1.27.6-1.1
  docker:
    version: 5:24.0.6-1~ubuntu.$(lsb_release -rs)~$(lsb_release -cs)
  containerd:
    version: 1.6.24-1

```
</details>
<details><summary>Kubernetes v1.28 in Europe</summary>

```yaml
ipRange: "10.0.0.0/16"
masterCount: 3
serverComponents:
  kubernetes:
    version: 1.28.2-1.1
  docker:
    version: 5:24.0.6-1~ubuntu.$(lsb_release -rs)~$(lsb_release -cs)
  containerd:
    version: 1.6.24-1

```
</details>
<details><summary>Kubernetes v1.28 in US East</summary>

```yaml
ipRange: "10.0.0.0/16"
masterCount: 3
networkZone: us-east
location: ash
datacenter: ash-dc1
masterServers:
  servertype: cpx21
serverComponents:
  kubernetes:
    version: 1.28.2-1.1
  docker:
    version: 5:24.0.6-1~ubuntu.$(lsb_release -rs)~$(lsb_release -cs)
  containerd:
    version: 1.6.24-1
cluster-autoscaler:
  autoscalingGroups:
  - name: CPX51:ASH:cpx51-ash
    minSize: 1
    maxSize: 20
```
</details>
<details><summary>Kubernetes v1.28 in Europe (ARM64 architecture)</summary>

```yaml
ipRange: "10.0.0.0/16"
masterCount: 3
serverComponents:
  ubuntu:
    architecture: arm
  kubernetes:
    version: 1.28.2-1.1
  docker:
    version: 5:24.0.6-1~ubuntu.$(lsb_release -rs)~$(lsb_release -cs)
  containerd:
    version: 1.6.24-1
masterServers:
  servertype: cax11
cluster-autoscaler:
  autoscalingGroups:
  - name: CAX41:FSN1:cax-fsn1
    minSize: 1
    maxSize: 20
```
</details>

<!--- move_e2e_details_end -->

```bash
# create 3 instance with 1 load balancer
# kubernetes autoscaler will create 1 worker node
hcloud-k8s-ctl -action=create
```

all nodes in cluster initialized with official kubeadm - for all nodes use this [script](https://github.com/maksim-paskal/hcloud-k8s-ctl/blob/main/scripts/common-install.sh), for master initializing this [script](https://github.com/maksim-paskal/hcloud-k8s-ctl/blob/main/scripts/init-master.sh), for initial applications in cluster this [script](https://github.com/maksim-paskal/hcloud-k8s-ctl/blob/main/scripts/post-install.sh)

## Access to cluster

```bash
export KUBECONFIG=$HOME/.kube/hcloud

kubectl get no
```

## Patch already created cluster

```bash
hcloud-k8s-ctl -action=patch-cluster
```

## List available location/datacenter/servertype at Hezner

```bash
hcloud-k8s-ctl -action=list-configurations
```

## Delete already created cluster

```bash
hcloud-k8s-ctl -action=delete
```

## To install NFS provisioner

You can easy install NFS provisioner for your cluster adding to your `config.yaml` next lines

```yaml
deployments:
  nfs:
    nfs-subdir-external-provisioner:
      enabled: true
    server:
      enabled: true
```

It will install [NFS Provisioner for Kubernetes (optional)](https://github.com/kubernetes-sigs/nfs-subdir-external-provisioner) with [NFS Server and Storage Class](scripts/chart/templates/nfs-server.yaml)

you can easy create new NFS volumes to your pod with this PersistentVolumeClaim

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: pvc-nfs
spec:
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 10Gi
  storageClassName: nfs
```
