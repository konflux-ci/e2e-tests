package spi

import (
	kubeCl "github.com/redhat-appstudio/e2e-tests/pkg/clients/kubernetes"
)

type SPIController struct {
	*kubeCl.CustomClient
}

func NewSuiteController(kube *kubeCl.CustomClient) (*SPIController, error) {
	// Initialize a new SPI controller with just the kube client
	return &SPIController{
		kube,
	}, nil
}
