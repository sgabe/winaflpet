.PHONY: all server agent

BUILD_VER := 0.0.6
BUILD_REV := $(shell git rev-parse --short HEAD)
BUILD_DATE ?= $(shell git log --pretty=format:%ct -1)

BUILD_VER_VAR := main.BuildVer
BUILD_REV_VAR := main.BuildRev
LDFLAGS :=	-X \"$(BUILD_VER_VAR)=$(BUILD_VER)\" \
	-X \"$(BUILD_REV_VAR)=$(BUILD_REV)\" \

server:
ifeq ($(OS),Windows_NT)
	go build -ldflags "$(LDFLAGS)" -o ./winaflpet-server.exe ./server
else
	docker build \
		--build-arg BUILD_VER=$(BUILD_VER) \
		--build-arg BUILD_REV=$(BUILD_REV) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--no-cache -t sgabe/winaflpet:$(BUILD_VER) .
endif

agent:
	go build -ldflags "$(LDFLAGS)" -o ./winaflpet-agent.exe ./agent

clean:
	go clean && del winaflpet-server.exe && del winaflpet-agent.exe

all:
	server
	agent
