# only works on linux/arm e.g. EC2 ARM instances
# use Docker to run on anything or crosscompile for your arch with below command
build:
	GOARCH=arm64 GOOS=linux go build -o ./bin/jinnybot.bin
run: build
	./bin/jinnybot.bin

build_renew:
	GOARCH=arm64 GOOS=linux go build -o ./bin/renew-user-token.bin ./renew-user-token/renew-user-token.go

build_docker:
	docker build . -t jinnybot
run_docker: build_docker
	docker run jinnybot
