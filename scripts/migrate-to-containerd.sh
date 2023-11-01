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

export KUBECONFIG=/etc/kubernetes/kubelet.conf

# stop kubelet on host
systemctl stop kubelet

# start all required services on host
systemctl start docker docker.socket

# add arguments to kubelet to use containerd
cat <<EOF | tee /var/lib/kubelet/kubeadm-flags.env
KUBELET_KUBEADM_ARGS="--container-runtime=remote --container-runtime-endpoint=unix:///run/containerd/containerd.sock --pod-infra-container-image=registry.k8s.io/pause:3.4.1"
EOF

# change node cri-socket to containerd socket
kubectl annotate node "$HOSTNAME" --overwrite kubeadm.alpha.kubernetes.io/cri-socket=unix:///run/containerd/containerd.sock

# stop all docker containers if running
docker kill $(docker ps -q) || true

# clean docker
docker system prune -af
docker volume prune -f

# stop docker services
systemctl stop docker docker.socket

# clean logs and docker
rm -rf /var/log/pods /var/log/containers /var/lib/docker

# configure containerd
/root/scripts/upgrade-worker.sh
