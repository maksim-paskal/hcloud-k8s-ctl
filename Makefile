KUBECONFIG=$(HOME)/.kube/hcloud
action=list-configurations
config=config.yaml
fullConfig=./examples/config-full.yaml
args=""

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
create-firewall:
	make run action=create-firewall
save-full-config:
	make run action=save-full-config args=-save-config-path=$(fullConfig)
upgrade-workers: # restart all pods on worker node
	make run action=adhoc args="-adhoc.copynewfile -adhoc.command=/root/scripts/upgrade-worker.sh"
upgrade-workers-kernel: # upgrade kernel and restart worker node
	make run action=adhoc args="-adhoc.copynewfile -adhoc.command=/root/scripts/upgrade-kernel.sh"
run:
	go run -race ./cmd -config=$(config) -action=$(action) -log.level=DEBUG $(args)
build:
	go run github.com/goreleaser/goreleaser@latest build --rm-dist --skip-validate
apply-yaml:
	kubectl apply -f ./scripts
apply-test:
	helm upgrade --install test ./examples/charts/test
delete-tests:
	helm delete test || true
test-kubernetes-yaml:
	make save-full-config
	helm dep up ./scripts/chart
	helm lint ./scripts/chart --values=$(fullConfig)
	helm lint ./examples/charts/test --values=$(fullConfig)
	helm template ./scripts/chart --values=$(fullConfig) | kubectl apply --dry-run=client --validate=true -f -
	helm template ./examples/charts/test --values=$(fullConfig) | kubectl apply --dry-run=client --validate=true -f -
download-yamls:
	curl -sSL -o ./scripts/chart/templates/kube-flannel.yml https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml
	curl -sSL -o ./scripts/chart/templates/metrics-server.yml https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
install:
	go run github.com/goreleaser/goreleaser@latest build \
	--single-target \
	--rm-dist \
	--skip-validate \
	--output /tmp/hcloud-k8s-ctl
	sudo mv /tmp/hcloud-k8s-ctl /usr/local/bin/hcloud-k8s-ctl