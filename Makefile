# only works on linux/arm e.g. EC2 ARM instances
# use Docker to run on anything or crosscompile for your arch with below command
build:
	GOARCH=arm64 GOOS=linux go build -o ./bin/harubot.bin
run: build
	./bin/harubot.bin

build_docker:
	docker build . -t harubot
run_docker: build_docker
	docker run harubot
