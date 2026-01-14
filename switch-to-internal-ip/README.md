# Change communication to master internal IP

## Default cluster config
```yaml
apiVersion: v1
kind: ConfigMap
data:
  ClusterConfiguration: |
    apiServer: {}
    apiVersion: kubeadm.k8s.io/v1beta4
    caCertificateValidityPeriod: 87600h0m0s
    certificateValidityPeriod: 8760h0m0s
    certificatesDir: /etc/kubernetes/pki
    clusterName: kubernetes
    controlPlaneEndpoint: 10.0.0.2:6443
    controllerManager: {}
    dns: {}
    encryptionAlgorithm: RSA-2048
    etcd:
      local:
        dataDir: /var/lib/etcd
        extraArgs:
        - name: auto-compaction-retention
          value: 10h
    imageRepository: registry.k8s.io
    kind: ClusterConfiguration
    kubernetesVersion: v1.33.7
    networking:
      dnsDomain: cluster.local
      podSubnet: 10.244.0.0/16
      serviceSubnet: 10.96.0.0/12
    proxy: {}
    scheduler: {}
```

```bash
export KUBECONFIG=/etc/kubernetes/admin.conf
export EDITOR=nano
# change to internal IP
kubectl -n kube-system edit cm kubeadm-config
kubectl -n kube-system edit cm kube-proxy

cat > /tmp/kubeadm-config.yaml <<EOF
apiVersion: kubeadm.k8s.io/v1beta4
kind: ClusterConfiguration
controlPlaneEndpoint: "10.0.0.6:6443"  # Load balancer
apiServer:
  certSANs:
  - "1.1.1.1"       # LoadBalancer external IP
  - "10.0.0.6"      # LoadBalancer internal IP
  - "10.0.0.5"      # current master internal IP
  - "master-3"      # current master hostname
  - "10.96.0.1"
  - "localhost"
etcd:
  local:
    serverCertSANs:
    - "1.1.1.1"       # current master external IP
    - "10.0.0.5"      # current master internal IP
    - "127.0.0.1"
    - "localhost"
    peerCertSANs:
    - "1.1.1.1"  # current master external IP
    - "10.0.0.5"      # current master internal IP
    - "127.0.0.1"
EOF

mkdir -p /backup
mv /etc/kubernetes/pki/apiserver.* /backup/
mv /etc/kubernetes/pki/etcd/server.* /backup/
mv /etc/kubernetes/pki/etcd/peer.* /backup/
mv /etc/kubernetes/pki/apiserver-etcd-client.* /backup/

# Regenerate certificates using this config
kubeadm init phase certs etcd-server --config=/tmp/kubeadm-config.yaml
kubeadm init phase certs etcd-peer --config=/tmp/kubeadm-config.yaml
kubeadm init phase certs apiserver-etcd-client --config=/tmp/kubeadm-config.yaml
kubeadm init phase certs apiserver --config=/tmp/kubeadm-config.yaml

# change configs to internal host
sed -i 's|https://[^:]*:6443|https://10.0.0.5:6443|g' /etc/kubernetes/*.conf
grep /etc/kubernetes/*.conf -e https

# list all members in etcd
etcdctl --endpoints=https://127.0.0.1:2379 \
--cacert=/etc/kubernetes/pki/etcd/ca.crt \
--cert=/etc/kubernetes/pki/etcd/server.crt \
--key=/etc/kubernetes/pki/etcd/server.key \
member list

# update to internal
etcdctl --endpoints=https://127.0.0.1:2379 \
--cacert=/etc/kubernetes/pki/etcd/ca.crt \
--cert=/etc/kubernetes/pki/etcd/server.crt \
--key=/etc/kubernetes/pki/etcd/server.key \
member update 6fcdf0943fc247ef \
--peer-urls=https://10.0.0.5:2380

# check health
etcdctl --endpoints=https://127.0.0.1:2379 \
--cacert=/etc/kubernetes/pki/etcd/ca.crt \
--cert=/etc/kubernetes/pki/etcd/server.crt \
--key=/etc/kubernetes/pki/etcd/server.key \
endpoint health --cluster

# check status
etcdctl --endpoints=https://127.0.0.1:2379 \
--cacert=/etc/kubernetes/pki/etcd/ca.crt \
--cert=/etc/kubernetes/pki/etcd/server.crt \
--key=/etc/kubernetes/pki/etcd/server.key \
endpoint status --cluster
```