.PHONY: all

all: ifmibpoller

ifmibpoller: *.go cmd/ifmibpoller.go
	go build cmd/ifmibpoller.go
