.PHONY: build-python-3.13 build-python-3.13-example build

build-python-3.13:
	docker build --platform=linux/amd64 -t generate-ext4 .
	./generate-rootfs.sh lambda-at-home-runtime-python:3.13 ./runtimes/python/3.13/Dockerfile

build-python-3.13-example:
	docker build --platform=linux/amd64 -t generate-ext4 .
	docker run --platform=linux/amd64 --rm --privileged -v "./runtimes/python/3.13/examples:/workspace/dist" -v "./artifacts:/workspace/artifacts" generate-ext4 python-3.13-example 50

build:
	GOOS=linux GOARCH=amd64 go build