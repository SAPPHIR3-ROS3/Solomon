package main

import (
	"context"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/providerui"
)

type desktopConnectProviderRequest struct {
	APIKey  string
	BaseURL string
	Kind    int
	Name    string
}

// ConnectProvider delegates provider setup to Solomon's existing connect
// command flow, including authentication, model discovery, validation, and
// config persistence.
func (DesktopBridge) ConnectProvider(request desktopConnectProviderRequest) (desktopModelChoice, error) {
	result, err := providerui.Connect(context.Background(), providerui.ConnectRequest{
		APIKey:  request.APIKey,
		BaseURL: request.BaseURL,
		Kind:    request.Kind,
		Name:    request.Name,
	})
	if err != nil {
		return desktopModelChoice{}, err
	}
	return desktopModelChoice{Model: result.CurrentModel, Provider: result.CurrentProvider}, nil
}
