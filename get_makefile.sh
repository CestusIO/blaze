
docker pull registry.gitlab.com/cestus/ci/runner-go:latest

docker run  --rm --volume $PWD:/w -v go-modules:/go/pkg/mod --workdir "/w" \
-v $HOME/.netrc:/root/.netrc \
 registry.gitlab.com/cestus/ci/runner-go:latest cp /scripts/golang/go.mk Makefile

