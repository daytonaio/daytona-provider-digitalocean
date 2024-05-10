package util

import (
	"context"
	"fmt"
	"time"

	provider_types "github.com/daytonaio/daytona-provider-digitalocean/pkg/types"
	"github.com/daytonaio/daytona/pkg/workspace"
	"github.com/digitalocean/godo"
	"github.com/pkg/errors"
)

func CreateDroplet(client *godo.Client, project *workspace.Project, targetOptions *provider_types.TargetOptions, serverDownloadUrl string) (*godo.Droplet, error) {
	fmt.Println("DiskSize:", targetOptions.DiskSize)

	// create volume
	volume, err := GetVolumeByName(client, project.Name)
	if err != nil {
		return nil, err
	} else if volume == nil {
		volume, _, err = client.Storage.CreateVolume(context.Background(), &godo.VolumeCreateRequest{
			Name:            GetDropletName(project),
			Region:          targetOptions.Region,
			SizeGigaBytes:   int64(targetOptions.DiskSize),
			FilesystemType:  "ext4",
			FilesystemLabel: "Daytona Data",
			Tags:            []string{"daytona"},
		})
		if err != nil {
			return nil, errors.Wrap(err, "create volume")
		}
	}

	// retrieve user data
	userData := `#!/bin/bash

umount /mnt/` + GetDropletName(project) + `

# Mount volume to home
mkdir -p /home/daytona
mount -o discard,defaults,noatime /dev/disk/by-id/scsi-0DO_Volume_` + GetDropletName(project) + ` /home/daytona

echo '/dev/disk/by-id/scsi-0DO_Volume_` + GetDropletName(project) + ` /home/daytona ext4 discard,defaults,noatime 0 0' | sudo tee -a /etc/fstab

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
	usermod -aG sudo daytona
elif grep -q wheel /etc/group; then
	usermod -aG wheel daytona
fi
echo "daytona ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/91-daytona
chown daytona:daytona /home/daytona
`

	for k, v := range project.EnvVars {
		userData += "echo 'export " + k + "=" + v + "' >> /etc/profile.d/daytona_env_vars.sh\n"
		userData += "export " + k + "=" + v + "\n"
	}

	userData += "echo 'export DAYTONA_WS_DIR=/home/daytona/" + project.Name + "' >> /etc/profile.d/daytona_env_vars.sh\n"
	userData += "export DAYTONA_WS_DIR=/home/daytona/" + project.Name + "\n"

	// TODO: "DAYTONA_WS_DIR=" + path.Join("/workspaces", project.Name),
	// userData += "su daytona\n"
	userData += fmt.Sprintf(`curl -sfL -H "Authorization: Bearer %s" %s | bash`, project.ApiKey, serverDownloadUrl)
	userData += `
	echo '[Unit]
Description=Daytona Agent Service
After=network.target

[Service]
User=daytona
ExecStart=/usr/local/bin/daytona agent
Restart=always
`

	for k, v := range project.EnvVars {
		userData += fmt.Sprintf("Environment='%s=%s'\n", k, v)
	}
	userData += "Environment='DAYTONA_WS_DIR=/home/daytona/" + project.Name + "'\n"

	userData += `
[Install]
WantedBy=multi-user.target' > /etc/systemd/system/daytona-agent.service

systemctl enable daytona-agent.service
systemctl start daytona-agent.service
`

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

		time.Sleep(time.Second * 10)
	}

	return droplet, nil
}
