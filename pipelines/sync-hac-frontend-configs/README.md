# Pipeline for syncing private frontend-configs repo to public github repo
### Why?
Private frontend-configs repo (hosted on internal gitlab instance) is the last thing
preventing us from being able to deploy ephemeral HAC from public internet (openshift-ci).

### What?
This pipeline just clones private gitlab repo, adds remote for public github and pushes it there.

In case of any failure, this pipeline sends message to Stonesoup QE Slack Channel

## Installation
* Create secrets for 
  * ssh access to gitlab ([documentation](https://tekton.dev/vault/pipelines-v0.15.2/auth/#ssh-authentication-git))
  * basic-auth for github ([documentation](https://tekton.dev/vault/pipelines-v0.15.2/auth/#basic-authentication-git))
  * slack token used for posting the messages to slack named `slack-token` (key is `token`)
* Link the git access secrets to `pipeline` service account:
    ```
    $ oc secrets link pipeline gitlab-ssh
    $ oc secrets link pipeline github-basic
    ```
* Apply all resources from this folder
    ```
    $ oc apply -f .
    ```