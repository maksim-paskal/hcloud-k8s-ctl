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

# stop kubelet on host
systemctl stop kubelet

# setup crictl
cat <<EOF | tee /etc/crictl.yaml
runtime-endpoint: unix:///var/run/containerd/containerd.sock
image-endpoint: unix:///var/run/containerd/containerd.sock
timeout: 10
debug: false
EOF

# start all required services on host
systemctl start containerd

# stop all containerd containers if running
crictl stopp $(crictl pods -q) || true
crictl rmp $(crictl pods -q) || true
crictl rm $(crictl ps -a -q) || true

# clear iptables
iptables -F && iptables -X
iptables -t nat -F && iptables -t nat -X
iptables -t raw -F && iptables -t raw -X
iptables -t mangle -F && iptables -t mangle -X

# kill all previous umount if running
pkill umount || true

# wait some time before umount
sleep 5s

# delete all pods
for mount in $(mount | grep '/var/lib/kubelet' | grep 'type nfs' | awk '{ print $3 }'); do umount --read-only --force --lazy $mount; done
for mount in $(mount | grep '/var/lib/kubelet' | awk '{ print $3 }'); do umount $mount; done

# wait some time before delete
sleep 5s

# delete pods folder
rm -rf /var/lib/kubelet/pods/ \
/var/lib/kubelet/plugins/ \
/var/lib/kubelet/pod-resources \
/var/lib/kubelet/device-plugins

# reconfigure containerd
/root/scripts/common-install.sh
