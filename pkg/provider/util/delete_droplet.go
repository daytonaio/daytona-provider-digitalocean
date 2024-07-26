package util

import (
	"context"

	"github.com/digitalocean/godo"
)

func DeleteDroplet(client *godo.Client, dropletID int) error {
	_, err := client.Droplets.Delete(context.Background(), dropletID)
	return err
}

func DeleteVolume(client *godo.Client, volumeID string) error {
	volume, err := GetVolumeByName(client, volumeID)
	if err != nil {
		return err
	}
	volumeID = volume.ID

	_, err = client.Storage.DeleteVolume(context.Background(), volumeID)
	return err
}
