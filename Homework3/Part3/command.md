# run go server

go run main.go

# use HttpUser (port 8089)

docker-compose up master-http worker-http --scale worker-http=4

# use FastHttpUser (port 8090)

docker-compose up master-fast worker-fast --scale worker-fast=4
