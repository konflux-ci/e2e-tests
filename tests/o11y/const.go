package o11y

import "time"

const (
	O11yUser = "o11y-e2e"
	O11ySA   = "pipeline"

	o11yUserSecret string = "o11y-tests-token"

	pipelineRunTimeout = int(time.Duration(5) * time.Minute)
)
