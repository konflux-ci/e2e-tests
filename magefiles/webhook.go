package main

import (
	"fmt"
	"github.com/averageflow/gohooks/v2/gohooks"
	"net/http"
)

// Webhook struct used for sending webhooks to https://smee.io/
type Webhook struct {
	Path          string `json:"path"`
	RepositoryURL string `json:"repository_url"`
	Repository    `json:"repository"`
}

// Repository struct - part of Webhook struct
type Repository struct {
	FullName   string `json:"full_name"`
	PullNumber string `json:"pull_number"`
}

func (w *Webhook) CreateAndSend(saltSecret, webhookTarget string) (*http.Response, error) {
	hook := &gohooks.GoHook{}
	hook.Create(w, w.Path, saltSecret)
	resp, err := hook.Send(webhookTarget)
	if err != nil {
		return nil, fmt.Errorf("error sending webhook: %+v", err)
	}
	return resp, nil
}
