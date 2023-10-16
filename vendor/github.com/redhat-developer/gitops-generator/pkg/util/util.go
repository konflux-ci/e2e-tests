/* Copyright 2022 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"errors"
	"math/rand"
	"net/url"
	"regexp"
	"strings"
)

var invalidRemoteMsg = errors.New("remote URL is invalid or missing the https scheme and/or supported github.com or gitlab.com hosts")

// ValidateRemote minimally validates the remote gitops URL to ensure it contains the "https" scheme and supported "github.com" and "gitlab.com" hosts
func ValidateRemote(remote string) error {
	remoteURL, parseErr := url.Parse(remote)
	if parseErr != nil {
		return invalidRemoteMsg
	}

	if remoteURL.Scheme == "https" && (remoteURL.Host == "github.com" || remoteURL.Host == "gitlab.com") {
		return nil
	}

	return invalidRemoteMsg
}

/* #nosec G101 -- regex for remote url segment that can contain a token.  This is not a hardcoded token*/
const (
	tokenRegex  = `(https:\/\/)(\w+)@`
	schemaBytes = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

// SanitizeErrorMessage takes in a given error message and returns a new, sanitized error with things like tokens removed
func SanitizeErrorMessage(err error) error {
	reg := regexp.MustCompile(tokenRegex)
	matches := reg.FindAllStringSubmatch(err.Error(), -1)
	newErrMsg := err.Error()

	for _, v := range matches {
		// check for length of 3 because this includes the string match for the entire regex and sub-matches to the two capturing groups
		if len(v) == 3 {
			// use newErrMsg in subsequent iterations to ensure multiple tokens in a message get redacted
			newErrMsg = strings.Replace(newErrMsg, v[2], "<TOKEN>", 1)
		}
	}

	return errors.New(newErrMsg)
}

// GetRandomString returns a random string which is n characters long.
// If lower is set to true a lower case string is returned.
func GetRandomString(n int, lower bool) string {
	b := make([]byte, n)
	for i := range b {
		/* #nosec G404 -- not used for cryptographic purposes*/
		b[i] = schemaBytes[rand.Intn(len(schemaBytes)-1)]
	}
	randomString := string(b)
	if lower {
		randomString = strings.ToLower(randomString)
	}
	return randomString
}
