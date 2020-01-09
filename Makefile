# Make is verbose in Linux. Make it silent.
MAKEFLAGS += --silent
include build.mk 
# -----------------------------------------------------------------------------
# DEFINES
# -----------------------------------------------------------------------------

SHELL                   := sh
SCRIPT_DIR              := hack
IS_CI                   ?= 0
GOBIN 					= "$(PWD)/$(BINDIR)"
BINARY_windows_ENDING   := .exe

# set dockerflags (20191201 -  -w is only implemented for build. push will implement it in a version soon - its merged)
ifeq ($(IS_CI), 1)
	DOCKER_FLAGS += -q
endif
export DOCKER_FLAGS

# Colors for shell
NO_COLOR=\033[0m
CYAN_COLOR=\033[0;36m

# -----------------------------------------------------------------------------
# CHECKS
# -----------------------------------------------------------------------------
HAS_DOCKER              := $(shell command -v docker;)
HAS_GO                  := $(shell command -v go;)

# -----------------------------------------------------------------------------
# MACROS
# -----------------------------------------------------------------------------

msg = @printf '$(CYAN_COLOR)$(1)$(NO_COLOR)\n'

# -----------------------------------------------------------------------------
# TARGETS - GOLANG
# -----------------------------------------------------------------------------
# set default os and architecture to have common naming for build and build_all targets
ifneq ($(HAS_GO),)
	ifndef $(GOOS)
		GOOS=$(shell go env GOOS)
		export GOOS
	endif
	ifndef $(GOARCH)
		GOARCH=$(shell go env GOARCH)
		export GOARCH
	endif
endif

LDFLAGS += -X main.Version=${svermakerBuildVersion}
LDFLAGS += -X main.BuildTime=$(VERSION_DATE)
export LDFLAGS

# building platform string
b_platform = --> Building $(APP)-$(GOOS)-$(GOARCH)\n
# building platform command
b_command = export GOOS=$(GOOS); export GOARCH=$(GOARCH); go build -ldflags "$(LDFLAGS)" -o $(BINDIR)/$(APP)-$(GOOS)-$(GOARCH)$(BINARY_$(GOOS)_ENDING) ./cmd/$(APP)/ ;
# for each iterations use build message
fb_platforms =$(foreach GOOS, $(PLATFORMS),$(foreach GOARCH, $(ARCHITECTURES),$(foreach APP, $(APPLICATIONS),$(b_platform))))
# foreach iterations to do multi platform build
fb_command = $(foreach GOOS, $(PLATFORMS),\
	$(foreach GOARCH, $(ARCHITECTURES),$(foreach APP, $(APPLICATIONS),$(b_command))))

all: compile
# No documentation; Installs tools
.PHONY: setup
setup:
	$(call msg, Installing tools)
	go install -v github.com/onsi/ginkgo/ginkgo
	go install -v github.com/golang/protobuf/protoc-gen-go
	
	
.PHONY: generate
## Generate files
generate: 
	$(call msg, Generating files)
	go generate ./...

.PHONY: compile
## Compile for current platform and architecture
compile: 
	$(call msg, $(b_platform))
	@$(b_command)

.PHONY: compile_all
## Compile for all platforms, architectures and apps
## Thows error if a invalid GOOS/GOARCH combo gets generated
compile_all: 
	$(call msg, $(fb_platforms))
	@$(fb_command)

.PHONY: build
## Build (including generation and setup) for the current platform
build: setup generate compile  

.PHONY: build_all
## Builds for all platforms and architectures (including generation and setup)
build_all: setup generate compile_all 

.PHONY: test
## Test (including generation and setup)
test: setup generate run_test

.PHONY: run_test
run_test:
	ginkgo ./...
# -----------------------------------------------------------------------------
# TARGETS - DOCKER
# -----------------------------------------------------------------------------

.PHONY: docker-build-local
## Build local docker image
docker-build-local: check-docker 
	$(call msg,-->Building Docker image $(IMAGE_NAME):local )
	docker build $(DOCKER_FLAGS) -t $(IMAGE_NAME):local \
    --build-arg http_proxy=$(HTTP_PROXY) \
    --build-arg https_proxy=$(HTTP_PROXY) .

.PHONY: check-docker
check-docker:
ifndef HAS_DOCKER
	$(error You must install Docker)
endif

.PHONY: docker-push
## Push a docker image
docker-push: docker-build-local 
	$(call msg,--> Pushing Docker image $(IMAGE):$(svermakerHelmLabel) )
	docker tag $(IMAGE_NAME):local $(IMAGE):$(svermakerHelmLabel)
	docker push $(IMAGE):$(svermakerHelmLabel)

# Plonk the following at the end of your Makefile
.DEFAULT_GOAL := show-help

# Inspired by <http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html>
# sed script explained:
# /^##/:
# 	* save line in hold space
# 	* purge line
# 	* Loop:
# 		* append newline + line to hold space
# 		* go to next line
# 		* if line starts with doc comment, strip comment character off and loop
# 	* remove target prerequisites
# 	* append hold space (+ newline) to line
# 	* replace newline plus comments by `---`
# 	* print line
# Separate expressions are necessary because labels cannot be delimited by
# semicolon; see <http://stackoverflow.com/a/11799865/1968>
.PHONY: show-help
show-help:
	@echo "$$(tput bold)Available rules:$$(tput sgr0)"
	@echo
	@sed -n -e "/^## / { \
		h; \
		s/.*//; \
		:doc" \
		-e "H; \
		n; \
		s/^## //; \
		t doc" \
		-e "s/:.*//; \
		G; \
		s/\\n## /---/; \
		s/\\n/ /g; \
		p; \
	}" ${MAKEFILE_LIST} \
	| LC_ALL='C' sort --ignore-case \
	| awk -F '---' \
		-v ncol=$$(tput cols) \
		-v indent=19 \
		-v col_on="$$(tput setaf 6)" \
		-v col_off="$$(tput sgr0)" \
	'{ \
		printf "%s%*s%s ", col_on, -indent, $$1, col_off; \
		n = split($$2, words, " "); \
		line_length = ncol - indent; \
		for (i = 1; i <= n; i++) { \
			line_length -= length(words[i]) + 1; \
			if (line_length <= 0) { \
				line_length = ncol - indent - length(words[i]) - 1; \
				printf "\n%*s ", -indent, " "; \
			} \
			printf "%s ", words[i]; \
		} \
		printf "\n"; \
	}'