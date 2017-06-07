#
# Copyright Morpheo Org. 2017
#
# contact@morpheo.co
#
# This software is part of the Morpheo project, an open-source machine
# learning platform.
#
# This software is governed by the CeCILL license, compatible with the
# GNU GPL, under French law and abiding by the rules of distribution of
# free software. You can  use, modify and/ or redistribute the software
# under the terms of the CeCILL license as circulated by CEA, CNRS and
# INRIA at the following URL "http://www.cecill.info".
#
# As a counterpart to the access to the source code and  rights to copy,
# modify and redistribute granted by the license, users are provided only
# with a limited warranty  and the software's author,  the holder of the
# economic rights,  and the successive licensors  have only  limited
# liability.
#
# In this respect, the user's attention is drawn to the risks associated
# with loading,  using,  modifying and/or developing or reproducing the
# software by the user in light of its specific status of free software,
# that may mean  that it is complicated to manipulate,  and  that  also
# therefore means  that it is reserved for developers  and  experienced
# professionals having in-depth computer knowledge. Users are therefore
# encouraged to load and test the software's suitability as regards their
# requirements in conditions enabling the security of their systems and/or
# data to be ensured and,  more generally, to use and operate it in the
# same conditions as regards security.
#
# The fact that you are presently reading this means that you have had
# knowledge of the CeCILL license and that you accept its terms.
#

# User defined variables (use env. variables to override)
DOCKER_REPO ?= registry.morpheo.io
DOCKER_TAG ?= $(shell git rev-parse --verify --short HEAD)

# (Containerized) build commands
BUILD_CONTAINER = \
  docker run -u $(shell id -u) -it --rm \
	  --workdir "/usr/local/go/src/github.com/MorpheoOrg/compute" \
	  -v $${PWD}:/usr/local/go/src/github.com/MorpheoOrg/compute:ro \
	  -v $${PWD}/vendor:/vendor/src \
	  -e GOPATH="/go:/vendor" \
	  -e CGO_ENABLED=0 \
	  -e GOOS=linux

GLIDE_CONTAINER = \
	docker run -it --rm \
	  --workdir "/usr/local/go/src/github.com/MorpheoOrg/compute" \
	  -v $${PWD}:/usr/local/go/src/github.com/MorpheoOrg/compute \
		$(BUILD_CONTAINER_IMAGE)

BUILD_CONTAINER_IMAGE = golang:1-onbuild

GOBUILD = go build --installsuffix cgo --ldflags '-extldflags \"-static\"'
GOTEST = go test

# Targets (files & phony targets)
TARGETS = api worker
TEST_TARGETS = $(foreach TARGET,$(TARGETS),$(TARGET)-test)
BIN_TARGETS = $(foreach TARGET,$(TARGETS),$(TARGET)-bin)
BIN_CLEAN_TARGETS = $(foreach TARGET,$(TARGETS),$(TARGET)-bin-clean)
DOCKER_TARGETS = $(foreach TARGET,$(TARGETS),$(TARGET)-docker)
DOCKER_CLEAN_TARGETS = $(foreach TARGET,$(TARGETS),$(TARGET)-docker-clean)

## Project-wide targets
test: $(TEST_TARGETS)
bin: $(BIN_TARGETS)
bin-clean: $(CLEAN_TARGETS)
docker: $(DOCKER_TARGETS)
docker-clean: $(DOCKER_CLEAN_TARGETS)

clean: docker-clean bin-clean vendor-clean

.DEFAULT: bin
.PHONY: test bin bin-clean clean docker docker-clean clean vendor-clean \
	      vendor-update $(TARGETS) $(TEST_TARGETS) $(BIN_TARGETS) \
				$(BIN_CLEAN_TARGETS) $(DOCKER_TARGETS) $(DOCKER_CLEAN_TARGETS)
.FORCE:

# 1. Vendoring
vendor: glide.yaml
	@echo "Pulling dependencies with glide... in a build container"
	rm -rf ./vendor
	mkdir ./vendor
	$(GLIDE_CONTAINER) bash -c \
		"go get github.com/Masterminds/glide && glide install && chown $(shell id -u):$(shell id -g) -R ./glide.lock ./vendor"

vendor-update:
	@echo "Pulling dependencies with glide... in a build container"
	$(GLIDE_CONTAINER) bash -c \
		"go get github.com/Masterminds/glide && glide update && chown $(shell id -u):$(shell id -g) -R ./glide.lock ./vendor"

vendor-clean:
	@echo "Dropping the vendor folder"
	rm -rf ./vendor

# 2. Testing
$(TEST_TARGETS): vendor
	@echo "Running go test in $(subst -test,,$(@)) directory"
	$(BUILD_CONTAINER) -v $${PWD}/$(@D):/build:rw $(BUILD_CONTAINER_IMAGE) \
    bash -c "cd $(subst -test,,$(@)) && $(GOTEST) "

# 3. Compiling
$(BIN_TARGETS):
	@echo "Building $(subst -bin,,$(@)) binary"
	$(MAKE) $(subst -bin,,$(@))/build/target

$(BIN_CLEAN_TARGETS):
	@echo "Removing $(subst -bin-clean,,$(@))/build directory"
	rm -rf $(subst -bin-clean,,$(@))/build

%/build/target: %/*.go vendor
	mkdir -p $${PWD}/$(@D)
	$(BUILD_CONTAINER) -v $${PWD}/$(@D):/build:rw $(BUILD_CONTAINER_IMAGE) \
		$(GOBUILD) -o /build/target ./$(dir $<)

# 4. Packaging
$(DOCKER_TARGETS): %-docker: %/build/target
	@echo "Building the $(DOCKER_REPO)/$(subst -docker,,$(@)):$(DOCKER_TAG) Docker image"
	docker build -t $(DOCKER_REPO)/$(subst -docker,,$(@)):$(DOCKER_TAG) \
	  ./$(subst -docker,,$(@))

$(DOCKER_CLEAN_TARGETS):
	@echo "Deleting the $(DOCKER_REPO)/$(subst -docker,,$(@)):$(DOCKER_TAG) Docker image"
	docker rmi $(DOCKER_REPO)/$(subst -docker-clean,,$(@)):$(DOCKER_TAG) || \
		echo "No $(DOCKER_REPO)/$(subst -docker-clean,,$(@)):$(DOCKER_TAG) docker image to remove"
