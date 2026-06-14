.PHONY: build serve clean

DOCKER ?= docker
DOCKER_FLAGS ?= --rm
ZENSICAL_IMAGE ?= zensical/zensical

build:
	@$(DOCKER) run $(DOCKER_FLAGS) -v "$(CURDIR):/docs:z" $(ZENSICAL_IMAGE) build

serve:
	@$(DOCKER) run $(DOCKER_FLAGS) -p 8000:8000 -v "$(CURDIR):/docs:z" $(ZENSICAL_IMAGE)

clean:
	@rm -rf site .cache
