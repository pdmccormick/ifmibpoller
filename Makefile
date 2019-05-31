.PHONY: all clean

all: bin/ifmibpoller

bin/ifmibpoller: *.go cmd/ifmibpoller.go
	go build -o $@ cmd/ifmibpoller.go

clean:
	rm -rf bin/
