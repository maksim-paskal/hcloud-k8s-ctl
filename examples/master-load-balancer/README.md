```bash
# update master IP
nano master-load-balancer.yaml
# copy to static manifests
cp master-load-balancer.yaml /etc/kubernetes/manifests/master-load-balancer.yaml
# update kubelet to https://localhost:16443
nano /etc/kubernetes/kubelet.conf
# restart service
systemctl restart kubelet.service
```