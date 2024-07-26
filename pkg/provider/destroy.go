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

	"github.com/daytonaio/daytona/pkg/logs"
	"github.com/daytonaio/daytona/pkg/provider"
)

func (p *DigitalOceanProvider) DestroyWorkspace(workspaceReq *provider.WorkspaceRequest) (*provider_util.Empty, error) {
	logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	if p.LogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(*p.LogsDir)
		wsLogWriter := loggerFactory.CreateWorkspaceLogger(workspaceReq.Workspace.Id, logs.LogSourceProvider)
		logWriter = io.MultiWriter(&log_writers.InfoLogWriter{}, wsLogWriter)
		defer wsLogWriter.Close()
	}

	dockerClient, err := p.getDockerClient(workspaceReq.Workspace.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	sshClient, err := p.getSshClient(workspaceReq.Workspace.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get ssh client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}
	defer sshClient.Close()

	err = dockerClient.DestroyWorkspace(workspaceReq.Workspace, p.getWorkspaceDir(workspaceReq.Workspace.Id), sshClient)
	if err != nil {
		logWriter.Write([]byte("Failed to destroy workspace: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	targetOptions, err := types.ParseTargetOptions(workspaceReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Error parsing target options: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	client, err := p.getDoClient(targetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to get client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	droplet, err := util.GetDroplet(client, util.GetDropletName(workspaceReq.Workspace))
	if err != nil {
		logWriter.Write([]byte("Failed to get droplet ID: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	err = util.DeleteDroplet(client, droplet.ID)
	if err != nil {
		logWriter.Write([]byte("Failed to delete droplet: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	for {
		_, _, err := client.Droplets.Get(context.Background(), droplet.ID)
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				break
			} else {
				logWriter.Write([]byte("Failed to get droplet: " + err.Error() + "\n"))
				return new(provider_util.Empty), err
			}
		}

		time.Sleep(1 * time.Second)
	}

	err = util.DeleteVolume(client, util.GetDropletName(workspaceReq.Workspace))
	if err != nil {
		logWriter.Write([]byte("Failed to delete volume: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	return new(provider_util.Empty), nil
}

func (p *DigitalOceanProvider) DestroyProject(projectReq *provider.ProjectRequest) (*provider_util.Empty, error) {
	logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	if p.LogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(*p.LogsDir)
		projectLogWriter := loggerFactory.CreateProjectLogger(projectReq.Project.WorkspaceId, projectReq.Project.Name, logs.LogSourceProvider)
		logWriter = io.MultiWriter(&log_writers.InfoLogWriter{}, projectLogWriter)
		defer projectLogWriter.Close()
	}

	dockerClient, err := p.getDockerClient(projectReq.Project.WorkspaceId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	sshClient, err := p.getSshClient(projectReq.Project.WorkspaceId)
	if err != nil {
		logWriter.Write([]byte("Failed to get ssh client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}
	defer sshClient.Close()

	return new(provider_util.Empty), dockerClient.DestroyProject(projectReq.Project, p.getProjectDir(projectReq), sshClient)
}
