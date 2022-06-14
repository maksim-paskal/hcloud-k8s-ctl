KUBECONFIG=$(HOME)/.kube/hcloud
action=list-configurations
config=config.yaml

test:
	./scripts/validate-license.sh
	go fmt ./cmd/... ./pkg/...
	go mod tidy
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run -v
	CONFIG=config_test.yaml go test -race -coverprofile coverage.out ./cmd/... ./pkg/...
coverage:
	go tool cover -html=coverage.out
create-cluster:
	make test
	make delete-cluster
	go run -race ./cmd -config=$(config) -action=create -log.level=DEBUG
delete-cluster:
	go run -race ./cmd -config=$(config) -action=delete -log.level=DEBUG
patch-cluster:
	make run action=patch-cluster
list-configurations:
	make run action=list-configurations
upgrade-controlplane:
	make run action=upgrade-controlplane
run:
	go run -race ./cmd -config=$(config) -action=$(action) -log.level=DEBUG
build:
	go run github.com/goreleaser/goreleaser@latest build --rm-dist --skip-validate
apply-yaml:
	kubectl apply -f ./scripts
apply-test:
	kubectl apply -f examples/test-autoscaler.yaml
	kubectl apply -f examples/test-deployment.yaml
delete-test:
	kubectl apply -f examples/test-autoscaler.yaml
	kubectl delete -f examples/test-deployment.yaml
test-kubernetes-yaml:
	helm template ./scripts/chart | kubectl apply --dry-run=client --validate=true -f -
	kubectl apply --dry-run=client --validate=true -f ./examples/test-deployment.yaml
download-yamls:
	curl -sSL -o ./scripts/chart/templates/kube-flannel.yml https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml
	curl -sSL -o ./scripts/chart/templates/hcloud-csi.yml https://raw.githubusercontent.com/hetznercloud/csi-driver/v1.6.0/deploy/kubernetes/hcloud-csi.yml
	curl -sSL -o ./scripts/chart/templates/metrics-server.yml https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
install:
	go run github.com/goreleaser/goreleaser@latest build \
	--single-target \
	--rm-dist \
	--skip-validate \
	--output /tmp/hcloud-k8s-ctl
	sudo mv /tmp/hcloud-k8s-ctl /usr/local/bin/hcloud-k8s-ctl