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

: ${EXTERNAL_IP:='10.0.0.2'}
: ${INTERNAL_IP:='10.0.0.2'}

echo "Migrating from $EXTERNAL_IP to $INTERNAL_IP"

if [ "$EXTERNAL_IP" == "10.0.0.2" ]; then
  echo "No external IP found"
  exit 1
fi

cat<<EOF > /etc/netplan/99-kubeadm.yaml
network:
  version: 2
  ethernets:
    enp7s0:
      dhcp4: true
      addresses:
      - $(hostname -I | awk '{print $2}')/32
      routes:
      - to: 10.96.0.0/12
        scope: link
EOF
chmod 600 /etc/netplan/*
ip route add 10.96.0.0/12 dev enp7s0 proto static scope link

sed -i "s/$EXTERNAL_IP/$INTERNAL_IP/" /etc/kubernetes/kubelet.conf

systemctl restart kubelet