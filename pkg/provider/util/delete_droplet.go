package util

import (
	"context"
	"fmt"

	"github.com/digitalocean/godo"
)

func DeleteDroplet(client *godo.Client, dropletID int) error {
	// Delete the droplet
	_, err := client.Droplets.Delete(context.Background(), dropletID)
	if err != nil {
		return fmt.Errorf("error deleting droplet: %v", err)
	}

	return nil
}
