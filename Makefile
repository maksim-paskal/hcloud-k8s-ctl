KUBECONFIG=$(HOME)/.kube/hcloud
action=list-configurations

test:
	./scripts/validate-license.sh
	go mod tidy
	go test --race ./cmd
	go test --race ./pkg/config -args --config=config_test.yaml
	go test --race ./pkg/api
	golangci-lint run -v
create-cluster:
	make test
	make delete-cluster
	go run -race ./cmd -action=create -log.level=DEBUG
delete-cluster:
	go run -race ./cmd -action=delete -log.level=DEBUG
patch-cluster:
	make run action=patch-cluster
list-configurations:
	make run action=list-configurations
run:
	go run -race ./cmd -action=$(action) -log.level=DEBUG
build:
	goreleaser build --rm-dist --skip-validate
apply-yaml:
	kubectl apply -f ./scripts
apply-test:
	kubectl apply -f examples/test-deployment.yaml
delete-test:
	kubectl delete -f examples/test-deployment.yaml
test-kubernetes-yaml:
	helm template ./scripts/chart | kubectl apply --dry-run=client --validate=true -f -
	kubectl apply --dry-run=client --validate=true -f ./examples/test-deployment.yaml
download-yamls:
	curl -sSL -o ./scripts/chart/templates/kube-flannel.yml https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml
	curl -sSL -o ./scripts/chart/templates/ccm.yaml https://github.com/hetznercloud/hcloud-cloud-controller-manager/releases/latest/download/ccm.yaml
	curl -sSL -o ./scripts/chart/templates/hcloud-csi.yml https://raw.githubusercontent.com/hetznercloud/csi-driver/master/deploy/kubernetes/hcloud-csi.yml
	curl -sSL -o ./scripts/chart/templates/metrics-server.yml https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml