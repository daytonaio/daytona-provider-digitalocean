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

func GetDroplet(client *godo.Client, dropletName string) (*godo.Droplet, error) {
	droplets, _, err := client.Droplets.ListByName(context.Background(), dropletName, nil)
	if err != nil {
		return nil, fmt.Errorf("error getting droplet: %v", err)
	}

	if len(droplets) > 0 {
		return &droplets[0], nil
	}

	return nil, fmt.Errorf("no droplet found with name %s", dropletName)

}
