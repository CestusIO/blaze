#region CODE_REGION(Make)
# Make is verbose in Linux. Make it silent.
MAKEFLAGS += --silent
include build.mk 
# -----------------------------------------------------------------------------
# DEFINES
# -----------------------------------------------------------------------------
TOOLS_MOD_DIR := ./tools
# All directories with go.mod files  Used for building, testing and linting.
ALL_GO_MOD_DIRS := $(filter-out $(TOOLS_MOD_DIR), $(shell find . -type f -name 'go.mod' -exec dirname {} \; | sort)) 

SHELL                   := sh
SCRIPT_DIR              := hack
IS_CI                   ?= 0
BINARY_windows_ENDING   := .exe

# Colors for shell
NO_COLOR=\033[0m
CYAN_COLOR=\033[0;36m

# -----------------------------------------------------------------------------
# CHECKS
# -----------------------------------------------------------------------------
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

LDFLAGS += $(ADDITIONALLDFLAGS)
LDFLAGS += -X code.cestus.io/libs/buildinfo.version=${goModuleBuildVersion}
LDFLAGS += -X code.cestus.io/libs/buildinfo.buildDate=$(VERSION_DATE)
export LDFLAGS

# building platform string
b_platform = --> Building $(APP)-$(GOOS)-$(GOARCH)\n
# building platform command
b_command = export GOOS=$(GOOS); export GOARCH=$(GOARCH); go build -ldflags "$(LDFLAGS) -X code.cestus.io/libs/buildinfo.name=$(APP)" -o $(BINDIR)/$(APP)-$(GOOS)-$(GOARCH)$(BINARY_$(GOOS)_ENDING) ./cmd/$(APP)/ ;
# for each iterations use build message
fb_platforms =$(foreach GOOS, $(PLATFORMS),$(foreach GOARCH, $(ARCHITECTURES),$(foreach APP, $(APPLICATIONS),$(b_platform))))
# foreach iterations to do multi platform build
fb_command = $(foreach GOOS, $(PLATFORMS),\
	$(foreach GOARCH, $(ARCHITECTURES),$(foreach APP, $(APPLICATIONS),$(b_command))))

.PHONY: all
## Builds for all platforms and architectures (including generation and setup and tests)
all: generate build_all test
	
# No documentation; Installs tools
setup:
	
.PHONY: clean
## Cleans generation folders
clean:

.PHONY: generate
## Generate files
generate:
	set -e; for dir in $(ALL_GO_MOD_DIRS); do \
		echo "go generate $${dir}/..."; \
		(cd "$${dir}" && \
			go generate ./...); \
	done

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

.PHONY: install_tools
install_tools:
	 if [ -d "tools" ]; then $(MAKE) -C tools all ;fi

.PHONY: build_all
## Builds for all platforms and architectures (including generation and setup)
build_all: install_tools build_all_ci

.PHONY: build_all_ci
## Builds for all platforms and architectures (including generation and setup)
build_all_ci: setup compile_all compile_absolute_everything changelog

.PHONY: compile_absolute_everything
## Builds all files even those not part of the desired outputs
compile_absolute_everything:
	$(call msg, --> Building all files even those not part of the desired apps)
	set -e; for dir in $(ALL_GO_MOD_DIRS); do \
		echo "go build $${dir}/..."; \
		(cd "$${dir}" && \
			go build ./...);\
	done

.PHONY: test
## Runs the tests
test: install_tools
	for dir in $(ALL_GO_MOD_DIRS); do \
	echo "ginkgo $${dir}/..."; \
	(cd "$${dir}" && \
		export CGO_ENABLED=1; ginkgo -race -cover -coverprofile=cover.coverprofile --output-dir=. ./...); \
	done; 
.PHONY: changelog
changelog:
	$(call msg, --> Generating changelog)
	git-chglog --next-tag $(goModuleBuildVersion) 1> /dev/null && ([ $$? -eq 0 ] && git-chglog --next-tag $(goModuleBuildVersion) -o CHANGELOG.md) || echo "changelog failure!"

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
# endregion