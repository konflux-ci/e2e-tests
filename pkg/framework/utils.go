package framework

import "fmt"

func (c *ControllerHub) StoreAllArtifactsForNamespace(namespace string) error {
	var finalError string
	finalError = appendErrorToString(finalError, c.HasController.StoreAllApplications(namespace))
	finalError = appendErrorToString(finalError, c.HasController.StoreAllComponents(namespace))
	finalError = appendErrorToString(finalError, c.IntegrationController.StoreAllSnapshots(namespace))
	finalError = appendErrorToString(finalError, c.TektonController.StoreAllPipelineRuns(namespace))
	finalError = appendErrorToString(finalError, c.CommonController.StoreAllPods(namespace))
	if len(finalError) > 0 {
		return fmt.Errorf(finalError)
	}
	return nil
}

func appendErrorToString(baseString string, err error) string {
	if err != nil {
		return fmt.Sprintf("%s\n%s", baseString, err)
	}
	return baseString
}
