#!/usr/bin/env bash
set -o verbose
set -eu
set -o pipefail
FILE="$JAVA_HOME/lib/security/cacerts"
if [ ! -f "$FILE" ]; then
    FILE="$JAVA_HOME/jre/lib/security/cacerts"
fi

if [ -f $(workspaces.tls.path)/service-ca.crt ]; then
    keytool -import -alias jbs-cache-certificate -keystore "$FILE" -file $(workspaces.tls.path)/service-ca.crt -storepass changeit -noprompt
fi


