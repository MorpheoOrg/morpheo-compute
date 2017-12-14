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

# Targets (files & phony targets)
TARGETS = api worker
BIN_TARGETS = $(foreach TARGET,$(TARGETS),$(TARGET)/build/target)
BIN_CLEAN_TARGETS = $(foreach TARGET,$(TARGETS),$(TARGET)/build/target/clean)
DOCKER_TARGETS = $(foreach TARGET,$(TARGETS),$(TARGET)-docker)
DOCKER_CLEAN_TARGETS = $(foreach TARGET,$(TARGETS),$(TARGET)-docker-clean)

DEP_CONTAINER = \
	docker run -it --rm \
	  --workdir "/go/src/github.com/MorpheoOrg/morpheo-compute" \
	  -v $${PWD}:/go/src/github.com/MorpheoOrg/morpheo-compute \
		golang:1.9

## Project-wide targets
bin: $(BIN_TARGETS)
bin-clean: $(BIN_CLEAN_TARGETS)
docker: $(DOCKER_TARGETS)
docker-clean: $(DOCKER_CLEAN_TARGETS)

clean: docker-clean bin-clean vendor-clean

.DEFAULT: bin
.PHONY: bin bin-clean \
	    vendor-docker vendor-update vendor-replace-local \
	    tests \
		docker docker-clean $(DOCKER_TARGETS) $(DOCKER_CLEAN_TARGETS)

# 1. Building
%/build/target: %/*.go # ../morpheo-go-packages/common/*.go ../morpheo-go-packages/client/*.go
	@echo "Building $(subst /build/target,,$(@)) binary..........................................................................."
	@mkdir -p $(@D)
	@CGO_ENABLED=1 GOOS=linux go build -a --installsuffix cgo -o $@ ./$(dir $<)
	@# TODO: $(eval OUTPUT = $(shell go build -v -o $@ ./$(subst /build/target,,$(@)) 2>&1 | grep -v "github.com/MorpheoOrg/morpheo-compute/"))
	@# TODO: $(if $(-z $(OUTPUT)); @echo "Great Success",@echo "\n***EXTERNAL PACKAGES***\n"$(OUTPUT))

%/build/target/clean:
	@echo "Removing $(subst /build/target,,$(@)) binary..."
	rm -f $(@D)

# 2. Vendoring
vendor: Gopkg.toml
	@echo "Pulling dependencies with dep..."
	dep ensure

# build vendor in a container
vendor-docker:
	@echo "Pulling dependencies with dep... in a container"
	rm -rf ./vendor
	mkdir ./vendor
	$(DEP_CONTAINER) bash -c \
		"go get -u github.com/golang/dep/cmd/dep && dep ensure && chown $(shell id -u):$(shell id -g) -R ./Gopkg.lock ./vendor"

vendor-update:
	@echo "Updating dependencies with dep..."
	dep ensure -update

vendor-replace-local:
	@echo "Replacing vendor/github.com/MorpheoOrg by local repository..."
	@rm -rf ./vendor/github.com/MorpheoOrg
	@mkdir -p ./vendor/github.com/MorpheoOrg
	@cp -Rf ../morpheo-go-packages ./vendor/github.com/MorpheoOrg/morpheo-go-packages
	@rm -rf ./vendor/github.com/MorpheoOrg/morpheo-go-packages/vendor

# 3. Testing
tests: vendor-replace-local
	go test ./worker

%-tests: vendor-replace-local
	go test ./$(subst -tests,,$(@))

# 4. Packaging
$(DOCKER_TARGETS): %-docker: %/build/target
	@echo "Building the $(DOCKER_REPO)/compute-$(subst -docker,,$(@)):$(DOCKER_TAG) Docker image"
	docker build -t $(DOCKER_REPO)/compute-$(subst -docker,,$(@)):$(DOCKER_TAG) \
	  ./$(subst -docker,,$(@))

$(DOCKER_CLEAN_TARGETS):
	@echo "Deleting the $(DOCKER_REPO)/compute-$(subst -docker,,$(@)):$(DOCKER_TAG) Docker image"
	docker rmi $(DOCKER_REPO)/compute-$(subst -docker-clean,,$(@)):$(DOCKER_TAG) || \
		echo "No $(DOCKER_REPO)/compute-$(subst -docker-clean,,$(@)):$(DOCKER_TAG) docker image to remove"
