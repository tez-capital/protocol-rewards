start:
	@go run main.go

docker_db:
	@docker run --rm -it --name pg_tezwatch1 -e POSTGRES_USER=tezwatch1 -e POSTGRES_PASSWORD=tezwatch1 -e POSTGRES_DB=tezwatch1 -p 127.0.0.1:5432:5432 docker.io/postgres:alpine

db:
	@podman run --rm -it --name pg_tezwatch -e POSTGRES_USER=tezwatch -e POSTGRES_PASSWORD=tezwatch -e POSTGRES_DB=tezwatch -p 127.0.0.1:5432:5432 docker.io/postgres:alpine