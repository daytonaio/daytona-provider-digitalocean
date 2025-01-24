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
	"github.com/daytonaio/daytona/pkg/models"
	provider_util "github.com/daytonaio/daytona/pkg/provider/util"
	"github.com/daytonaio/daytona/pkg/ssh"
	"github.com/daytonaio/daytona/pkg/tailscale"
	"github.com/digitalocean/godo"

	"github.com/daytonaio/daytona/pkg/provider"
)

func (p *DigitalOceanProvider) CreateTarget(targetReq *provider.TargetRequest) (*provider_util.Empty, error) {
	logWriter, cleanupFunc := p.getTargetLogWriter(targetReq.Target.Id, targetReq.Target.Name)
	defer cleanupFunc()
	logWriter.Write([]byte("\033[?25h\n"))

	targetOptions, err := types.ParseTargetOptions(targetReq.Target.TargetConfig.Options)
	if err != nil {
		logWriter.Write([]byte("Error parsing target config options:" + err.Error()))
		return new(provider_util.Empty), err
	}

	client, err := p.getDoClient(targetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to get client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	_, err = p.createDroplet(client, targetReq.Target, targetOptions, logWriter)
	if err != nil {
		logWriter.Write([]byte("Failed to create droplet: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	dockerClient, err := p.getDockerClient(targetReq.Target.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	sshClient, err := p.getSshClient(targetReq.Target.Id)
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}
	defer sshClient.Close()

	targetDir := p.getTargetDir(targetReq.Target.Id)

	return new(provider_util.Empty), dockerClient.CreateTarget(targetReq.Target, targetDir, logWriter, sshClient)
}

func (p *DigitalOceanProvider) CreateWorkspace(workspaceReq *provider.WorkspaceRequest) (*provider_util.Empty, error) {
	logWriter, cleanupFunc := p.getWorkspaceLogWriter(workspaceReq.Workspace.Id, workspaceReq.Workspace.Name)
	defer cleanupFunc()

	dockerClient, err := p.getDockerClient(workspaceReq.Workspace.TargetId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	sshClient, err := tailscale.NewSshClient(p.tsnetConn, &ssh.SessionConfig{
		Hostname: workspaceReq.Workspace.TargetId,
		Port:     config.SSH_PORT,
	})
	if err != nil {
		logWriter.Write([]byte("Failed to create ssh client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}
	defer sshClient.Close()

	return new(provider_util.Empty), dockerClient.CreateWorkspace(&docker.CreateWorkspaceOptions{
		Workspace:           workspaceReq.Workspace,
		WorkspaceDir:        p.getWorkspaceDir(workspaceReq),
		ContainerRegistries: workspaceReq.ContainerRegistries,
		BuilderImage:        workspaceReq.BuilderImage,
		LogWriter:           logWriter,
		Gpc:                 workspaceReq.GitProviderConfig,
		SshClient:           sshClient,
	})
}

func (p *DigitalOceanProvider) createDroplet(client *godo.Client, tg *models.Target, targetOptions *types.TargetOptions, logWriter io.Writer) (*godo.Droplet, error) {
	dropletName := util.GetDropletName(tg)

	existingDroplet, err := util.GetDroplet(client, dropletName)
	if err == nil && existingDroplet != nil {
		return existingDroplet, nil
	}

	logWriter.Write([]byte("Creating droplet...\n"))

	tg.EnvVars["DAYTONA_AGENT_LOG_FILE_PATH"] = "/home/daytona/.daytona-agent.log"

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

# Check if docker is installed
if ! command -v docker &> /dev/null; then
  curl -fsSL https://get.docker.com | bash
fi

# Move docker data dir
service docker stop
cat > /etc/docker/daemon.json << EOF
{
  "data-root": "/home/daytona/.docker-daemon",
	"hosts": ["unix:///var/run/docker.sock", "tcp://0.0.0.0:2375"],
  "live-restore": true
}
EOF
# https://docs.docker.com/config/daemon/remote-access/#configuring-remote-access-with-systemd-unit-file
mkdir -p /etc/systemd/system/docker.service.d
cat > /etc/systemd/system/docker.service.d/options.conf << EOF
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd
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

	for k, v := range tg.EnvVars {
		userData += fmt.Sprintf("export %s=%s\n", k, v)
	}

	userData += fmt.Sprintf(`curl -sfL -H "Authorization: Bearer %s" %s | bash`, tg.ApiKey, *p.DaytonaDownloadUrl)
	userData += `
	echo '[Unit]
Description=Daytona Agent Service
After=network.target

[Service]
User=daytona
ExecStart=/usr/local/bin/daytona agent --target
Restart=always
`

	for k, v := range tg.EnvVars {
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

	logWriter.Write([]byte("Droplet created.\n"))

	initializingDropletSpinner := log_writers.ShowSpinner(logWriter, "Initializing droplet", "Droplet initialized")

	err = p.waitForDial(tg.Id, 10*time.Minute)
	close(initializingDropletSpinner)

	if err != nil {
		logWriter.Write([]byte("Failed to dial: " + err.Error() + "\n"))
		return nil, err
	}
	logWriter.Write([]byte("Workspace agent started.\n"))

	return droplet, nil
}
