package wait

import (
	"fmt"
	"strings"
)

// Split splits the templateRef into a triple of string corresponding to the `tier`, `type` and `revision`
// returns an error if this TemplateRef's format is invalid
func Split(templateRef string) (string, string, string, error) { // nolint:unparam
	parts := strings.SplitN(templateRef, "-", 3) // "<tier>-<type>-<based-on-tier-revision>-<template-revision>"
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid templateref: '%v'", templateRef)
	}
	return parts[0], parts[1], parts[2], nil
}
