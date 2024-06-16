start:
	@go run main.go

docker_db:
	@docker run --rm -it --name pg_ogun -e POSTGRES_USER=ogun -e POSTGRES_PASSWORD=ogun -e POSTGRES_DB=ogun -p 127.0.0.1:5432:5432 docker.io/postgres:alpine

db:
	@podman run --rm -it --name pg_ogun -e POSTGRES_USER=ogun -e POSTGRES_PASSWORD=ogun -e POSTGRES_DB=ogun -p 127.0.0.1:5432:5432 docker.io/postgres:alpine