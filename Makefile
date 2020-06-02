MAINS   := $(wildcard cmd/*/main.go)
CMDS    := $(patsubst cmd/%/main.go,%,$(MAINS))
BINS    = $(foreach cmd,$(CMDS),bin/$(cmd))

.PHONY: all build test clean

all: build

build: bin $(BINS)

bin:
	@mkdir -p bin/

bin/%: cmd/%/*.go *.go
	@echo "    BUILD    $@"
	@go build -o $@ ./$(shell dirname $<)

test:
	@echo "    TEST"
	@go test ./...

clean:
	@echo "    CLEAN"
	@rm -rf bin/

print-%: ; @echo $*=$($*)
