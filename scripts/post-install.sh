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

# create custom user values
echo "$VALUES" | base64 -d > $SCRIPT_PATH/values.yaml

# install helm for kubernetes deploymens
curl -o helm.tar.gz https://get.helm.sh/helm-v3.4.2-linux-amd64.tar.gz
tar -xvf helm.tar.gz
mv linux-amd64/helm /usr/local/bin/helm
rm helm.tar.gz
rm -rf linux-amd64

# delete all tokens
kubeadm token list -o jsonpath='{.token}{"\n"}' | xargs kubeadm token delete

# create token for nodes
cp /root/scripts/common-install.sh /root/scripts/cloud-init.sh
kubeadm token create --ttl=0 --print-join-command >> /root/scripts/cloud-init.sh

# create autoscaler configuration
HCLOUD_CLOUD_INIT=$(base64 -w 0 < /root/scripts/cloud-init.sh)
kubectl -n kube-system delete configmap hcloud-init || true
kubectl -n kube-system create configmap hcloud-init --from-literal=bootstrap="$HCLOUD_CLOUD_INIT"
kubectl -n kube-system rollout restart deploy cluster-autoscaler || true

# install helm chart
helm upgrade --install hcloud-k8s-ctl --namespace kube-system $SCRIPT_PATH/scripts/chart --values=$SCRIPT_PATH/values.yaml

# patch coredns
kubectl -n kube-system patch deployment coredns --patch "$(cat $SCRIPT_PATH/scripts/patch-coredns.yaml)"
