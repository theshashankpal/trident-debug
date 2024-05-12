# Copyright 2023 NetApp, Inc. All Rights Reserved.

# Parameters
# PLATFORMS defines which platforms are built for each target. If platforms is set to all, builds all supported platforms.
PLATFORMS ?= linux/amd64
ifeq ($(PLATFORMS),all)
PLATFORMS = $(ALL_PLATFORMS)
endif

# REGISTRY defines the container registry to push or tag images
REGISTRY ?= $(DEFAULT_REGISTRY)

# BUILDX_OUTPUT set to `load` or `push` with docker buildx to load or push images, default load
BUILDX_OUTPUT ?= load

# GO_IMAGE golang image used in default GO_SHELL
GO_IMAGE ?= golang:1.22

# GO_CMD go command used for go build
GO_CMD ?= go

# GOPROXY override default Go proxy
GOPROXY ?=

# GOFLAGS custom flags used in Go commands. See https://pkg.go.dev/cmd/go#hdr-Environment_variables
GOFLAGS ?=

# HELM_IMAGE helm image used in default HELM_CMD
HELM_IMAGE ?= alpine/helm:3.6.1

# DOCKER_CLI the docker-compatible cli used to run and tag images
DOCKER_CLI ?= docker

# BUILD_CLI the docker-compatible cli used to build images. If set to "docker buildx", the image build script will
# ensure an image builder instance exists. Windows builds and the manifest targets require BUILD_CLI set to "docker buildx"
BUILD_CLI ?= docker

# GO_SHELL sets the Go environment. By default uses DOCKER_CLI to create a container using GO_IMAGE. Set to empty string
# to use local shell
GO_SHELL ?= $(DOCKER_CLI) run \
	-e XDG_CACHE_HOME=/go/cache \
	-v $(TRIDENT_VOLUME):/go \
	-v $(ROOT):$(BUILD_ROOT) \
	-w $(BUILD_ROOT) \
	--rm \
	$(GO_IMAGE) \
	sh -c

# HELM_CMD sets the helm command. By default uses DOCKER_CLI to create a container using HELM_IMAGE. Set to 'helm' to
# use local helm command
HELM_CMD ?= $(DOCKER_CLI) run \
	-v $(ROOT):$(BUILD_ROOT) \
	-w $(BUILD_ROOT) \
	--rm \
	$(HELM_IMAGE)

# GITHASH git commit hash used in binaries
GITHASH ?= $(shell git describe --match=NeVeRmAtCh --always --abbrev=40 --dirty || echo unknown)

# BUILD_TYPE custom/stable/alpha/beta/empty string, default is custom for dev builds
BUILD_TYPE ?= custom

# BUILD_TYPE_REV build type revision, used by CI
BUILD_TYPE_REV ?= 0

# TRIDENT_IMAGE trident image name
TRIDENT_IMAGE ?= trident

# TRIDENT_IMAGE operator image name
OPERATOR_IMAGE ?= trident-operator

# MANIFEST_TAG tag for trident manifest
MANIFEST_TAG ?= $(TRIDENT_TAG)

# OPERATOR_MANIFEST_TAG tag for operator manifest
OPERATOR_MANIFEST_TAG ?= $(OPERATOR_TAG)

# BUILDX_CONFIG_FILE path to buildkitd config file for docker buildx. Set this to use an insecure registry with
# cross-platform builds, see example config: https://github.com/moby/buildkit/blob/master/docs/buildkitd.toml.md
BUILDX_CONFIG_FILE ?=

# DEFAULT_AUTOSUPPORT_IMAGE override the default asup image in tridentctl and operator
DEFAULT_AUTOSUPPORT_IMAGE ?=

# DEFAULT_ACP_IMAGE override the default acp image in tridentctl and operator
DEFAULT_ACP_IMAGE ?=

ARTIFACTORY_NAMESPACE ?=

ARTIFACTORY_FOLDER ?=

# Constants
ALL_PLATFORMS = linux/amd64 linux/arm64 windows/amd64/ltsc2022 windows/amd64/1809 darwin/amd64
DEFAULT_REGISTRY = docker.io/netapp
NETAPP_REGISTRY = docker.repo.eng.netapp.com
TRIDENT_CONFIG_PKG = github.com/netapp/trident/config
OPERATOR_CONFIG_PKG = github.com/netapp/trident/operator/config
TRIDENT_KUBERNETES_PKG = github.com/netapp/trident/persistent_store/crd
OPERATOR_CONFIG_PKG = github.com/netapp/trident/operator/config
OPERATOR_INSTALLER_CONFIG_PKG = github.com/netapp/trident/operator/controllers/orchestrator/installer
OPERATOR_KUBERNETES_PKG = github.com/netapp/trident/operator/crd
VERSION_FILE = github.com/netapp/trident/hack/VERSION
BUILD_ROOT = /go/src/github.com/netapp/trident
TRIDENT_VOLUME = trident-build
DOCKER_BUILDX_INSTANCE_NAME = trident-builder
DOCKER_BUILDX_BUILD_CLI = docker buildx
WINDOWS_IMAGE_REPO = mcr.microsoft.com/windows/nanoserver
BUILDX_MANIFEST_DIR = /tmp/trident_buildx_manifests
K8S_CODE_GENERATOR = code-generator-kubernetes-1.18.2
DELVE_PATH = github.com/go-delve/delve/cmd/dlv@latest
TRIDENT_DEBUG = trident-debug


# Calculated values
BUILD_TIME = $(shell date)
ROOT = $(shell pwd)

ifeq ($(ARTIFACTORY_FOLDER),)
ARTIFACTORY := $(ARTIFACTORY_NAMESPACE)
else
ARTIFACTORY := $(ARTIFACTORY_NAMESPACE)/$(ARTIFACTORY_FOLDER)
endif

# tag variables
TRIDENT_DEBUG_TAG := $(NETAPP_REGISTRY)/$(ARTIFACTORY)/$(TRIDENT_DEBUG):latest

# linker flags need to be properly encapsulated with double quotes to handle spaces in values
LINKER_FLAGS = "-s -w -X \"$(TRIDENT_CONFIG_PKG).BuildHash=$(GITHASH)\" -X \"$(TRIDENT_CONFIG_PKG).BuildType=$(BUILD_TYPE)\" -X \"$(TRIDENT_CONFIG_PKG).BuildTypeRev=$(BUILD_TYPE_REV)\" -X \"$(TRIDENT_CONFIG_PKG).BuildTime=$(BUILD_TIME)\" -X \"$(TRIDENT_CONFIG_PKG).BuildImage=$(TRIDENT_TAG)\" -X \"$(OPERATOR_CONFIG_PKG).BuildImage=$(OPERATOR_TAG)\"$(if $(DEFAULT_AUTOSUPPORT_IMAGE), -X \"$(TRIDENT_CONFIG_PKG).DefaultAutosupportImage=$(DEFAULT_AUTOSUPPORT_IMAGE)\")$(if $(DEFAULT_ACP_IMAGE), -X \"$(TRIDENT_CONFIG_PKG).DefaultACPImage=$(DEFAULT_ACP_IMAGE)\")"
LINKER_FLAGS_DEBUG = "-X \"$(TRIDENT_CONFIG_PKG).BuildHash=$(GITHASH)\" -X \"$(TRIDENT_CONFIG_PKG).BuildType=$(BUILD_TYPE)\" -X \"$(TRIDENT_CONFIG_PKG).BuildTypeRev=$(BUILD_TYPE_REV)\" -X \"$(TRIDENT_CONFIG_PKG).BuildTime=$(BUILD_TIME)\" -X \"$(TRIDENT_CONFIG_PKG).BuildImage=$(TRIDENT_TAG)\" -X \"$(OPERATOR_CONFIG_PKG).BuildImage=$(OPERATOR_TAG)\"$(if $(DEFAULT_AUTOSUPPORT_IMAGE), -X \"$(TRIDENT_CONFIG_PKG).DefaultAutosupportImage=$(DEFAULT_AUTOSUPPORT_IMAGE)\")$(if $(DEFAULT_ACP_IMAGE), -X \"$(TRIDENT_CONFIG_PKG).DefaultACPImage=$(DEFAULT_ACP_IMAGE)\")"

# Functions

# trident_image_platforms returns a list of platforms that support the trident image. Currently only linux and windows
# are supported.
# usage: $(call trident_image_platforms,$(platforms))
trident_image_platforms = $(filter linux% windows%,$1)

# operator_image_platforms returns a list of platforms that support the operator image. Currently only linux is supported.
# usage: $(call operator_image_platforms,$(platforms))
operator_image_platforms = $(filter linux%,$1)

# all_image_platforms returns a list of all platforms supported by all images. Currently an alias for trident_image_platforms.
# usage: $(call all_image_platforms,$(platforms))
all_image_platforms = $(call trident_image_platforms,$1)

# os returns the OS from platform, i.e. 'linux' from 'linux/amd64'
# usage: $(call os,$(platform))
os = $(word 1,$(subst /, ,$1))

# arch returns the architecture from platform, i.e. 'amd64' from 'linux/amd64'
# usage: $(call arch,$(platform))
arch = $(word 2,$(subst /, ,$1))

# os_version returns the OS version from platform, i.e. 'ltsc2022' from 'windows/amd64/ltsc2022'
# usage: $(call os_version,$(platform))
os_version = $(word 3,$(subst /, ,$1))

# image_tag returns the image tag for a platform
# usage: $(call image_tag,$(image_name),$(platform))
image_tag = $1-$(call os,$2)-$(call arch,$2)$(if $(call os_version,$2),-$(call os_version,$2))

# binary_path returns the repo-relative path to a binary, depending on platform
# usage: $(call binary_path,$(binary_name),$(platform))
binary_path = bin/$(call os,$2)/$(call arch,$2)/$(1)$(if $(findstring windows,$(call os,$2)),.exe)

# usage: $(call binary_path,$(platform))
binary_path_without_name = bin/$(call os,$1)/$(call arch,$1)/

# go_env sets environment variables for go commands
# usage: $(call go_env,$(platform))
go_env = CGO_ENABLED=0 GOOS=$(call os,$1) GOARCH=$(call arch,$1)$(if $(GOPROXY), GOPROXY=$(GOPROXY))$(if $(GOFLAGS), GOFLAGS='$(GOFLAGS)')

# go_build returns the go build command for the named binary, platform, and source
# usage: $(call go_build,$(binary_name),$(source_path),$(platform),$(linker_flags))
go_build = echo $(call binary_path,$1,$3) && $(call go_env,$3) \
	$(GO_CMD) build \
	-o $(call binary_path,$1,$3) \
	-ldflags $4 \
	-gcflags="all=-N -l" \
	$2

# chwrap_build returns a script that will build chwrap.tar for the platform
# usage: $(call chwrap_build,$(platform),$(linker_flags))
chwrap_build = $(call go_build,chwrap,./chwrap,$1,$2)\
	&& ./chwrap/make-tarball.sh $(call binary_path,chwrap,$1) $(call binary_path,chwrap.tar,$1)\
	&& rm -f $(call binary_path,chwrap,$1)

# binaries_for_platform returns a script to build all binaries required for platform. The binaries are tridentctl,
# trident_orchestrator, chwrap.tar, and trident_operator. chwrap.tar and trident_operator are only built for linux
# plaforms.
# usage: $(call binaries_for_platform,$(platform),$(linker_flags))
binaries_for_platform = $(call go_build,tridentctl,./cli,$1,$2) \
	$(if $(findstring darwin,$1),,\
		&& $(call go_build,trident_orchestrator,.,$1,$2)\
		$(if $(findstring linux,$1),\
			&& $(call chwrap_build,$1,$2) ))

# build_binaries_for_platforms returns a script to build all binaries for platforms. Attempts to add current directory
# as a safe git directory, in case GO_SHELL uses a different user than the source repo.
# usage: $(call build_binaries_for_platforms,$(platforms),$(go_shell),$(linker_flags))
build_binaries_for_platforms = $(strip $(if $2,$2 'git config --global --add safe.directory $$(pwd) || true; )\
	$(foreach platform,$(call remove_version,$1),$(call binaries_for_platform,$(platform),$3)&&) true$(if $2,'))

# remove_version removes os_version from platforms
# usage: $(call remove_version,$(platforms))
remove_version = $(sort $(foreach platform,$1,$(call os,$(platform))/$(call arch,$(platform))))

# docker_build_linux returns the docker build command for linux images. Set output to `load` or `push` to load
# or push with docker buildx
# usage: $(call docker_build_linux,$(build_cli),$(platform),$(tag),$(output))
docker_build_linux = $1 build \
	--platform $2 \
	--build-arg ARCH=$(call arch,$2) \
	--build-arg BIN=$(call binary_path,trident_orchestrator,$2) \
	--build-arg CLI_BIN=$(call binary_path,tridentctl,$2) \
	--build-arg CHWRAP_BIN=$(call binary_path,chwrap.tar,$2) \
	--tag $3 \
	--rm \
	$(if $(findstring $(DOCKER_BUILDX_BUILD_CLI),$1),--builder trident-builder) \
	$(if $(findstring $(DOCKER_BUILDX_BUILD_CLI),$1),--$4) \
	.

# build_images_for_platforms returns a script that will build container images for platforms.
# usage: $(call build_images_for_platforms,$(platforms),$(build_cli),$(trident_tag),$(buildx_output))
build_images_for_platforms = $(foreach platform,$1,\
	$(if $(findstring linux,$(platform)),\
		$(call docker_build_linux,$2,$(platform),$3,$4))\
		&& docker push $3 \
	$(if $(findstring windows,$(platform)),\
		$(call docker_build_windows,$2,$(platform),$(call $3),$4)) &&) true

# Build targets
default:
	CGO_ENABLED=0 go build -o trident-debug

debug: images

# builds binaries using configured build tool (docker or go) for platforms
binaries:
	@$(call build_binaries_for_platforms,$(PLATFORMS),$(GO_SHELL),$(LINKER_FLAGS_DEBUG))

# builds docker images for platforms. As a special case, if only one platform is provided, the images are retagged
# without platform.
images: binaries
ifeq ($(BUILD_CLI),$(DOCKER_BUILDX_BUILD_CLI))
	-@$(call buildx_create_instance,$(BUILDX_CONFIG_FILE))
endif
	@$(call build_images_for_platforms,$(call all_image_platforms,$(PLATFORMS)),$(BUILD_CLI),$(TRIDENT_DEBUG_TAG),$(BUILDX_OUTPUT))

linker_flags:
	@echo $(LINKER_FLAGS)

clean:
	@rm -rf \
		bin \
		$(BUILDX_MANIFEST_DIR) \
		$(BUILDX_MANIFEST_DIR)_operator \
		trident-installer-*.tar.gz \
		trident-operator-*.tgz \
		image_digests.json \
		operator_image_digests.json \
		default binaries operator_binaries images operator_images manifest operator_manifest chart installer all
