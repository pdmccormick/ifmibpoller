MAINS   := $(wildcard cmd/*/main.go)
CMDS    := $(patsubst cmd/%/main.go,%,$(MAINS))
BINS    = $(foreach cmd,$(CMDS),bin/$(cmd))

.PHONY: all build test clean

all: build

build: bin $(BINS)

bin:
	@mkdir -p bin/

bin/%: cmd/%/*.go
	@echo "    BUILD $@"
	@go build -o $@ ./cmd/$*

test:
	@echo "    TEST"
	@go test calcula/...

clean:
	@echo "    CLEAN"
	@rm -rf bin/

print-%: ; @echo $*=$($*)
