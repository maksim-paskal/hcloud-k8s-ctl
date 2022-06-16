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

export KUBERNETES_VERSION=1.21.13
export DOCKER_VERSION=5:20.10.13~3-0~ubuntu-focal
export CONTAINERD_VERSION=1.4.13-1

# to select all available versions, run
# apt-cache madison docker-ce containerd.io kubelet

export DEBIAN_FRONTEND=noninteractive
export HOME=/root/

# uninstall old versions if exists
dpkg --purge docker docker-engine docker.io containerd runc

apt update
apt install -y \
apt-transport-https \
ca-certificates \
curl \
software-properties-common \
nfs-common \
"linux-headers-$(uname -r)"

# hold linux kernel update on server restart
apt-mark hold "linux-image-$(uname -r)" "linux-headers-$(uname -r)"

# create new user to ssh into server
hcloud_user=hcloud-user
if ! id -u "$hcloud_user" > /dev/null 2>&1; then
  groupadd --gid 1000 $hcloud_user
  useradd -rm -d /home/$hcloud_user -s /bin/bash -g 1000 -u 1000 $hcloud_user
  mkdir -p /home/$hcloud_user/.ssh
  cp /root/.ssh/authorized_keys /home/$hcloud_user/.ssh
  chown -R $hcloud_user:$hcloud_user /home/$hcloud_user
  echo "$hcloud_user ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers
fi

cat > /etc/ssh/sshd_config <<EOF
AllowUsers hcloud-user

PermitRootLogin no
PasswordAuthentication no
ChallengeResponseAuthentication no
UsePAM yes
AuthenticationMethods publickey
PubkeyAuthentication yes
PermitEmptyPasswords no

PrintMotd no
AcceptEnv LANG LC_*
Subsystem sftp /usr/lib/openssh/sftp-server

AllowTcpForwarding no
X11Forwarding no
AllowAgentForwarding no
EOF

# restart sshd to apply new config
sshd -t
systemctl restart sshd.service

# disable swap
swapoff -a

# disable 111/udp 111/tcp port
systemctl stop rpcbind.service
systemctl stop rpcbind.socket
systemctl stop rpcbind.target
systemctl disable rpcbind.service
systemctl disable rpcbind.socket
systemctl disable rpcbind.target

mkdir -p /etc/apt/keyrings

rm -rf /usr/share/keyrings/docker-archive-keyring.gpg /usr/share/keyrings/kubernetes-archive-keyring.gpg

curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --dearmor -o /usr/share/keyrings/kubernetes-archive-keyring.gpg

cat <<EOF >/etc/apt/sources.list.d/kubernetes.list
deb [signed-by=/usr/share/keyrings/kubernetes-archive-keyring.gpg] https://apt.kubernetes.io/ kubernetes-xenial main
EOF
cat <<EOF >/etc/apt/sources.list.d/docker.list
deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable
EOF

apt update
apt-mark unhold docker-ce docker-ce-cli containerd.io
apt install -y docker-ce=$DOCKER_VERSION docker-ce-cli=$DOCKER_VERSION containerd.io=$CONTAINERD_VERSION
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
  "max-concurrent-uploads": 10,
  "default-ulimits": {
    "memlock": {
      "Hard": -1,
      "Name": "memlock",
      "Soft": -1
    }
  },
  "storage-driver": "overlay2",
  "insecure-registries": ["10.100.0.0/16"]
}
EOF
mkdir -p /etc/systemd/system/docker.service.d

cat <<EOF | tee /etc/sysctl.conf
fs.inotify.max_user_watches=524288
fs.inotify.max_user_instances=8192
vm.max_map_count=524288
vm.overcommit_memory=1
kernel.panic=10
kernel.panic_on_oops=1
EOF
sysctl -p

apt-mark unhold kubelet kubeadm kubectl
apt-get install -y kubelet=${KUBERNETES_VERSION}-00 kubeadm=${KUBERNETES_VERSION}-00 kubectl=${KUBERNETES_VERSION}-00
apt-mark hold kubelet kubeadm kubectl

INTERNAL_IP=$(hostname -I | awk '{print $2}')

mkdir -p /etc/kubernetes/kubelet/

# https://docs.hetzner.com/dns-console/dns/general/recursive-name-servers
cat > /etc/kubernetes/kubelet/resolv.conf <<EOF
nameserver 185.12.64.1
nameserver 185.12.64.2
EOF

# kubeadm config print init-defaults --component-configs KubeletConfiguration
cat > /etc/kubernetes/kubelet/config.yaml <<EOF
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
authentication:
  anonymous:
    enabled: false
  webhook:
    cacheTTL: 0s
    enabled: true
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
  webhook:
    cacheAuthorizedTTL: 0s
    cacheUnauthorizedTTL: 0s
cgroupDriver: systemd
clusterDNS:
- 10.96.0.10
clusterDomain: cluster.local
cpuManagerReconcilePeriod: 0s
evictionPressureTransitionPeriod: 0s
fileCheckFrequency: 0s
healthzBindAddress: 127.0.0.1
healthzPort: 10248
httpCheckFrequency: 0s
imageMinimumGCAge: 0s
logging: {}
nodeStatusReportFrequency: 0s
nodeStatusUpdateFrequency: 0s
resolvConf: /etc/kubernetes/kubelet/resolv.conf
rotateCertificates: true
runtimeRequestTimeout: 0s
shutdownGracePeriod: 0s
shutdownGracePeriodCriticalPods: 0s
staticPodPath: /etc/kubernetes/manifests
streamingConnectionIdleTimeout: 0s
syncFrequency: 0s
volumeStatsAggPeriod: 0s
featureGates:
  RotateKubeletServerCertificate: true
evictionHard:
  memory.available: "100Mi"
  nodefs.available: "10%"
  nodefs.inodesFree: "5%"
protectKernelDefaults: true
serializeImagePulls: false
serverTLSBootstrap: true
EOF

cat <<EOF | tee /etc/default/kubelet
KUBELET_CONFIG_ARGS=--config=/etc/kubernetes/kubelet/config.yaml
KUBELET_EXTRA_ARGS=--container-runtime=docker --cloud-provider=external --node-ip=$INTERNAL_IP --v=2
EOF

apt -y autoremove
apt -y autoclean

systemctl daemon-reload
systemctl restart docker
systemctl restart containerd
systemctl restart kubelet
