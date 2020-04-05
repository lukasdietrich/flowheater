.PHONY: all clean build

all: clean build

clean:
	rm -rf target/

build: target/flowheater

target/flowheater: *.go
	go build -o target/flowheater
