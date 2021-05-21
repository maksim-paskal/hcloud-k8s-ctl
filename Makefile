KUBECONFIG=$(HOME)/.kube/hcloud

run:
	./scripts/validate-license.sh
	go mod tidy
	golangci-lint run -v
	go run -race ./cmd -action=create -deleteBeforeCreation -log.level=DEBUG
delete-cluster:
	go run -race ./cmd -action=delete -log.level=DEBUG
build:
	@./scripts/build-all.sh
apply-yaml:
	kubectl apply -f ./scripts
apply-test:
	kubectl apply -f examples/test-deployment.yaml
delete-test:
	kubectl delete -f examples/test-deployment.yaml
test-kubernetes-yaml:
	kubectl apply --dry-run=client --validate=true -f ./scripts/deploy
	kubectl apply --dry-run=client --validate=true -f ./test