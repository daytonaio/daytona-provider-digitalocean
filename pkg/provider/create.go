package provider

import (
	"context"
	"fmt"
	"io"
	"time"

	log_writers "github.com/daytonaio/daytona-provider-digitalocean/internal/log"
	"github.com/daytonaio/daytona-provider-digitalocean/pkg/provider/util"
	"github.com/daytonaio/daytona-provider-digitalocean/pkg/types"
	"github.com/daytonaio/daytona/pkg/agent/ssh/config"
	"github.com/daytonaio/daytona/pkg/docker"
	"github.com/daytonaio/daytona/pkg/logs"
	provider_util "github.com/daytonaio/daytona/pkg/provider/util"
	"github.com/daytonaio/daytona/pkg/ssh"
	"github.com/daytonaio/daytona/pkg/tailscale"
	"github.com/daytonaio/daytona/pkg/workspace"
	"github.com/digitalocean/godo"

	"github.com/daytonaio/daytona/pkg/provider"
)

func (p *DigitalOceanProvider) CreateWorkspace(workspaceReq *provider.WorkspaceRequest) (*provider_util.Empty, error) {
	logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	if p.LogsDir != nil {
		loggerFactory := logs.NewLoggerFactory(*p.LogsDir)
		wsLogWriter := loggerFactory.CreateWorkspaceLogger(workspaceReq.Workspace.Id, logs.LogSourceProvider)
		logWriter = io.MultiWriter(&log_writers.InfoLogWriter{}, wsLogWriter)
		defer wsLogWriter.Close()
	}

	targetOptions, err := types.ParseTargetOptions(workspaceReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Error parsing target options:" + err.Error()))
		return new(provider_util.Empty), err
	}

	client, err := p.getDoClient(targetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to get client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	_, err = p.createDroplet(client, workspaceReq.Workspace, targetOptions, logWriter)
	if err != nil {
		logWriter.Write([]byte("Failed to create droplet: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
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
				logWriter.Write([]byte(fmt.Sprintf("%s Waiting for the agent to start...\n", spinner[i%len(spinner)])))
			}
		}
	}()

	err = p.waitForDial(workspaceReq.Workspace.Id, 10*time.Minute)
	stopSpinnerChan <- true

	if err != nil {
		logWriter.Write([]byte("Failed to dial: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}
	logWriter.Write([]byte("Workspace agent started.\n"))

	dockerClient, err := p.getDockerClient(workspaceReq.Workspace.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	sshClient, err := p.getSshClient(workspaceReq.Workspace.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}
	defer sshClient.Close()

	workspaceDir := p.getWorkspaceDir(workspaceReq.Workspace.Id)

	return new(provider_util.Empty), dockerClient.CreateWorkspace(workspaceReq.Workspace, workspaceDir, logWriter, sshClient)
}

func (p *DigitalOceanProvider) CreateProject(projectReq *provider.ProjectRequest) (*provider_util.Empty, error) {
	logWriter, cleanupFunc := p.getProjectLogWriter(projectReq.Project.WorkspaceId, projectReq.Project.Name)
	defer cleanupFunc()

	dockerClient, err := p.getDockerClient(projectReq.Project.WorkspaceId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	sshClient, err := tailscale.NewSshClient(p.tsnetConn, &ssh.SessionConfig{
		Hostname: projectReq.Project.WorkspaceId,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}
	defer sshClient.Close()

	return new(provider_util.Empty), dockerClient.CreateProject(&docker.CreateProjectOptions{
		Project:    projectReq.Project,
		ProjectDir: p.getProjectDir(projectReq),
		Cr:         projectReq.ContainerRegistry,
		LogWriter:  logWriter,
		Gpc:        projectReq.GitProviderConfig,
		SshClient:  sshClient,
	})
}

func (p *DigitalOceanProvider) createDroplet(client *godo.Client, ws *workspace.Workspace, targetOptions *types.TargetOptions, logWriter io.Writer) (*godo.Droplet, error) {
	dropletName := util.GetDropletName(ws)

	droplets, _, err := client.Droplets.List(context.Background(), nil)
	if err != nil {
		return nil, fmt.Errorf("error listing droplets: %v", err)
	}
	for _, d := range droplets {
		if d.Name == dropletName {
			return &d, nil
		}
	}

	logWriter.Write([]byte("Creating droplet...\n"))

	ws.EnvVars["DAYTONA_AGENT_LOG_FILE_PATH"] = "/home/daytona/.daytona-agent.log"

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
# https://docs.docker.com/config/daemon/remote-access/#configuring-remote-access-with-systemd-unit-file
mkdir -p /etc/systemd/system/docker.service.d
cat > /etc/systemd/system/docker.service.d/options.conf << EOF
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd -H fd:// -H tcp://127.0.0.1:2375
EOF
systemctl daemon-reload

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

	for k, v := range ws.EnvVars {
		userData += fmt.Sprintf("export %s=%s\n", k, v)
	}

	userData += fmt.Sprintf(`curl -sfL -H "Authorization: Bearer %s" %s | bash`, ws.ApiKey, *p.DaytonaDownloadUrl)
	userData += `
	echo '[Unit]
Description=Daytona Agent Service
After=network.target

[Service]
User=daytona
ExecStart=/usr/local/bin/daytona agent --host
Restart=always
`

	for k, v := range ws.EnvVars {
		userData += fmt.Sprintf("Environment='%s=%s'\n", k, v)
	}

	userData += `
[Install]
WantedBy=multi-user.target' > /etc/systemd/system/daytona-agent.service

systemctl enable daytona-agent.service
systemctl start daytona-agent.service
`

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
