package util

import (
	"context"
	"fmt"

	"github.com/daytonaio/daytona/pkg/workspace"
	"github.com/digitalocean/godo"
)

func GetDropletName(project *workspace.Project) string {
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

func GetVolumeByName(client *godo.Client, name string) (*godo.Volume, error) {
	volumes, _, err := client.Storage.ListVolumes(context.Background(), &godo.ListVolumeParams{Name: name})
	if err != nil {
		return nil, err
	} else if len(volumes) > 1 {
		return nil, fmt.Errorf("multiple volumes with name %s found", name)
	} else if len(volumes) == 0 {
		return nil, nil
	}

	return &volumes[0], nil
}
