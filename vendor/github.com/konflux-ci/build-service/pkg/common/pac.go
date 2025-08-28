package common

// IsPaCApplicationConfigured checks if Pipelines as Code credentials configured for given provider.
// Application is preffered over webhook if possible.
func IsPaCApplicationConfigured(gitProvider string, config map[string][]byte) bool {
	isAppUsed := false

	switch gitProvider {
	case "github":
		if len(config[PipelinesAsCodeGithubAppIdKey]) != 0 || len(config[PipelinesAsCodeGithubPrivateKey]) != 0 {
			isAppUsed = true
		}
	default:
		// Application is not supported
		isAppUsed = false
	}

	return isAppUsed
}
