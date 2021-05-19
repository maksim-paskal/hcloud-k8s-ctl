#!/bin/bash

set -ex

kubeadm reset -f

apt purge -y --allow-change-held-packages kube*
apt -y autoremove

iptables -F && iptables -X
iptables -t nat -F && iptables -t nat -X
iptables -t raw -F && iptables -t raw -X
iptables -t mangle -F && iptables -t mangle -X

rm -rf /root/* /var/lib/kubelet /etc/kubernetes /var/lib/etcd
