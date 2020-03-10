DEFAULT: build

GO           ?= go
GOFMT        ?= $(GO)fmt
APP          := themis
DOCKER_ORG   := xmidt
FIRST_GOPATH := $(firstword $(subst :, ,$(shell $(GO) env GOPATH)))
BINARY    	 := $(FIRST_GOPATH)/bin/$(APP)

VERSION ?= $(shell git describe --tag --always --dirty)
PROGVER = $(shell git describe --tags `git rev-list --tags --max-count=1` | tail -1 | sed 's/v\(.*\)/\1/')
RPM_VERSION=$(shell echo $(PROGVER) | sed 's/\(.*\)-\(.*\)/\1/')
RPM_RELEASE=$(shell echo $(PROGVER) | sed -n 's/.*-\(.*\)/\1/p'  | grep . && (echo "$(echo $(PROGVER) | sed 's/.*-\(.*\)/\1/')") || echo "1")
BUILDTIME = $(shell date -u '+%Y-%m-%d %H:%M:%S')
GITCOMMIT = $(shell git rev-parse --short HEAD)
GOBUILDFLAGS = -a -ldflags "-w -s -X 'main.BuildTime=$(BUILDTIME)' -X main.GitCommit=$(GITCOMMIT) -X main.Version=$(VERSION)" -o $(APP)

.PHONY: vendor
vendor:
	$(GO) mod vendor

.PHONY: build
build: vendor
	CGO_ENABLED=0 $(GO) build $(GOBUILDFLAGS)

rpm:
	mkdir -p ./.ignore/SOURCES

	# CPE service 
	tar -czf ./.ignore/SOURCES/cpe_themis-$(RPM_VERSION)-$(RPM_RELEASE).tar.gz --transform 's/^\./cpe_themis-$(RPM_VERSION)-$(RPM_RELEASE)/' --exclude ./.git --exclude ./.ignore --exclude ./conf --exclude ./deploy --exclude ./vendor --exclude ./vendor .
	cp conf/cpe_themis.service ./.ignore/SOURCES
	cp themis.yaml  ./.ignore/SOURCES/cpe_themis.yaml

	# RBL service
	tar -czf ./.ignore/SOURCES/rbl_themis-$(RPM_VERSION)-$(RPM_RELEASE).tar.gz --transform 's/^\./rbl_themis-$(RPM_VERSION)-$(RPM_RELEASE)/' --exclude ./.git --exclude ./.ignore --exclude ./conf --exclude ./deploy --exclude ./vendor --exclude ./vendor .
	cp conf/rbl_themis.service ./.ignore/SOURCES
	cp themis.yaml  ./.ignore/SOURCES/rbl_themis.yaml

	# Standalone-mode service - All other XMiDT services are setup this way
	tar -czf ./.ignore/SOURCES/$(APP)-$(RPM_VERSION)-$(RPM_RELEASE).tar.gz --transform 's/^\./$(APP)-$(RPM_VERSION)-$(RPM_RELEASE)/' --exclude ./.git --exclude ./.ignore --exclude ./conf --exclude ./deploy --exclude ./vendor --exclude ./vendor .
	cp conf/themis.service ./.ignore/SOURCES
	cp themis.yaml  ./.ignore/SOURCES

	cp LICENSE ./.ignore/SOURCES
	cp NOTICE ./.ignore/SOURCES
	cp CHANGELOG.md ./.ignore/SOURCES

	# CPE service
	rpmbuild --define "_topdir $(CURDIR)/.ignore" \
			--define "_version $(RPM_VERSION)" \
			--define "_release $(RPM_RELEASE)" \
			-ba deploy/packaging/cpe_themis.spec

	# RBL service
	rpmbuild --define "_topdir $(CURDIR)/.ignore" \
			--define "_version $(RPM_VERSION)" \
			--define "_release $(RPM_RELEASE)" \
			-ba deploy/packaging/rbl_themis.spec

	# Standalone-mode service - All other XMiDT services are setup this way
	rpmbuild --define "_topdir $(CURDIR)/.ignore" \
			--define "_version $(RPM_VERSION)" \
			--define "_release $(RPM_RELEASE)" \
			-ba deploy/packaging/$(APP).spec

.PHONY: version
version:
	@echo $(PROGVER)

# If the first argument is "update-version"...
ifeq (update-version,$(firstword $(MAKECMDGOALS)))
  # use the rest as arguments for "update-version"
  RUN_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  # ...and turn them into do-nothing targets
  $(eval $(RUN_ARGS):;@:)
endif

.PHONY: update-version
update-version:
	@echo "Update Version $(PROGVER) to $(RUN_ARGS)"
	git tag v$(RUN_ARGS)


.PHONY: install
install: vendor
	$(GO) install -ldflags "-w -s -X 'main.BuildTime=$(BUILDTIME)' -X main.GitCommit=$(GITCOMMIT) -X main.Version=$(PROGVER)"

.PHONY: release-artifacts
release-artifacts: vendor
	mkdir -p ./.ignore
	GOOS=darwin GOARCH=amd64 $(GO) build -o ./.ignore/$(APP)-$(PROGVER).darwin-amd64 -ldflags "-X 'main.BuildTime=$(BUILDTIME)' -X main.GitCommit=$(GITCOMMIT) -X main.Version=$(PROGVER)"
	GOOS=linux  GOARCH=amd64 $(GO) build -o ./.ignore/$(APP)-$(PROGVER).linux-amd64 -ldflags "-X 'main.BuildTime=$(BUILDTIME)' -X main.GitCommit=$(GITCOMMIT) -X main.Version=$(PROGVER)"

.PHONY: docker
docker:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GITCOMMIT=$(GITCOMMIT) \
		--build-arg BUILDTIME='$(BUILDTIME)' \
		-f ./deploy/Dockerfile -t $(DOCKER_ORG)/$(APP):$(PROGVER) .

.PHONY: local-docker
local-docker:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg GITCOMMIT=$(GITCOMMIT) \
		--build-arg BUILDTIME='$(BUILDTIME)' \
		-f ./deploy/Dockerfile -t $(DOCKER_ORG)/$(APP):local .

.PHONY: style
style:
	! $(GOFMT) -d $$(find . -path ./vendor -prune -o -name '*.go' -print) | grep '^'

.PHONY: test
test: vendor
	GO111MODULE=on $(GO) test -v -race  -coverprofile=cover.out ./...

.PHONY: test-cover
test-cover: test
	$(GO) tool cover -html=cover.out

.PHONY: codecov
codecov: test
	curl -s https://codecov.io/bash | bash

.PHONEY: it
it:
	./it.sh

.PHONY: clean
clean:
	rm -rf ./$(APP) ./.ignore ./coverage.txt ./vendor
