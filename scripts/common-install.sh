#!/usr/bin/env bash

# Copyright paskal.maksim@gmail.com
#
# Licensed under the Apache License, Version 2.0 (the "License")
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
set -ex

export KUBERNETES_VERSION=1.21.1
export DOCKER_VERSION=5:20.10.10~3-0~ubuntu-focal
export DEBIAN_FRONTEND=noninteractive
export HOME=/root/

apt update
apt install -y apt-transport-https ca-certificates curl software-properties-common nfs-common

# disable swap
swapoff -a

# disable 111/udp 111/tcp port
systemctl stop rpcbind.service
systemctl stop rpcbind.socket
systemctl stop rpcbind.target
systemctl disable rpcbind.service
systemctl disable rpcbind.socket
systemctl disable rpcbind.target

curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
curl -fsSLo /usr/share/keyrings/kubernetes-archive-keyring.gpg https://packages.cloud.google.com/apt/doc/apt-key.gpg

cat <<EOF >/etc/apt/sources.list.d/kubernetes.list
deb [signed-by=/usr/share/keyrings/kubernetes-archive-keyring.gpg] https://apt.kubernetes.io/ kubernetes-xenial main
EOF
cat <<EOF >/etc/apt/sources.list.d/docker.list
deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable
EOF

apt update
apt install -y docker-ce=$DOCKER_VERSION docker-ce-cli=$DOCKER_VERSION containerd.io
apt-mark hold docker-ce docker-ce-cli containerd.io

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
  "storage-driver": "overlay2",
  "insecure-registries": ["10.100.0.11:5000"]
}
EOF
mkdir -p /etc/systemd/system/docker.service.d

cat <<EOF | tee /etc/sysctl.conf
fs.inotify.max_user_watches=524288
fs.inotify.max_user_instances=8192
vm.max_map_count=524288
EOF
sysctl -p

apt-get install -y kubelet=${KUBERNETES_VERSION}-00 kubeadm=${KUBERNETES_VERSION}-00 kubectl=${KUBERNETES_VERSION}-00
apt-mark hold kubelet kubeadm kubectl

INTERNAL_IP=$(hostname -I | awk '{print $2}')
cat <<EOF | tee /etc/default/kubelet
KUBELET_EXTRA_ARGS=--cgroup-driver=systemd --cloud-provider=external --node-ip=$INTERNAL_IP --v=2
EOF

systemctl daemon-reload
systemctl restart docker
systemctl restart kubelet

