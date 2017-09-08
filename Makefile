all: deps compile


compile:
	go build

deps:
	go get

fmt:
	gofmt -s -w -l .

clean:
	rm -fr obj docker-volume-sheepdog

.PHONY: deps compile
