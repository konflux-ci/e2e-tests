
# Document how to upload offline tokens to GitHub secrets for loadtest workflow

Staging users are splitted and stored as base64 encoded secrets - STAGING_USERS_1, STAGING_USERS_2, STAGING_USERS_3, STAGING_USERS_4
The GITHUB secrets link is <https://github.com/redhat-appstudio/e2e-tests/settings/secrets/actions>
or for any fork (like: naftalysh) in <https://github.com/naftalysh/e2e-tests/settings/secrets/actions>

# The users secrets creation process

* Input file - (ex: users_approved.list) space separated file of users records with structure as below:
  username (test-rhtap-1,..test-rhtap-100) password offline_token ssourl apiurl true

Prerequisite: that file is populated as part of Jan's process (<https://gitlab.cee.redhat.com/jhutar/mystoneinst/-/blob/main/README.md#get-offline-tokenpy>)

  a. get a space separated file like the input file above with just the two fields - username and password
     Let's assume that users_approved.list records have only the first two fields
  b. make sure to clone Jan's repo (<https://gitlab.cee.redhat.com/jhutar/mystoneinst>) first
  c. Jan's process to produce offline tokens per all the users used
    export INPUT_FILE="users_approved.list"
    export IFS=$'\n'
    for row in $( grep -v -e '^\s*$' -e '^\s*#' $INPUT_FILE ); do
        U=$( echo "$row" | cut -d ' ' -f 1 )
        P=$( echo "$row" | cut -d ' ' -f 2 )
        echo "===== $U - $P ====="
        ./get-offline-token.py --results-file test.json --test-concurrency 1 --test-target <https://console.redhat.com> --test-username "$U" --test-password "$P" --selenium-url <http://url:port_num> -d 2>&1 | tee "get-offline-token-$U.log"
        T="$( grep 'Offline token' "get-offline-token-$U.log" | cut -d ' ' -f 9 )"
        sed -i "s|^\($U .*\)|\1 $T ssourl apiurl true|" $INPUT_FILE
    done

* Process

## Convert space separated file into a json file

  `convert_users_list_to_json_v2.sh users_approved.list users_approved.json`  

## Split json file and base64 encode it into 4 separate files  

  `json_split_and_encode_v2.sh users_approved.json 4`
  The base64 encoded files produced are in part_1.encoded, part_2.encoded, part_3.encoded, part_4.encoded

  Comment: convert_users_list_to_json_v2.sh and json_split_and_encode_v2.sh exist in tests/load-tests/ci-scripts directory next to merge-json.sh script

## Store the above files contents into STAGING_USERS_1, STAGING_USERS_2, STAGING_USERS_3, STAGING_USERS_4 GITHUB secrets

* To validate the secrets -

# Test the loadtest workflow in my forked repo and it should show that the users.json file is

   created and used in the "Prepare Load Test" step
   (<https://github.com/naftalysh/e2e-tests/actions/workflows/loadtest.yaml>)

> Runs `./ci-scripts/merge-json.sh $STAGING_USERS_1 $STAGING_USERS_2 $STAGING_USERS_3 $STAGING_USERS_4`
> shell: /usr/bin/bash -e {0}
> env:
> ARTIFACT_DIR: /home/runner/work/e2e-tests/e2e-tests/tests/load-test/.artifacts
> STAGING_USERS_1:***
> STAGING_USERS_2:***
> STAGING_USERS_3:***
> STAGING_USERS_4:***
>  
> Decoded JSON data merged and stored in users.json.

# Produce locally the users.json file that is used by the loadtest workflow

   Use part_1.encoded, part_2.encoded, part_3.encoded, part_4.encoded that were created in the process above -

   export STAGING_USERS_1=$(cat part_1.encoded)
   export STAGING_USERS_2=$(cat part_2.encoded)
   export STAGING_USERS_3=$(cat part_3.encoded)
   export STAGING_USERS_4=$(cat part_4.encoded)

   Run the /tests/load-tests/ci-scripts/merge-json.sh script and it will produce the users.json file
    as it's used in the loadtest.yaml workflow
   ./merge-json.sh STAGING_USERS_1 STAGING_USERS_2 STAGING_USERS_3 STAGING_USERS_4                                                                                         ─╯

   The output:
   Decoded JSON data merged and stored in users.json.

   Open file users.json and you can see it's a users json file that's organized as array of users objects
   [
    {
        "username": "test-rhtap-1",
        "password": "kqwkdqkdoqkwd",
        "token": "eyJhbGciOiJIUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJhZDUyMjdhMy1iY2ZkLTRjZjAtYTdiNi0zOTk4MzVhMDg1NjYifQ.",
        "ssourl": "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token",
        "apiurl": "https://api-toolchain-host-operator.apps.stone-stg-host.qc0p.p1.openshiftapps.com",
        "verified": true
    },
    {
        "username": "test-rhtap-2",
        "password": "kqwkdqkdowwwd",
        "token": "eyJabcciOiJIUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJhZDUyMjdhMy1iY2ZkLTRjZjAtYTdiNi0zOTk4MzVhMDg1NjYifQ.",
        "ssourl": "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token",
        "apiurl": "https://api-toolchain-host-operator.apps.stone-stg-host.qc0p.p1.openshiftapps.com",
        "verified": true
    }, ..
    { }
   ]
