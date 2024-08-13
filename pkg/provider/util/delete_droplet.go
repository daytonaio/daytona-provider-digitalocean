package util

import (
	"context"
	"strings"
	"time"

	"github.com/daytonaio/daytona/pkg/workspace"
	"github.com/digitalocean/godo"
)

func DeleteDroplet(client *godo.Client, workspace *workspace.Workspace, deleteVolume bool) error {
	if deleteVolume {
		err := DeleteVolume(client, GetDropletName(workspace))
		if err != nil {
			return err
		}
	}

	droplet, err := GetDroplet(client, GetDropletName(workspace))
	if err != nil {
		return err
	}

	_, err = client.Droplets.Delete(context.Background(), droplet.ID)
	if err != nil {
		return err
	}

	for {
		_, _, err := client.Droplets.Get(context.Background(), droplet.ID)
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				break
			} else {
				return err
			}
		}

		time.Sleep(1 * time.Second)
	}

	return nil
}

func DeleteVolume(client *godo.Client, name string) error {
	volume, err := GetVolumeByName(client, name)
	if err != nil {
		return err
	}

	if volume == nil {
		return nil
	}

	ctx := context.Background()

	for _, dropletID := range volume.DropletIDs {
		_, _, err = client.StorageActions.DetachByDropletID(ctx, volume.ID, dropletID)
		if err != nil {
			return err
		}
	}

	for len(volume.DropletIDs) > 0 {
		time.Sleep(time.Second)

		volume, err = GetVolumeByName(client, name)
		if err != nil {
			return err
		} else if volume == nil {
			break
		}
	}

	_, err = client.Storage.DeleteVolume(context.Background(), volume.ID)
	return err
}
