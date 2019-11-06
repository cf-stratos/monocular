IMAGE_REPO ?= quay.io/helmpack/chartsvc
IMAGE_TAG ?= fdb-suse

docker-build:
	# We use the context of the root dir
	docker build --pull --rm -t ${IMAGE_REPO}:${IMAGE_TAG} -f Dockerfile ../../
