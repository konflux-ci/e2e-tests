export DOCKER_CONFIG_JSON=ewogICJhdXRocyI6IHsKICAgICJxdWF5LmlvIjogewogICAgICAiYXV0aCI6ICJiWE5oZDI5dlpEcEhhMGt3WWpsUWMwdGpialZoZGxSdEswWmxOQ3RvZG1SUmVGaHBTVGhFVG1Oa2VEWnVOVUk1YjBJNFJUZHBRUzkyTldoaGJWTTVOWEJyWlhaRFVUaDMiLAogICAgICAiZW1haWwiOiAiIgogICAgfQogIH0KfQ==

if [ -z ${DOCKER_CONFIG_JSON+x} ]; then echo "env DOCKER_CONFIG_JSON need to be defined"; exit 1;  else echo "DOCKER_CONFIG_JSON is set"; fi

go run loadtest.go --username testuser --users 200 --batch 25 --waitpipelines


