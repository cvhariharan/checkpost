.PHONY: build serve clean

DOCKER ?= docker
DOCKER_FLAGS ?= --rm -it
DOCKER_USER_FLAGS ?= --user $(shell id -u):$(shell id -g)
ZENSICAL_IMAGE ?= zensical/zensical

build:
	@$(DOCKER) run $(DOCKER_FLAGS) $(DOCKER_USER_FLAGS) -v "$(CURDIR):/docs" $(ZENSICAL_IMAGE) build

serve:
	@$(DOCKER) run $(DOCKER_FLAGS) $(DOCKER_USER_FLAGS) -p 8000:8000 -v "$(CURDIR):/docs" $(ZENSICAL_IMAGE)

clean:
	@rm -rf site .cache
