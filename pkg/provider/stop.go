package provider

import (
	"io"

	log_writers "github.com/daytonaio/daytona-provider-digitalocean/internal/log"
	"github.com/daytonaio/daytona-provider-digitalocean/pkg/provider/util"
	"github.com/daytonaio/daytona-provider-digitalocean/pkg/types"
	provider_util "github.com/daytonaio/daytona/pkg/provider/util"

	"github.com/daytonaio/daytona/pkg/logs"
	"github.com/daytonaio/daytona/pkg/provider"
)

func (p *DigitalOceanProvider) StopWorkspace(workspaceReq *provider.WorkspaceRequest) (*provider_util.Empty, error) {
	logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	if p.LogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(*p.LogsDir)
		wsLogWriter := loggerFactory.CreateWorkspaceLogger(workspaceReq.Workspace.Id, logs.LogSourceProvider)
		logWriter = io.MultiWriter(&log_writers.InfoLogWriter{}, wsLogWriter)
		defer wsLogWriter.Close()
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

	err = util.DeleteDroplet(client, workspaceReq.Workspace, false)
	if err != nil {
		logWriter.Write([]byte("Failed to delete droplet: " + err.Error() + "\n"))
		return nil, err
	}

	logWriter.Write([]byte("Droplet deleted.\n"))

	return new(provider_util.Empty), nil
}

func (p *DigitalOceanProvider) StopProject(projectReq *provider.ProjectRequest) (*provider_util.Empty, error) {
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
		return nil, err
	}

	return new(provider_util.Empty), dockerClient.StopProject(projectReq.Project, logWriter)
}
