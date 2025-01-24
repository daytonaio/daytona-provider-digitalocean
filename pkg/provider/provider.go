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
	logwriters "github.com/daytonaio/daytona-provider-digitalocean/internal/log"
	"github.com/daytonaio/daytona-provider-digitalocean/pkg/types"
	"github.com/daytonaio/daytona/pkg/agent/ssh/config"
	"github.com/daytonaio/daytona/pkg/docker"
	"github.com/daytonaio/daytona/pkg/logs"
	"github.com/daytonaio/daytona/pkg/models"
	provider_util "github.com/daytonaio/daytona/pkg/provider/util"
	"github.com/daytonaio/daytona/pkg/ssh"
	"github.com/daytonaio/daytona/pkg/tailscale"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"tailscale.com/tsnet"

	"github.com/daytonaio/daytona/pkg/provider"
	"github.com/digitalocean/godo"
)

type DigitalOceanProvider struct {
	BasePath           *string
	DaytonaDownloadUrl *string
	DaytonaVersion     *string
	ServerUrl          *string
	ApiUrl             *string
	ApiKey             *string
	ApiPort            *uint32
	ServerPort         *uint32
	WorkspaceLogsDir   *string
	TargetLogsDir      *string
	NetworkKey         *string

	tsnetConn *tsnet.Server
}

func (p *DigitalOceanProvider) Initialize(req provider.InitializeProviderRequest) (*provider_util.Empty, error) {
	p.BasePath = &req.BasePath
	p.DaytonaDownloadUrl = &req.DaytonaDownloadUrl
	p.DaytonaVersion = &req.DaytonaVersion
	p.ServerUrl = &req.ServerUrl
	p.ApiUrl = &req.ApiUrl
	p.ApiKey = req.ApiKey
	p.ApiPort = &req.ApiPort
	p.ServerPort = &req.ServerPort
	p.WorkspaceLogsDir = &req.WorkspaceLogsDir
	p.TargetLogsDir = &req.TargetLogsDir
	p.NetworkKey = &req.NetworkKey

	return new(provider_util.Empty), nil
}

func (p *DigitalOceanProvider) GetInfo() (models.ProviderInfo, error) {
	label := "DigitalOcean"
	return models.ProviderInfo{
		Name:                 "digitalocean-provider",
		Label:                &label,
		Version:              internal.Version,
		TargetConfigManifest: *types.GetTargetConfigManifest(),
	}, nil
}

func (p *DigitalOceanProvider) GetPresetTargetConfigs() (*[]provider.TargetConfig, error) {
	return new([]provider.TargetConfig), nil
}

func (p *DigitalOceanProvider) GetTargetProviderMetadata(targetReq *provider.TargetRequest) (string, error) {
	logWriter, cleanupFunc := p.getTargetLogWriter(targetReq.Target.Id, targetReq.Target.Name)
	defer cleanupFunc()

	dockerClient, err := p.getDockerClient(targetReq.Target.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return "", err
	}

	return dockerClient.GetTargetProviderMetadata(targetReq.Target)
}

func (p *DigitalOceanProvider) GetWorkspaceProviderMetadata(workspaceReq *provider.WorkspaceRequest) (string, error) {
	logWriter, cleanupFunc := p.getWorkspaceLogWriter(workspaceReq.Workspace.Id, workspaceReq.Workspace.Name)
	defer cleanupFunc()

	dockerClient, err := p.getDockerClient(workspaceReq.Workspace.TargetId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return "", err
	}

	return dockerClient.GetWorkspaceProviderMetadata(workspaceReq.Workspace)
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

func (p *DigitalOceanProvider) getDockerClient(targetId string) (docker.IDockerClient, error) {
	tsnetConn, err := p.getTsnetConn()
	if err != nil {
		return nil, err
	}

	cli, err := client.NewClientWithOpts(client.WithDialContext(tsnetConn.Dial), client.WithHost(fmt.Sprintf("http://%s:2375", targetId)), client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	err = p.waitForDial(targetId, 15*time.Second)
	if err != nil {
		return nil, err
	}

	return docker.NewDockerClient(docker.DockerClientConfig{
		ApiClient: cli,
	}), nil
}

func (p *DigitalOceanProvider) waitForDial(targetId string, dialTimeout time.Duration) error {
	tsnetConn, err := p.getTsnetConn()
	if err != nil {
		return err
	}

	dialStartTime := time.Now()
	for {
		if time.Since(dialStartTime) > dialTimeout {
			return fmt.Errorf("timeout: dialing timed out after %f minutes", dialTimeout.Minutes())
		}

		dialConn, err := tsnetConn.Dial(context.Background(), "tcp", fmt.Sprintf("%s:%d", targetId, config.SSH_PORT))
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

func (p *DigitalOceanProvider) getWorkspaceDir(workspaceReq *provider.WorkspaceRequest) string {
	return path.Join(
		p.getTargetDir(workspaceReq.Workspace.TargetId),
		workspaceReq.Workspace.Id,
		workspaceReq.Workspace.WorkspaceFolderName(),
	)
}

func (a *DigitalOceanProvider) CheckRequirements() (*[]provider.RequirementStatus, error) {
	results := []provider.RequirementStatus{}
	return &results, nil
}

func (p *DigitalOceanProvider) getTargetDir(targetId string) string {
	return fmt.Sprintf("/home/daytona/.workspace-data/%s", targetId)
}

func (p *DigitalOceanProvider) getWorkspaceLogWriter(workspaceId, workspaceName string) (io.Writer, func()) {
	logWriter := io.MultiWriter(&logwriters.InfoLogWriter{})
	cleanupFunc := func() {}

	if p.WorkspaceLogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(logs.LoggerFactoryConfig{
			LogsDir:     *p.WorkspaceLogsDir,
			ApiUrl:      p.ApiUrl,
			ApiKey:      p.ApiKey,
			ApiBasePath: &logs.ApiBasePathWorkspace,
		})
		workspaceLogWriter, err := loggerFactory.CreateLogger(workspaceId, workspaceName, logs.LogSourceProvider)
		if err == nil {
			logWriter = io.MultiWriter(&logwriters.InfoLogWriter{}, workspaceLogWriter)
			cleanupFunc = func() { workspaceLogWriter.Close() }
		}
	}

	return logWriter, cleanupFunc
}

func (p *DigitalOceanProvider) getTargetLogWriter(targetId, targetName string) (io.Writer, func()) {
	logWriter := io.MultiWriter(&logwriters.InfoLogWriter{})
	cleanupFunc := func() {}

	if p.TargetLogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(logs.LoggerFactoryConfig{
			LogsDir:     *p.TargetLogsDir,
			ApiUrl:      p.ApiUrl,
			ApiKey:      p.ApiKey,
			ApiBasePath: &logs.ApiBasePathTarget,
		})
		workspaceLogWriter, err := loggerFactory.CreateLogger(targetId, targetName, logs.LogSourceProvider)
		if err == nil {
			logWriter = io.MultiWriter(&logwriters.InfoLogWriter{}, workspaceLogWriter)
			cleanupFunc = func() { workspaceLogWriter.Close() }
		}
	}

	return logWriter, cleanupFunc
}
