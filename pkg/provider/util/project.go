package util

import (
	"context"
	"fmt"

	"github.com/daytonaio/daytona/pkg/types"
	"github.com/digitalocean/godo"
)

func GetDropletName(project *types.Project) string {
	return fmt.Sprintf("%s-%s", project.Name, project.WorkspaceId)
}

func GetDropletIDByName(client *godo.Client, dropletName string) (int, error) {
	// Get all droplets
	droplets, _, err := client.Droplets.List(context.Background(), nil)
	if err != nil {
		return 0, fmt.Errorf("error getting droplets: %v", err)
	}

	// Find the droplet with the given name
	for _, droplet := range droplets {
		if droplet.Name == dropletName {
			return droplet.ID, nil
		}
	}

	return 0, fmt.Errorf("no droplet found with name %s", dropletName)
}
