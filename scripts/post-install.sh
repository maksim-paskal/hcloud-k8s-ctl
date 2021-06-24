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
: ${SCRTIPT_PATH:='/root'}

export KUBECONFIG=$KUBECONFIG_PATH

kubectl annotate node --overwrite -lnode-role.kubernetes.io/master cluster-autoscaler.kubernetes.io/scale-down-disabled=true

echo $VALUES | base64 -d > $SCRTIPT_PATH/scripts/chart/values.yaml

# install helm for kubernetes deploymens
curl -o helm.tar.gz https://get.helm.sh/helm-v3.4.2-linux-amd64.tar.gz
tar -xvf helm.tar.gz
mv linux-amd64/helm /usr/local/bin/helm
rm helm.tar.gz
rm -rf linux-amd64

helm template $SCRTIPT_PATH/scripts/chart | kubectl apply -f -

kubectl -n kube-system patch deployment coredns --patch "$(cat $SCRTIPT_PATH/scripts/patch-coredns.yaml)"