#!/bin/bash
set -ex

export KUBERNETES_VERSION=1.21.1
export DEBIAN_FRONTEND=noninteractive
export HOME=/root/
apt-get update && apt-get install -y apt-transport-https ca-certificates curl software-properties-common
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
cat <<EOF >/etc/apt/sources.list.d/kubernetes.list
deb https://apt.kubernetes.io/ kubernetes-xenial main
EOF
add-apt-repository   "deb [arch=amd64] https://download.docker.com/linux/ubuntu   $(lsb_release -cs)   stable"
apt-get update
apt-get upgrade -y
apt-get install -y kubelet=${KUBERNETES_VERSION}-00 kubeadm=${KUBERNETES_VERSION}-00 kubectl=${KUBERNETES_VERSION}-00
apt-mark hold kubelet kubeadm kubectl
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | apt-key add -
add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu bionic stable"
apt update
apt install -y docker-ce

INTERNAL_IP=`hostname -I | awk '{print $2}'`
cat <<EOF | tee /etc/default/kubelet
KUBELET_EXTRA_ARGS=--cloud-provider=external --node-ip=$INTERNAL_IP --v=2
EOF

mkdir -p /etc/docker/
cat > /etc/docker/daemon.json <<EOF
{
  "exec-opts": ["native.cgroupdriver=systemd"],
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "100m",
    "max-file": "10"
  },
  "live-restore": true,
  "max-concurrent-downloads": 10,
  "default-ulimits": {
    "memlock": {
      "Hard": -1,
      "Name": "memlock",
      "Soft": -1
    }
  },
  "storage-driver": "overlay2"
}
EOF
mkdir -p /etc/systemd/system/docker.service.d
systemctl daemon-reload
systemctl restart docker

cat <<EOF | tee /etc/sysctl.conf
fs.inotify.max_user_watches=524288
fs.inotify.max_user_instances=8192
vm.max_map_count=524288
EOF
sysctl -p

