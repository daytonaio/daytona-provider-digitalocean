package util

import (
	"context"
	"fmt"
	"time"

	provider_types "github.com/daytonaio/daytona-provider-digitalocean/pkg/types"
	"github.com/daytonaio/daytona/pkg/provider/util"
	"github.com/daytonaio/daytona/pkg/types"
	"github.com/digitalocean/godo"
)

func CreateDroplet(client *godo.Client, project *types.Project, targetOptions *provider_types.TargetOptions, serverDownloadUrl string) (*godo.Droplet, error) {
	// retrieve user data
	userData := `#!/bin/bash
    # Create Daytona user
    useradd daytona -m -s /bin/bash
	if grep -q sudo /etc/groups; then
		usermod -aG sudo daytona
	elif grep -q wheel /etc/groups; then
		usermod -aG wheel daytona
	fi
	echo "daytona ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/91-daytona
	`

	for k, v := range project.EnvVars {
		userData += "echo 'export " + k + "=" + v + "' >> /etc/profile.d/daytona_env_vars.sh\n"
		userData += "export " + k + "=" + v + "\n"
	}

	userData += "echo 'export DAYTONA_WS_DIR=/home/daytona/project' >> /etc/profile.d/daytona_env_vars.sh\n"
	userData += "export DAYTONA_WS_DIR=/home/daytona/project\n"

	// TODO: "DAYTONA_WS_DIR=" + path.Join("/workspaces", project.Name),
	userData += "su daytona\n"
	userData += util.GetProjectStartScript(serverDownloadUrl, project.ApiKey)

	// Get the droplet name
	dropletName := GetDropletName(project)

	// Check if a droplet with the same name already exists
	droplets, _, err := client.Droplets.List(context.Background(), nil)
	if err != nil {
		return nil, fmt.Errorf("error listing droplets: %v", err)
	}
	for _, d := range droplets {
		if d.Name == dropletName {
			return nil, fmt.Errorf("a droplet with the name %s already exists", dropletName)
		}
	}

	// generate instance object
	instance := &godo.DropletCreateRequest{
		Name:   GetDropletName(project),
		Region: targetOptions.Region,
		Size:   targetOptions.Size,
		Image: godo.DropletCreateImage{
			Slug: targetOptions.Image,
		},
		UserData: userData,
		Tags:     []string{"daytona"},
	}

	// Create the droplet
	droplet, _, err := client.Droplets.Create(context.Background(), instance)
	if err != nil {
		return nil, fmt.Errorf("error creating droplet: %v", err)
	}

	// Poll the droplet's status until it becomes active
	for {
		droplet, _, err = client.Droplets.Get(context.Background(), droplet.ID)
		if err != nil {
			return nil, fmt.Errorf("error creating droplet: %v", err)
		}

		if droplet.Status == "active" {
			break
		}

		time.Sleep(time.Second * 10)
	}

	return droplet, nil
}
