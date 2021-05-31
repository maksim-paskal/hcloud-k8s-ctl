KUBECONFIG=$(HOME)/.kube/hcloud

create-cluster:
	./scripts/validate-license.sh
	go mod tidy
	golangci-lint run -v
	make delete-cluster
	go run -race ./cmd -action=create -log.level=DEBUG
delete-cluster:
	go run -race ./cmd -action=delete -log.level=DEBUG
build:
	@./scripts/build-all.sh
	go mod tidy
apply-yaml:
	kubectl apply -f ./scripts
apply-test:
	kubectl apply -f examples/test-deployment.yaml
delete-test:
	kubectl delete -f examples/test-deployment.yaml
test-kubernetes-yaml:
	kubectl apply --dry-run=client --validate=true -f ./scripts/deploy
	kubectl apply --dry-run=client --validate=true -f ./examples/test-deployment.yaml
download-yamls:
	curl -sSL -o scripts/deploy/kube-flannel.yml https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml
	curl -sSL -o scripts/deploy/ccm.yaml https://github.com/hetznercloud/hcloud-cloud-controller-manager/releases/latest/download/ccm.yaml
	curl -sSL -o scripts/deploy/hcloud-csi.yml https://raw.githubusercontent.com/hetznercloud/csi-driver/master/deploy/kubernetes/hcloud-csi.yml
	curl -sSL -o scripts/deploy/metrics-server.yml https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
patch-cluster:
	KUBECONFIG_PATH=$(KUBECONFIG) SCRTIPT_PATH=. ./scripts/post-install.sh