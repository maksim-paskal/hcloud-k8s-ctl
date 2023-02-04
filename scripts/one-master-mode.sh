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

export KUBECONFIG='/etc/kubernetes/admin.conf'

# scale down all deployments to 1
kubectl -n kube-system scale deploy --all --replicas=1

# remove pdb for correct scaledown of worker nodes
kubectl -n kube-system delete pdb --all

# remove taint from node if taint found
kubectl taint nodes node-role.kubernetes.io/master- --all || true
kubectl taint nodes node-role.kubernetes.io/control-plane- --all || true
