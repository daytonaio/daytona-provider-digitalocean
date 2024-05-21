package provider

import (
	"fmt"
	"io"
	"time"

	log_writers "github.com/daytonaio/daytona-provider-digitalocean/internal/log"
	"github.com/daytonaio/daytona-provider-digitalocean/pkg/types"
	provider_util "github.com/daytonaio/daytona/pkg/provider/util"

	"github.com/daytonaio/daytona/pkg/logger"
	"github.com/daytonaio/daytona/pkg/provider"
)

func (p *DigitalOceanProvider) StartWorkspace(workspaceReq *provider.WorkspaceRequest) (*provider_util.Empty, error) {
	logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	if p.LogsDir != nil {
		loggerFactory := logger.NewLoggerFactory(*p.LogsDir)
		wsLogWriter := loggerFactory.CreateWorkspaceLogger(workspaceReq.Workspace.Id)
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

	_, err = p.createDroplet(client, workspaceReq.Workspace, targetOptions, logWriter)
	if err != nil {
		logWriter.Write([]byte("Failed to create droplet: " + err.Error() + "\n"))
		return nil, err
	}

	logWriter.Write([]byte("Droplet created.\n"))
	stopSpinnerChan := make(chan bool)

	go func() {
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		for i := 0; ; i++ {
			select {
			case <-stopSpinnerChan:
				return
			case <-time.After(200 * time.Millisecond):
				if i > 0 {
					logWriter.Write([]byte("\033[1F"))
				}
				logWriter.Write([]byte(fmt.Sprintf("%s Waiting for agent to start...\n", spinner[i%len(spinner)])))
			}
		}
	}()

	err = p.waitForDial(workspaceReq.Workspace.Id, 10*time.Minute)
	stopSpinnerChan <- true

	if err != nil {
		logWriter.Write([]byte("Failed to dial: " + err.Error() + "\n"))
		return nil, err
	}
	logWriter.Write([]byte("Workspace agent started.\n"))

	return new(provider_util.Empty), nil
}

func (p *DigitalOceanProvider) StartProject(projectReq *provider.ProjectRequest) (*provider_util.Empty, error) {
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

	err = dockerClient.StartProject(projectReq.Project)
	if err != nil {
		logWriter.Write([]byte("Failed to start project: " + err.Error() + "\n"))
		return nil, err
	}

	return new(provider_util.Empty), nil
}
