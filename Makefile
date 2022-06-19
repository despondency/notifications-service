run-load:
	docker-compose -f docker-compose-load.yml up --remove-orphans --force-recreate

run-local:
	docker-compose -f docker-compose.yml down --remove-orphans && docker-compose build && docker-compose -f docker-compose.yml up --force-recreate

integration-tests:
	ginkgo -r --randomize-suites --keep-going ./test/integration/

unit-tests:
	ginkgo -r --randomize-suites --keep-going --skip-package integration

lint:
	golangci-lint run

generate:
	go generate ./...