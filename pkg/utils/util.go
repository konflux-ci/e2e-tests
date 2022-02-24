package utils

import "os"

// CheckIfEnvironmentExists return true/false if the environment variable exists
func CheckIfEnvironmentExists(env string) bool {
	_, exist := os.LookupEnv(env)
	return exist
}

// Retrieve an environment variable. If will not exists will be used a default value
func GetEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
