# Red Hat AppStudio Load Test Scripts

This Test Section Provides Load Testing Scripts for Red Hat AppStudio 

## Requirements 

- Install AppStudio in Preview Mode refer [infra-deployments](https://github.com/redhat-appstudio/infra-deployments) for installation details 
- Install sandbox operators in e2e mode
- Encode your docker config json to a base64 encoded string 

## Running the script
1. Change your directory to `tests/load-tests` 
2. Environment variables are required to set for the e2e framework that the load test uses. Refer to [Running the tests](https://github.com/redhat-appstudio/e2e-tests#running-the-tests).
3. Run the bash script
```
./run.sh 
```
For help run `go run main.go --help`.
You can configure the parameters by editing `run.sh` and add/change parameters(e.g. number of users, number of batches...).

## How does this work 
The Script works in Steps
- Starts by creating `n` number of UserSignup CRD's which will create `n` number of NameSpaces , number of users can be changed by the flag `--users`
- Then it proceeds by creating AppStudio Applications for each user followed by Appstudio Component, i.e Creates users on a 1:1 basis 
- Creating the Component will start the pipelines, if the `-w` flag is given it will wait for the pipelines to finish then print results 
- Then after the tests are completed it will dump the results / stats, on error the stats will still get dumped along with the trace

## How to contribute
Just edit the file `cmd/loadTests.go` 
