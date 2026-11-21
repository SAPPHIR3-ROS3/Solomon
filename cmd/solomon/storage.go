package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/chatstore"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/paths"
	"github.com/SAPPHIR3-ROS3/Solomon/v2026/internal/project"
)

func verifyLegacyRegistry(ctx context.Context, output io.Writer) error {
	path, err := paths.ProjectsMapPath()
	if err != nil {
		return err
	}
	differences, err := project.VerifyLegacyRegistry(ctx, path)
	if err != nil {
		return fmt.Errorf("verify legacy project registry: %w", err)
	}
	return json.NewEncoder(output).Encode(differences)
}

func verifyLegacyChats(ctx context.Context, output io.Writer) error {
	differences, err := chatstore.VerifyLegacyChats(ctx)
	if err != nil {
		return fmt.Errorf("verify legacy chats: %w", err)
	}
	return json.NewEncoder(output).Encode(differences)
}
