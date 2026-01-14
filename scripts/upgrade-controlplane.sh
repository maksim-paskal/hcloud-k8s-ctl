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

export INDEX=${HOSTNAME##*-}

# update kubernetes binaries
/root/scripts/common-install.sh

if [[ $INDEX -eq 1 ]]; then
  # update controlplane on first node
  kubeadm upgrade --ignore-preflight-errors=CoreDNSUnsupportedPlugins apply "$(kubeadm version -o short)" -f
else
  # update controlplane on other node
  kubeadm upgrade --ignore-preflight-errors=CoreDNSUnsupportedPlugins node
fi

# cleanup tmp files
rm -rf /etc/kubernetes/tmp/

systemctl restart kubelet
