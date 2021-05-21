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
apiVersion: kubeadm.k8s.io/v1beta2
kind: ClusterConfiguration
controlPlaneEndpoint: $MASTER_LB:6443
networking:
  podSubnet: "10.244.0.0/16" # --pod-network-cidr
---
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
featureGates:
  RotateKubeletServerCertificate: true
serverTLSBootstrap: true
evictionHard:
  memory.available: "100Mi"
  nodefs.available: "10%"
  nodefs.inodesFree: "5%"
EOF

kubeadm init --upload-certs --config=/root/scripts/kubeadm-config.yaml

export KUBECONFIG=/etc/kubernetes/admin.conf
kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml

kubectl -n kube-system patch deployment coredns --patch "$(cat /root/scripts/patch-coredns.yaml)"

kubectl -n kube-system create secret generic hcloud --from-literal=token=$HCLOUD_TOKEN
kubectl -n kube-system create secret generic hcloud-csi --from-literal=token=$HCLOUD_TOKEN

kubectl apply -f https://github.com/hetznercloud/hcloud-cloud-controller-manager/releases/latest/download/ccm.yaml
kubectl apply -f https://raw.githubusercontent.com/hetznercloud/csi-driver/master/deploy/kubernetes/hcloud-csi.yml

kubectl -n kube-system patch deployment hcloud-cloud-controller-manager --patch "$(cat /root/scripts/patch-ccm.yaml)"

# create token for nodes
cp /root/scripts/common-install.sh /root/scripts/cloud-init.sh
kubeadm token create --ttl=0 --print-join-command >> /root/scripts/cloud-init.sh

# create token for master
kubeadm token create --print-join-command --certificate-key `kubeadm init phase upload-certs --upload-certs | tail -1` > /root/scripts/join-master.sh

HCLOUD_CLOUD_INIT=`cat /root/scripts/cloud-init.sh | base64 -w 0`
kubectl -n kube-system create configmap hcloud-init --from-literal=bootstrap=$HCLOUD_CLOUD_INIT

kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
kubectl apply -f /root/scripts/deploy
