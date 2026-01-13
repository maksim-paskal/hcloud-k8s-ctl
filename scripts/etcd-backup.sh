#!/bin/sh

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

# /usr/local/sbin/etcd-backup.sh
# chmod +x /usr/local/sbin/etcd-backup.sh
set -ex
export ETCDCTL_API=3
BACKUP_DIR="/var/lib/etcd/backups"
BACKUP_NAME="etcd-snapshot-$(date +%Y%m%d%H%M%S).db"

mkdir -p $BACKUP_DIR

# Defragment the etcd database before taking a snapshot
etcdctl defrag --cluster \
--endpoints=https://127.0.0.1:2379 \
--cacert=/etc/kubernetes/pki/etcd/ca.crt \
--cert=/etc/kubernetes/pki/etcd/server.crt \
--key=/etc/kubernetes/pki/etcd/server.key

# Take a snapshot of the etcd database
etcdctl snapshot save $BACKUP_DIR/$BACKUP_NAME \
--endpoints=https://127.0.0.1:2379 \
--cacert=/etc/kubernetes/pki/etcd/ca.crt \
--cert=/etc/kubernetes/pki/etcd/server.crt \
--key=/etc/kubernetes/pki/etcd/server.key

# Optional: Remove backups older than 7 days
# find $BACKUP_DIR -type f -name "etcd-snapshot-*.db" -mtime +7 -exec rm {} \;

# Optional: Leave only last 5 backups
ls -1t $BACKUP_DIR/etcd-snapshot-*.db | tail -n +6 | xargs -r rm --