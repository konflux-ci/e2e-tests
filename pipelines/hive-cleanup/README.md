# Hive-cleanup pipeline

## Why?
We have limited resources on our Openstack instance we use for short-lived clusters provisioned by Hive (as a service). People tend to leave their cluster instances running for long time. This pipeline is intended to solve this by automatically deleting clusterclaims older than 7 days.

## Prerequisities
Secret named `hive-kubeconfig` having key `kubeconfig` and actual kubeconfig with access to the hive instance as a value.

## Instalation
```
$ cd pipelines/hive-cleanup
$ oc apply -f .
```
