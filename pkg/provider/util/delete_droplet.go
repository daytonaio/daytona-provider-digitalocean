package util

import (
	"context"
	"fmt"

	"github.com/digitalocean/godo"
)

func DeleteDroplet(client *godo.Client, dropletID int) error {
	_, err := client.Droplets.Delete(context.Background(), dropletID)
	if err != nil {
		return fmt.Errorf("error deleting droplet: %v", err)
	}

	return nil
}

func DeleteVolume(client *godo.Client, volumeID string) error {
	volume, _ := GetVolumeByName(client, volumeID)
	volumeID = volume.ID

	_, err := client.Storage.DeleteVolume(context.Background(), volumeID)
	if err != nil {
		return fmt.Errorf("error deleting volume: %v", err)
	}

	return nil
}
