GO ?= $(CURDIR)/.tools/go/bin/go
export PATH := $(CURDIR)/.tools/go/bin:$(PATH)
export GOPATH := $(CURDIR)/.tools/gopath
export GOCACHE := $(CURDIR)/.tools/gocache
export GOEXPERIMENT := nojsonv2
export XDG_DATA_HOME := $(CURDIR)/.local/xdg-data
export XDG_CONFIG_HOME := $(CURDIR)/.local/xdg-config
STATE ?= .local
BROWSER_ARGS ?=
.PHONY: bootstrap build demo test browser-test browser-check integration-test fixture clean clean-all
bootstrap:
	./scripts/bootstrap.sh
build: bootstrap
	$(GO) build -o bin/qwacamole ./cmd/qwacamole
	.tools/bin/xcaddy build v2.10.2 --with github.com/cztamas/qwacamole=$(CURDIR) --output bin/caddy > .local/evidence/xcaddy-build.log 2>&1
demo: build
	bin/qwacamole demo -state $(STATE)
test:
	$(GO) test -race ./...
# Full browser acceptance remains nonzero while stock Chrome ignores exp.
browser-test: build
	bin/qwacamole integration-test -state $(STATE) $(BROWSER_ARGS)
integration-test: browser-test
browser-check:
	bin/qwacamole browser -state $(STATE) -ct-exception -tls-root $(STATE)/pki/pebble-root.pem $(BROWSER_ARGS)
fixture:
	bin/qwacamole fixture -state $(STATE)
# Stop this project's running demo with Ctrl-C before deleting state.
clean:
	bin/qwacamole clean -state $(STATE)
clean-all: clean
	rm -rf bin .tools
