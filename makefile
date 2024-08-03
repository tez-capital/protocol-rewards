start:
	@go run main.go

docker_db:
	@docker run --rm -it --name pg_protocol_rewards -e POSTGRES_USER=protocol_rewards -e POSTGRES_PASSWORD=protocol_rewards -e POSTGRES_DB=protocol_rewards -p 127.0.0.1:5432:5432 docker.io/postgres:alpine

db:
	@podman run --rm -it --name pg_protocol_rewards -e POSTGRES_USER=protocol_rewards -e POSTGRES_PASSWORD=protocol_rewards -e POSTGRES_DB=protocol_rewards -p 127.0.0.1:5432:5432 docker.io/postgres:alpine