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

/root/scripts/common-install.sh

cat<<EOF > /root/scripts/kubeadm-config.yaml
apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
nodeRegistration:
  criSocket: "unix:///run/containerd/containerd.sock"
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
controlPlaneEndpoint: $MASTER_LB:6443
networking:
  podSubnet: "10.244.0.0/16" # --pod-network-cidr
EOF

kubeadm init --upload-certs --config=/root/scripts/kubeadm-config.yaml --v=10


# create join command to join to the cluster
CERTIFICATE_KEY=$(kubeadm init phase upload-certs --upload-certs | tail -1)
JOIN=$(kubeadm token create --print-join-command --certificate-key="$CERTIFICATE_KEY")

echo "$JOIN --cri-socket=unix:///run/containerd/containerd.sock" > /root/scripts/join-master.sh
