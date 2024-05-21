package provider

import (
	"context"
	"io"
	"strings"
	"time"

	log_writers "github.com/daytonaio/daytona-provider-digitalocean/internal/log"
	"github.com/daytonaio/daytona-provider-digitalocean/pkg/provider/util"
	"github.com/daytonaio/daytona-provider-digitalocean/pkg/types"
	provider_util "github.com/daytonaio/daytona/pkg/provider/util"

	"github.com/daytonaio/daytona/pkg/logger"
	"github.com/daytonaio/daytona/pkg/provider"
)

func (p *DigitalOceanProvider) DestroyWorkspace(workspaceReq *provider.WorkspaceRequest) (*provider_util.Empty, error) {
	logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	if p.LogsDir != nil {
		loggerFactory := logger.NewLoggerFactory(*p.LogsDir)
		wsLogWriter := loggerFactory.CreateWorkspaceLogger(workspaceReq.Workspace.Id)
		logWriter = io.MultiWriter(&log_writers.InfoLogWriter{}, wsLogWriter)
		defer wsLogWriter.Close()
	}

	dockerClient, err := p.getDockerClient(workspaceReq.Workspace.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	err = dockerClient.DestroyWorkspace(workspaceReq.Workspace)
	if err != nil {
		logWriter.Write([]byte("Failed to destroy workspace: " + err.Error() + "\n"))
		return nil, err
	}

	targetOptions, err := types.ParseTargetOptions(workspaceReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Error parsing target options: " + err.Error() + "\n"))
		return nil, err
	}

	client, err := p.getDoClient(targetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to get client: " + err.Error() + "\n"))
		return nil, err
	}

	droplet, err := util.GetDroplet(client, util.GetDropletName(workspaceReq.Workspace))
	if err != nil {
		logWriter.Write([]byte("Failed to get droplet ID: " + err.Error() + "\n"))
		return nil, err
	}

	err = util.DeleteDroplet(client, droplet.ID)
	if err != nil {
		logWriter.Write([]byte("Failed to delete droplet: " + err.Error() + "\n"))
		return nil, err
	}

	for {
		_, _, err := client.Droplets.Get(context.Background(), droplet.ID)
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				break
			} else {
				logWriter.Write([]byte("Failed to get droplet: " + err.Error() + "\n"))
				return nil, err
			}
		}

		time.Sleep(1 * time.Second)
	}

	err = util.DeleteVolume(client, util.GetDropletName(workspaceReq.Workspace))
	if err != nil {
		logWriter.Write([]byte("Failed to delete volume: " + err.Error() + "\n"))
		return nil, err
	}

	return new(provider_util.Empty), nil
}

func (p *DigitalOceanProvider) DestroyProject(projectReq *provider.ProjectRequest) (*provider_util.Empty, error) {
	logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	if p.LogsDir != nil {
		loggerFactory := logger.NewLoggerFactory(*p.LogsDir)
		projectLogWriter := loggerFactory.CreateProjectLogger(projectReq.Project.WorkspaceId, projectReq.Project.Name)
		logWriter = io.MultiWriter(&log_writers.InfoLogWriter{}, projectLogWriter)
		defer projectLogWriter.Close()
	}

	dockerClient, err := p.getDockerClient(projectReq.Project.WorkspaceId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	err = dockerClient.DestroyProject(projectReq.Project)
	if err != nil {
		logWriter.Write([]byte("Failed to destroy project: " + err.Error() + "\n"))
		return nil, err
	}

	return new(provider_util.Empty), nil
}
