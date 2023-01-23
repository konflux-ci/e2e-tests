# Red Hat AppStudio Load Test Scripts

This Test Section Provides Load Testing Scripts for Red Hat AppStudio 

## Requirements 

- Install AppStudio in Preview Mode refer [infra-deployments](https://github.com/redhat-appstudio/infra-deployments) for installation details 
- Install sandbox operators in e2e mode
- Encode your docker config json to a base64 encoded string 

## Running the script
1. Change your directory to `tests/load-tests` 
2. Open `run.sh` and add your encoded docker config json:
```bash
  DOCKER_CONFIG_JSON=<PLEASE_ENTER_BASE64_ENCODED_CONFIG>
```
3. Run the bash script
```
./run.sh 
```
For help run `go run main.go --help`.
You can configure the parameters by editing `run.sh` and add/change parameters(e.g. number of users, number of batches...).

## How does this work 
The Script works in Steps
- Starts by creating `n` number of UserSignup CRD's which will create `n` number of NameSpaces , number of users can be changed by the flag `--users`
- Next the Script Adds a Secret named `redhat-appstudio-registry-pull-secret` which will contain the docker config you provided when you run the script
- Then it proceeds by creating AppStudio Applications for each user followed by Appstudio Component , i.e Creates users on a 1:1 basis 
- Creating the Component will start the pipelines , if the `-w` flag is given it will wait for the pipelines to finish then print results 
- Then after the tests are completed it will dump the results / stats , on error the stats will still get dumped along with the trace

## How to contribute
Just edit the file `cmd/loadTests.go` 
