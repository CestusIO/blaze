# -----------------------------------------------------------------------------
# DEFINES
# -----------------------------------------------------------------------------

# Directory to compile binaries to
BINDIR                  ?= bin
# List of platforms to target [linux/windows/darwin]
PLATFORMS               ?= linux
# List of architectures to target [amd64/arm64]
ARCHITECTURES           := amd64
# Name of the app used for single application builds
APP 					:= protoc-gen-blaze
# List of applications to build (must reside in ./cmd/<name>)
APPLICATIONS            := protoc-gen-blaze
# Buildtime of a version will be passed as ldflag to go compiler
VERSION_DATE            ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
# Default version
svermakerBuildVersion   ?= localbuild
# GOPRIVATE will disable go cache
export GOPRIVATE        := code.cestus.io
# image name for docker image
IMAGE_NAME              ?= protoc-gen-blaze
# base registry (used for docker-build-local)
REGISTRY                ?= ""
# docker image name
IMAGE                   := $(REGISTRY)$(IMAGE_NAME)
# default docker version 
svermakerHelmLabel 	    ?= unreleased
goModuleBuildVersion    ?= unreleased
# additional LDFGLAGS (e.g. -w -s)
ADDITIONALLDFLAGS       ?= 
