
# Redhat AppStudio Load Test Scripts

This Test Section Provides Performance Testing Scripts for Redhat AppStudio 

## Requirements 

- Install AppStudio in Preview Mode refer [infra-deployments](https://github.com/redhat-appstudio/infra-deployments) for installation details 
- Install sandbox operators in e2e mode
- encode your docker config json to a base64 encoded string 

## Running the script
- change your directory to `tests/load_tests` 
- Open `run.sh` and add your encoded docker config json

```bash
  DOCKER_CONFIG_JSON=<PLEASE_ENTER_BASE64_ENCODED_CONFIG>
```
- Next run the bash script
```
./run.sh 
```  
- to configure the prams open `run.sh` and add prams that you like 
- run `go run main.go --help` for help 