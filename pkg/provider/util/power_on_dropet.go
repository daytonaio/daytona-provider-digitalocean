package util

import (
	"context"
	"fmt"
	"time"

	"github.com/digitalocean/godo"
)

func PowerOnDroplet(client *godo.Client, dropletID int) error {
	_, _, err := client.DropletActions.PowerOn(context.Background(), dropletID)
	if err != nil {
		return fmt.Errorf("error powering off droplet: %v", err)
	}

	for {
		droplet, _, err := client.Droplets.Get(context.Background(), dropletID)
		if err != nil {
			return fmt.Errorf("error getting a droplet: %v", err)
		}

		if droplet.Status == "active" {
			break
		}

		time.Sleep(time.Second * 10)
	}

	return nil
}
