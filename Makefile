run:
	docker-compose -f docker-compose.yml up --remove-orphans --force-recreate

lint:
	golangci-lint run