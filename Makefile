KUBECONFIG=$(HOME)/.kube/hcloud

hcloud_token=<create-token>
id_rsa=$(HOME)/.ssh/id_rsa
id_rsa_pub=$(id_rsa).pub

master1_host=`hcloud server ip master-1`
master2_host=`hcloud server ip master-2`
master3_host=`hcloud server ip master-3`
master_lb=`hcloud load-balancer list k8s-master --output noheader --output columns=ipv4`
hcloud_datacenter=fsn1-dc14
hcloud_location=fsn1
master_instance_type=cx21
master_lb_type=lb11

hcloud-init:
	hcloud context delete k8s-cluster || true
	hcloud context create k8s-cluster
hcloud-create:
	hcloud network create --name k8s --ip-range 10.0.0.0/16
	hcloud network add-subnet k8s --network-zone eu-central --type server --ip-range 10.0.0.0/16
	hcloud ssh-key create --name k8s --public-key-from-file $(id_rsa_pub)

	hcloud server create --label role=master --type $(master_instance_type) --datacenter $(hcloud_datacenter) --name master-1 --image ubuntu-20.04 --ssh-key k8s --network k8s
	hcloud server create --label role=master --type $(master_instance_type) --datacenter $(hcloud_datacenter) --name master-2 --image ubuntu-20.04 --ssh-key k8s --network k8s
	hcloud server create --label role=master --type $(master_instance_type) --datacenter $(hcloud_datacenter) --name master-3 --image ubuntu-20.04 --ssh-key k8s --network k8s

	hcloud load-balancer create --name k8s-master --type $(master_lb_type) --location $(hcloud_location)
	hcloud load-balancer attach-to-network --network k8s k8s-master
	hcloud load-balancer add-target k8s-master --label-selector role=master
	hcloud load-balancer add-service k8s-master --listen-port 6443 --destination-port 6443 --protocol tcp
hcloud-destroy:
	hcloud server delete master-1 || true
	hcloud server delete master-2 || true
	hcloud server delete master-3 || true
	hcloud ssh-key delete k8s || true
	hcloud network delete k8s || true
	hcloud load-balancer delete k8s-master || true
	hcloud server list -lhcloud/node-group -o columns=name -o noheader | xargs -n1 hcloud server delete || true
create-master:
	# try to connect to servers and clean up directory
	ssh -i $(id_rsa) root@$(master1_host) "rm -rf /root/scripts/"
	ssh -i $(id_rsa) root@$(master2_host) "rm -rf /root/scripts/"
	ssh -i $(id_rsa) root@$(master3_host) "rm -rf /root/scripts/"

	scp -r -i $(id_rsa) ./scripts/ root@$(master1_host):/root/scripts/
	ssh -i $(id_rsa) root@$(master1_host) "MASTER_LB=$(master_lb) HCLOUD_TOKEN=$(hcloud_token) /root/scripts/init-master.sh"
	scp -i $(id_rsa) root@$(master1_host):/root/scripts/join-master.sh ./scripts/join-master.sh
	chmod +x ./scripts/join-master.sh
	make copy-key

	scp -r -i $(id_rsa) ./scripts/ root@$(master2_host):/root/scripts/
	ssh -i $(id_rsa) root@$(master2_host) "/root/scripts/common-install.sh; /root/scripts/join-master.sh"

	scp -r -i $(id_rsa) ./scripts/ root@$(master3_host):/root/scripts/
	ssh -i $(id_rsa) root@$(master3_host) "/root/scripts/common-install.sh; /root/scripts/join-master.sh"	
copy-key:
	rm -rf $(KUBECONFIG)
	scp -i $(id_rsa) root@$(master1_host):/etc/kubernetes/admin.conf $(KUBECONFIG)
destroy-master:
	ssh -i $(id_rsa) root@$(master1_host) /root/scripts/clear-cluster.sh || true
	ssh -i $(id_rsa) root@$(master2_host) /root/scripts/clear-cluster.sh || true
	ssh -i $(id_rsa) root@$(master3_host) /root/scripts/clear-cluster.sh || true
	hcloud server list -lhcloud/node-group -o columns=name -o noheader | xargs -n1 hcloud server delete
apply-yaml:
	kubectl apply -f ./scripts
apply-test:
	kubectl apply -f test/test-deployment.yaml
delete-test:
	kubectl delete -f test/test-deployment.yaml
test-kubernetes-yaml:
	kubectl apply --dry-run=client --validate=true -f ./scripts/deploy
	kubectl apply --dry-run=client --validate=true -f ./test
