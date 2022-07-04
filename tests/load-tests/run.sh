export DOCKER_CONFIG_JSON=

if [ -z ${DOCKER_CONFIG_JSON+x} ]; then echo "env DOCKER_CONFIG_JSON need to be defined"; exit 1;  else echo "DOCKER_CONFIG_JSON is set"; fi

go run loadtest.go --username testuser --users 50 --batch 10 -w && ./clear.sh


