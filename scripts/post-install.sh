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

: ${KUBECONFIG_PATH:='/etc/kubernetes/admin.conf'}
: ${SCRIPT_PATH:='/root'}

export KUBECONFIG=$KUBECONFIG_PATH

# create annotation for recent node deletion
kubectl annotate node --overwrite -lnode-role.kubernetes.io/master cluster-autoscaler.kubernetes.io/scale-down-disabled=true
kubectl annotate node --overwrite -lnode-role.kubernetes.io/control-plane cluster-autoscaler.kubernetes.io/scale-down-disabled=true

OS_ARCH=$(dpkg --print-architecture)

# install helm for kubernetes deploymens
curl -o helm.tar.gz "https://get.helm.sh/helm-v3.11.3-linux-${OS_ARCH}.tar.gz"
tar -xvf helm.tar.gz
mv "linux-${OS_ARCH}/helm" /usr/local/bin/helm
rm helm.tar.gz
rm -rf "linux-${OS_ARCH}"

# delete all tokens
kubeadm token list -o jsonpath='{.token}{"\n"}' | xargs kubeadm token delete

# create token to join worker nodes
cp /root/scripts/common-install.sh /root/scripts/cloud-init.sh
JOIN=$(kubeadm token create --ttl=0 --print-join-command)

echo "$JOIN --cri-socket=unix:///run/containerd/containerd.sock" >> /root/scripts/cloud-init.sh

# create autoscaler configuration
HCLOUD_CLOUD_INIT=$(base64 -w 0 < /root/scripts/cloud-init.sh)
kubectl -n kube-system delete configmap hcloud-init || true
kubectl -n kube-system create configmap hcloud-init --from-literal=bootstrap="$HCLOUD_CLOUD_INIT"
# shutdown current autoscaler that have wrong cloud-init configuration
kubectl -n kube-system scale deploy hcloud-k8s-ctl-hetzner-cluster-autoscaler --replicas=0 || true

# install dependencies
helm dep up $SCRIPT_PATH/scripts/chart

# install helm chart
helm upgrade --install hcloud-k8s-ctl --namespace kube-system $SCRIPT_PATH/scripts/chart --values=$SCRIPT_PATH/values.yaml

# patch objects
kubectl -n kube-system patch deploy coredns --patch "$(cat $SCRIPT_PATH/scripts/patch/coredns.yaml)"
kubectl -n kube-system patch ds kube-flannel-ds --patch "$(cat $SCRIPT_PATH/scripts/patch/kube-flannel-ds.yaml)"
kubectl -n kube-system patch ds kube-proxy --patch "$(cat $SCRIPT_PATH/scripts/patch/kube-proxy.yaml)"
