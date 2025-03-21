#!/bin/sh

COUNT=1000
CONCURRENCY=10

function create_stuff() {
    local u_seq="${1}"

    USERNAME="test${u_seq}"
    USEREMAIL="jhutar+${USERNAME}@redhat.com"
    NAMESPACE="${USERNAME}-tenant"
    APPLICATION="my-app"
    COMPONENT="my-comp"
    GIT_URL="https://github.com/rhtap-test-local/testrepo"
    GIT_CONTEXT="/"
    GIT_BRANCH="main"

    start=$( date +%s )
    echo "$( date --utc -Ins ) DEBUG Creating user ${USERNAME}"

    # Create user
    oc apply -f - <<EOF
apiVersion: toolchain.dev.openshift.com/v1alpha1
kind: UserSignup
metadata:
  name: "${USERNAME}"
  namespace: toolchain-host-operator
  annotations:
    toolchain.dev.openshift.com/user-email: "${USEREMAIL}"
  labels:
    toolchain.dev.openshift.com/email-hash: "$( echo -n "${USEREMAIL}" | md5sum | cut -d " " -f 1 )"
    ###toolchain.dev.openshift.com/state: approved
spec:
  userID: "${USERNAME}"
  username: "${USERNAME}"
  identityClaims:
    preferredUsername: "${USERNAME}"
    email: "${USEREMAIL}"
    sub: "I_do_not_know_what_to_put_here"
  ###states:
  ###  - approved
EOF

    # Wait for namespace to be created
    ns_start=$( date +%s )
    while true; do
        ns_now=$( date +%s )
        if [ $(( $ns_now - $ns_start )) -ge 100 ]; then
            echo "$( date --utc -Ins ) WARNING Failed to create ${NAMESPACE} namespace in time, giving up"
            return
        fi
        oc get "namespace/${NAMESPACE}" -o name && break
        sleep 3
    done

    # Create some secret
    oc apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: pipelines-as-code-secret
  namespace: $NAMESPACE
  labels:
    appstudio.redhat.com/credentials: scm
    appstudio.redhat.com/scm.host: gitlab.cee.redhat.com
type: kubernetes.io/basic-auth
stringData:
  password: ...
EOF

    # Create application
    oc apply -f - <<EOF
apiVersion: appstudio.redhat.com/v1alpha1
kind: Application
metadata:
  name: $APPLICATION
  namespace: $NAMESPACE
spec:
  displayName: $APPLICATION
EOF

    # Create component
    oc apply -f - <<EOF
apiVersion: appstudio.redhat.com/v1alpha1
kind: Component
metadata:
  name: $COMPONENT
  namespace: $NAMESPACE
  annotations:
    build.appstudio.openshift.io/request: 'configure-pac'
    build.appstudio.openshift.io/pipeline: '{"name":"docker-build-multi-platform-oci-ta","bundle":"latest"}'
    ###git-provider: gitlab
    ###git-provider-url: https://gitlab.cee.redhat.com
spec:
  application: $APPLICATION
  componentName: $COMPONENT
  source:
    git:
      dockerfileUrl: Dockerfile
      revision: $GIT_BRANCH
      url: $GIT_URL
      context: $GIT_CONTEXT
EOF

    # Create image repository
    oc apply -f - <<EOF
apiVersion: appstudio.redhat.com/v1alpha1
kind: ImageRepository
metadata:
  annotations:
    image-controller.appstudio.redhat.com/update-component-image: 'true'
  name: $USERNAME
  namespace: $NAMESPACE
  labels:
    appstudio.redhat.com/application: $APPLICATION
    appstudio.redhat.com/component: $COMPONENT
spec:
  image:
    name: $NAMESPACE/$COMPONENT
    visibility: public
  notifications:
    - config:
        url: https://bombino.api.redhat.com/v1/sbom/quay/push
      event: repo_push
      method: webhook
      title: SBOM-event-to-Bombino
EOF

    # Create integration test scenario
    oc apply -f - <<EOF
apiVersion: appstudio.redhat.com/v1beta2
kind: IntegrationTestScenario
metadata:
  annotations:
    test.appstudio.openshift.io/kind: enterprise-contract
  name: ${APPLICATION}-enterprise-contract
  namespace: $NAMESPACE
spec:
  application: $APPLICATION
  contexts:
  - description: execute the integration test in all cases - this would be the default state
    name: application
  resolverRef:
    params:
    - name: url
      value: https://github.com/konflux-ci/build-definitions
    - name: revision
      value: main
    - name: pathInRepo
      value: pipelines/enterprise-contract.yaml
    resolver: git
EOF

    # Create 10 release plans
    for rp_seq in $( seq 0 9); do
        oc apply -f - <<EOF
apiVersion: appstudio.redhat.com/v1alpha1
kind: ReleasePlan
metadata:
  name: ${USERNAME}-${rp_seq}
  namespace: $NAMESPACE
  labels:
    release.appstudio.openshift.io/auto-release: 'true'
    release.appstudio.openshift.io/standing-attribution: 'true'
    release.appstudio.openshift.io/releasePlanAdmission: 'rpa-experimental'
spec:
  application: $APPLICATION
  data:
    key0: value0
    key1: value1
    key2: value2
    key3: value3
    key4: value4
    key5: value5
    key6: value6
    key7: value7
    key8: value8
    key9: value9
  ###pipelineRef: <pipeline-ref>
  ###serviceAccount: <service-account>
  ###releaseGracePeriodDays: <days>
  target: managed-workspace
EOF
    done

    end=$( date +%s )
    echo "$( date --utc -Ins ) DEBUG Created user ${USERNAME} in $(( $end - $start )) seconds"
}


function pwait() {
    while [[ $(jobs -p -r | wc -l) -ge $1 ]]; do
        echo "$( date --utc -Ins ) DEBUG Wating for one job to finish"
        wait -n
    done
}


date -Ins --utc >started

for u_seq in $( seq 0 $((COUNT-1)) ); do
    create_stuff "${u_seq}" &
    pwait $CONCURRENCY
done
wait

date -Ins --utc >ended


# Fake files to make collect script to pass
touch load-test-timings.csv
echo '{"COUNT":'"$COUNT"',"CONCURRENCY":'"$CONCURRENCY"'}' >load-test-options.json
