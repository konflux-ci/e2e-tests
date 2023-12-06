
# How to upload offline tokens to GitHub secrets for loadtest workflow

Staging users are splitted and stored as base64 encoded secrets - STAGING_USERS_1, STAGING_USERS_2, STAGING_USERS_3, STAGING_USERS_4
The GITHUB secrets link is <https://github.com/redhat-appstudio/e2e-tests/settings/secrets/actions>
or for any fork (like: naftalysh) in <https://github.com/naftalysh/e2e-tests/settings/secrets/actions>

# The users secrets creation process

* Input file - (ex: users_approved.list) space separated file of users records with structure as below:
  username (test-rhtap-1,..test-rhtap-100) password offline_token ssourl apiurl true

Prerequisite: that file is populated as part of Jan's process (<https://gitlab.cee.redhat.com/jhutar/mystoneinst/-/blob/main/README.md#get-offline-tokenpy>)

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
        "username": "username1",
        "password": "password1",
        "token": "token-1",
        "ssourl": "sso-token-url",
        "apiurl": "api-url",
        "verified": true
    },
    {
        "username": "username2",
        "password": "password2",
        "token": "token-2",
        "ssourl": "sso-token-url",
        "apiurl": "api-url",
        "verified": true
    }, ..
    { }
   ]
