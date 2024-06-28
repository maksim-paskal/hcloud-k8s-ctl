KUBECONFIG=$(HOME)/.kube/hcloud
action=list-configurations
config=config.yaml
fullConfig=./e2e/configs/full.yaml
args=""
branch=`git rev-parse --abbrev-ref HEAD`

test:
	./scripts/validate-license.sh
	go fmt ./cmd/... ./pkg/...
	go mod tidy
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run -v
	CONFIG=config_test.yaml go test -race -coverprofile coverage.out ./cmd/... ./pkg/...
.PHONY: e2e
e2e:
	GIT_BRANCH=$(branch) go test -v -count=1 -timeout=4h ./e2e/e2e_test.go
update-readme:
	go run ./utils/main.go
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
	go run github.com/goreleaser/goreleaser@latest build --clean --skip=validate
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
install:
	go run github.com/goreleaser/goreleaser@latest build \
	--single-target \
	--clean \
	--snapshot \
	--skip=validate \
	--output /tmp/hcloud-k8s-ctl
	sudo mv /tmp/hcloud-k8s-ctl /usr/local/bin/hcloud-k8s-ctl

ubuntu-versions:
	docker run -v `pwd`:/app -it ubuntu:22.04 /app/scripts/ubuntu-versions.sh