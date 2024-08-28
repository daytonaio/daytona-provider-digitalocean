package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	internal "github.com/daytonaio/daytona-provider-digitalocean/internal"
	log_writers "github.com/daytonaio/daytona-provider-digitalocean/internal/log"
	"github.com/daytonaio/daytona-provider-digitalocean/pkg/types"
	"github.com/daytonaio/daytona/pkg/agent/ssh/config"
	"github.com/daytonaio/daytona/pkg/docker"
	provider_util "github.com/daytonaio/daytona/pkg/provider/util"
	"github.com/daytonaio/daytona/pkg/ssh"
	"github.com/daytonaio/daytona/pkg/tailscale"
	"github.com/daytonaio/daytona/pkg/workspace/project"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"tailscale.com/tsnet"

	"github.com/daytonaio/daytona/pkg/logs"
	"github.com/daytonaio/daytona/pkg/provider"
	"github.com/daytonaio/daytona/pkg/workspace"
	"github.com/digitalocean/godo"
)

type DigitalOceanProvider struct {
	BasePath           *string
	DaytonaDownloadUrl *string
	DaytonaVersion     *string
	ServerUrl          *string
	ApiUrl             *string
	LogsDir            *string
	ApiPort            *uint32
	ServerPort         *uint32
	NetworkKey         *string

	tsnetConn *tsnet.Server
}

func (p *DigitalOceanProvider) Initialize(req provider.InitializeProviderRequest) (*provider_util.Empty, error) {
	p.BasePath = &req.BasePath
	p.DaytonaDownloadUrl = &req.DaytonaDownloadUrl
	p.DaytonaVersion = &req.DaytonaVersion
	p.ServerUrl = &req.ServerUrl
	p.ApiUrl = &req.ApiUrl
	p.LogsDir = &req.LogsDir
	p.ApiPort = &req.ApiPort
	p.ServerPort = &req.ServerPort
	p.NetworkKey = &req.NetworkKey

	return new(provider_util.Empty), nil
}

func (p *DigitalOceanProvider) GetInfo() (provider.ProviderInfo, error) {
	return provider.ProviderInfo{
		Name:    "digitalocean-provider",
		Version: internal.Version,
	}, nil
}

func (p *DigitalOceanProvider) GetTargetManifest() (*provider.ProviderTargetManifest, error) {
	return types.GetTargetManifest(), nil
}

func (p *DigitalOceanProvider) GetDefaultTargets() (*[]provider.ProviderTarget, error) {
	return &[]provider.ProviderTarget{}, nil
}

func (p *DigitalOceanProvider) GetWorkspaceInfo(workspaceReq *provider.WorkspaceRequest) (*workspace.WorkspaceInfo, error) {
	providerMetadata, err := p.getWorkspaceMetadata(workspaceReq)
	if err != nil {
		return nil, err
	}

	workspaceInfo := &workspace.WorkspaceInfo{
		Name:             workspaceReq.Workspace.Name,
		ProviderMetadata: providerMetadata,
	}

	projectInfos := []*project.ProjectInfo{}
	for _, project := range workspaceReq.Workspace.Projects {
		projectInfo, err := p.GetProjectInfo(&provider.ProjectRequest{
			TargetOptions: workspaceReq.TargetOptions,
			Project:       project,
		})
		if err != nil {
			return nil, err
		}
		projectInfos = append(projectInfos, projectInfo)
	}
	workspaceInfo.Projects = projectInfos

	return workspaceInfo, nil
}

func (p *DigitalOceanProvider) GetProjectInfo(projectReq *provider.ProjectRequest) (*project.ProjectInfo, error) {
	logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	if p.LogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(p.LogsDir, nil)
		projectLogWriter := loggerFactory.CreateProjectLogger(projectReq.Project.WorkspaceId, projectReq.Project.Name, logs.LogSourceProvider)
		logWriter = io.MultiWriter(&log_writers.InfoLogWriter{}, projectLogWriter)
		defer projectLogWriter.Close()
	}

	dockerClient, err := p.getDockerClient(projectReq.Project.WorkspaceId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	return dockerClient.GetProjectInfo(projectReq.Project)
}

func (p *DigitalOceanProvider) getWorkspaceMetadata(workspaceReq *provider.WorkspaceRequest) (string, error) {
	return string(""), nil
}

func (p *DigitalOceanProvider) getDoClient(targetOptions *types.TargetOptions) (*godo.Client, error) {
	doToken := targetOptions.AuthToken

	if doToken == nil {
		envToken := os.Getenv("DIGITALOCEAN_ACCESS_TOKEN")
		// Get the DigitalOcean token from the environment variable
		if envToken == "" {
			return nil, errors.New("DigitalOcean Token not found")
		}

		doToken = &envToken
	}

	// Create a new DigitalOcean client
	oauthClient := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *doToken}))
	client := godo.NewClient(oauthClient)

	return client, nil
}

func (p *DigitalOceanProvider) getTsnetConn() (*tsnet.Server, error) {
	if p.tsnetConn == nil {
		tsnetConn, err := tailscale.GetConnection(&tailscale.TsnetConnConfig{
			AuthKey:    *p.NetworkKey,
			ControlURL: *p.ServerUrl,
			Dir:        filepath.Join(*p.BasePath, "tsnet", uuid.NewString()),
			Logf:       func(format string, args ...any) {},
			Hostname:   fmt.Sprintf("digitalocean-provider-%s", uuid.NewString()),
		})
		if err != nil {
			return nil, err
		}
		p.tsnetConn = tsnetConn
	}

	return p.tsnetConn, nil
}

func (p *DigitalOceanProvider) getDockerClient(workspaceId string) (docker.IDockerClient, error) {
	tsnetConn, err := p.getTsnetConn()
	if err != nil {
		return nil, err
	}

	cli, err := client.NewClientWithOpts(client.WithDialContext(tsnetConn.Dial), client.WithHost(fmt.Sprintf("http://%s:2375", workspaceId)), client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return docker.NewDockerClient(docker.DockerClientConfig{
		ApiClient: cli,
	}), nil
}

func (p *DigitalOceanProvider) waitForDial(workspaceId string, dialTimeout time.Duration) error {
	tsnetConn, err := p.getTsnetConn()
	if err != nil {
		return err
	}

	dialStartTime := time.Now()
	for {
		if time.Since(dialStartTime) > dialTimeout {
			return errors.New("timeout: dialing timed out after 3 minutes")
		}

		dialConn, err := tsnetConn.Dial(context.Background(), "tcp", fmt.Sprintf("%s:%d", workspaceId, config.SSH_PORT))
		if err == nil {
			defer dialConn.Close()
			break
		}

		time.Sleep(time.Second)
	}
	return nil
}

func (p *DigitalOceanProvider) getSshClient(workspaceId string) (*ssh.Client, error) {
	tsnetConn, err := p.getTsnetConn()
	if err != nil {
		return nil, err
	}

	return tailscale.NewSshClient(tsnetConn, &ssh.SessionConfig{
		Hostname: workspaceId,
		Port:     config.SSH_PORT,
	})
}

func (p *DigitalOceanProvider) getProjectDir(projectReq *provider.ProjectRequest) string {
	return path.Join(
		p.getWorkspaceDir(projectReq.Project.WorkspaceId),
		fmt.Sprintf("%s-%s", projectReq.Project.WorkspaceId, projectReq.Project.Name),
	)
}

func (p *DigitalOceanProvider) getWorkspaceDir(workspaceId string) string {
	return fmt.Sprintf("/tmp/%s", workspaceId)
}

func (p *DigitalOceanProvider) getProjectLogWriter(workspaceId string, projectName string) (io.Writer, func()) {
	logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	cleanupFunc := func() {}

	if p.LogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(p.LogsDir, nil)
		projectLogWriter := loggerFactory.CreateProjectLogger(workspaceId, projectName, logs.LogSourceProvider)
		logWriter = io.MultiWriter(&log_writers.InfoLogWriter{}, projectLogWriter)
		cleanupFunc = func() { projectLogWriter.Close() }
	}

	return logWriter, cleanupFunc
}
