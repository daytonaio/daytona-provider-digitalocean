package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	internal "github.com/daytonaio/daytona-provider-digitalocean/internal"
	log_writers "github.com/daytonaio/daytona-provider-digitalocean/internal/log"
	"github.com/daytonaio/daytona-provider-digitalocean/pkg/provider/util"
	"github.com/daytonaio/daytona-provider-digitalocean/pkg/types"
	"github.com/daytonaio/daytona/pkg/agent/ssh/config"
	"github.com/daytonaio/daytona/pkg/docker"
	provider_util "github.com/daytonaio/daytona/pkg/provider/util"
	"github.com/daytonaio/daytona/pkg/tailscale"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"tailscale.com/tsnet"

	"github.com/daytonaio/daytona/pkg/logger"
	"github.com/daytonaio/daytona/pkg/provider"
	"github.com/daytonaio/daytona/pkg/workspace"
	"github.com/digitalocean/godo"

	log "github.com/sirupsen/logrus"
)

type DigitalOceanProvider struct {
	BasePath          *string
	ServerDownloadUrl *string
	ServerVersion     *string
	ServerUrl         *string
	ServerApiUrl      *string
	LogsDir           *string
	NetworkKey        *string
	OwnProperty       string

	tsnetConn *tsnet.Server
}

func (p *DigitalOceanProvider) Initialize(req provider.InitializeProviderRequest) (*provider_util.Empty, error) {
	p.OwnProperty = "my-own-property"

	p.BasePath = &req.BasePath
	p.ServerDownloadUrl = &req.ServerDownloadUrl
	p.ServerVersion = &req.ServerVersion
	p.ServerUrl = &req.ServerUrl
	p.ServerApiUrl = &req.ServerApiUrl
	p.LogsDir = &req.LogsDir
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
	info, err := p.GetInfo()
	if err != nil {
		return nil, err
	}

	defaultTargets := []provider.ProviderTarget{
		{
			Name:         "default-target",
			ProviderInfo: info,
			Options:      "{\n\t\"Required String\": \"default-required-string\"\n}",
		},
	}
	return &defaultTargets, nil
}

func (p *DigitalOceanProvider) CreateWorkspace(workspaceReq *provider.WorkspaceRequest) (*provider_util.Empty, error) {
	logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	if p.LogsDir != nil {
		loggerFactory := logger.NewLoggerFactory(*p.LogsDir)
		wsLogWriter := loggerFactory.CreateWorkspaceLogger(workspaceReq.Workspace.Id)
		logWriter = io.MultiWriter(&log_writers.InfoLogWriter{}, wsLogWriter)
		defer wsLogWriter.Close()
	}

	targetOptions, err := types.ParseTargetOptions(workspaceReq.TargetOptions)
	if err != nil {
		log.Fatalf("Error parsing target options: %v", err)
	}

	client, err := p.getDoClient(targetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to get client: " + err.Error() + "\n"))
		return nil, err
	}

	_, err = p.createDroplet(client, workspaceReq.Workspace, targetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to create droplet: " + err.Error() + "\n"))
		return nil, err
	}

	logWriter.Write([]byte("Droplet created.\nWaiting for the workspace agent to start...\n"))
	err = p.waitForDial(workspaceReq.Workspace.Id, 10*time.Minute)
	if err != nil {
		logWriter.Write([]byte("Failed to dial: " + err.Error() + "\n"))
		return nil, err
	}
	logWriter.Write([]byte("Workspace agent started.n"))

	dockerClient, err := p.getDockerClient(workspaceReq.Workspace.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return nil, err
	}

	err = dockerClient.CreateWorkspace(workspaceReq.Workspace, logWriter)
	if err != nil {
		logWriter.Write([]byte("Failed to destroy project: " + err.Error() + "\n"))
		return nil, err
	}

	return new(provider_util.Empty), nil
}

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

	droplet, err := util.GetDroplet(client, util.GetDropletName(workspaceReq.Workspace))
	if err != nil {
		logWriter.Write([]byte("Failed to get droplet ID: " + err.Error() + "\n"))
		return nil, err
	}

	err = util.PowerOnDroplet(client, droplet.ID)
	if err != nil {
		logWriter.Write([]byte("Failed to delete droplet: " + err.Error() + "\n"))
		return nil, err
	}

	logWriter.Write([]byte("Droplet powered on.\n"))

	return new(provider_util.Empty), nil
}

func (p *DigitalOceanProvider) StopWorkspace(workspaceReq *provider.WorkspaceRequest) (*provider_util.Empty, error) {
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

	droplet, err := util.GetDroplet(client, util.GetDropletName(workspaceReq.Workspace))
	if err != nil {
		logWriter.Write([]byte("Failed to get droplet ID: " + err.Error() + "\n"))
		return nil, err
	}

	err = util.PowerOffDroplet(client, droplet.ID)
	if err != nil {
		logWriter.Write([]byte("Failed to delete droplet: " + err.Error() + "\n"))
		return nil, err
	}

	logWriter.Write([]byte("Droplet powered off.\n"))

	return new(provider_util.Empty), nil
}

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

func (p *DigitalOceanProvider) GetWorkspaceInfo(workspaceReq *provider.WorkspaceRequest) (*workspace.WorkspaceInfo, error) {
	providerMetadata, err := p.getWorkspaceMetadata(workspaceReq)
	if err != nil {
		return nil, err
	}

	workspaceInfo := &workspace.WorkspaceInfo{
		Name:             workspaceReq.Workspace.Name,
		ProviderMetadata: providerMetadata,
	}

	projectInfos := []*workspace.ProjectInfo{}
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

func (p *DigitalOceanProvider) CreateProject(projectReq *provider.ProjectRequest) (*provider_util.Empty, error) {
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

	err = dockerClient.CreateProject(projectReq.Project, *p.ServerDownloadUrl, projectReq.ContainerRegistry, logWriter)
	if err != nil {
		logWriter.Write([]byte("Failed to create project: " + err.Error() + "\n"))
		return nil, err
	}

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

func (p *DigitalOceanProvider) StopProject(projectReq *provider.ProjectRequest) (*provider_util.Empty, error) {
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

	err = dockerClient.StopProject(projectReq.Project)
	if err != nil {
		logWriter.Write([]byte("Failed to stop project: " + err.Error() + "\n"))
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

func (p *DigitalOceanProvider) GetProjectInfo(projectReq *provider.ProjectRequest) (*workspace.ProjectInfo, error) {
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

	return dockerClient.GetProjectInfo(projectReq.Project)
}

func (p *DigitalOceanProvider) getWorkspaceMetadata(workspaceReq *provider.WorkspaceRequest) (string, error) {
	metadata := types.WorkspaceMetadata{
		Property: workspaceReq.Workspace.Id,
	}

	jsonContent, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}

	return string(jsonContent), nil
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

	localSockPath := filepath.Join(*p.BasePath, workspaceId, "docker-forward.sock")

	if _, err := os.Stat(filepath.Dir(localSockPath)); err != nil {
		err := os.MkdirAll(filepath.Dir(localSockPath), 0755)
		if err != nil {
			return nil, err
		}

		startedChan, errChan := tailscale.ForwardRemoteUnixSock(tailscale.ForwardConfig{
			Ctx:        context.Background(),
			TsnetConn:  tsnetConn,
			Hostname:   workspaceId,
			SshPort:    config.SSH_PORT,
			LocalSock:  localSockPath,
			RemoteSock: "/var/run/docker.sock",
		})

		go func() {
			err := <-errChan
			if err != nil {
				log.Error(err)
				startedChan <- false
				os.Remove(localSockPath)
			}
		}()

		started := <-startedChan
		if !started {
			return nil, errors.New("failed to start SSH tunnel")
		}
	}

	cli, err := client.NewClientWithOpts(client.WithHost(fmt.Sprintf("unix://%s", localSockPath)), client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return docker.NewDockerClient(docker.DockerClientConfig{
		ApiClient: cli,
	}), nil
}

func (p *DigitalOceanProvider) createDroplet(client *godo.Client, ws *workspace.Workspace, targetOptions *types.TargetOptions) (*godo.Droplet, error) {
	wsEnv := workspace.GetWorkspaceEnvVars(ws, workspace.WorkspaceEnvVarParams{
		ApiUrl:        *p.ServerApiUrl,
		ApiKey:        ws.ApiKey,
		ServerUrl:     *p.ServerUrl,
		ServerVersion: *p.ServerVersion,
	})

	wsEnv["DAYTONA_AGENT_LOG_FILE_PATH"] = "/home/daytona/.daytona-agent.log"

	dropletName := util.GetDropletName(ws)

	volume, err := util.GetVolumeByName(client, dropletName)
	if err != nil {
		return nil, err
	} else if volume == nil {
		volume, _, err = client.Storage.CreateVolume(context.Background(), &godo.VolumeCreateRequest{
			Name:            dropletName,
			Region:          targetOptions.Region,
			SizeGigaBytes:   int64(targetOptions.DiskSize),
			FilesystemType:  "ext4",
			FilesystemLabel: "Daytona Data",
			Tags:            []string{"daytona"},
		})
		if err != nil {
			return nil, err
		}
	}

	// retrieve user data
	userData := `#!/bin/bash

umount /mnt/` + dropletName + `

# Mount volume to home
mkdir -p /home/daytona
mount -o discard,defaults,noatime /dev/disk/by-id/scsi-0DO_Volume_` + dropletName + ` /home/daytona

echo '/dev/disk/by-id/scsi-0DO_Volume_` + dropletName + ` /home/daytona ext4 discard,defaults,noatime 0 0' | sudo tee -a /etc/fstab

curl -fsSL https://get.docker.com | bash

# Move docker data dir
service docker stop
cat > /etc/docker/daemon.json << EOF
{
  "data-root": "/home/daytona/.docker-daemon",
  "live-restore": true
}
EOF
# Make sure we only copy if volumes isn't initialized
if [ ! -d "/home/daytona/.docker-daemon" ]; then
  mkdir -p /home/daytona/.docker-daemon
  rsync -aP /var/lib/docker/ /home/daytona/.docker-daemon
fi
service docker start

# Create Daytona user
useradd daytona -d /home/daytona -s /bin/bash
if grep -q sudo /etc/group; then
	usermod -aG sudo,docker daytona
elif grep -q wheel /etc/group; then
	usermod -aG wheel,docker daytona
fi
echo "daytona ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/91-daytona
chown daytona:daytona /home/daytona
`

	for k, v := range wsEnv {
		userData += fmt.Sprintf("export %s=%s\n", k, v)
	}

	userData += fmt.Sprintf(`curl -sfL -H "Authorization: Bearer %s" %s | bash`, ws.ApiKey, *p.ServerDownloadUrl)
	userData += `
	echo '[Unit]
Description=Daytona Agent Service
After=network.target

[Service]
User=daytona
ExecStart=/usr/local/bin/daytona agent --host
Restart=always
`

	for k, v := range wsEnv {
		userData += fmt.Sprintf("Environment='%s=%s'\n", k, v)
	}

	userData += `
[Install]
WantedBy=multi-user.target' > /etc/systemd/system/daytona-agent.service

systemctl enable daytona-agent.service
systemctl start daytona-agent.service
`

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
		Name:   dropletName,
		Region: targetOptions.Region,
		Size:   targetOptions.Size,
		Image: godo.DropletCreateImage{
			Slug: targetOptions.Image,
		},
		UserData: userData,
		Tags:     []string{"daytona"},
		Volumes:  []godo.DropletCreateVolume{{ID: volume.ID}},
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

		time.Sleep(time.Second * 2)
	}

	return droplet, nil
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
