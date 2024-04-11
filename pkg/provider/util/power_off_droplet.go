package util

import (
	"context"
	"fmt"
	"time"

	"github.com/digitalocean/godo"
)

func PowerOffDroplet(client *godo.Client, dropletID int) error {
	_, _, err := client.DropletActions.PowerOff(context.Background(), dropletID)

	if err != nil {
		return fmt.Errorf("error powering off droplet: %v", err)
	}

	for {
		droplet, _, err := client.Droplets.Get(context.Background(), dropletID)
		if err != nil {
			return fmt.Errorf("error getting a droplet: %v", err)
		}

		if droplet.Status == "off" {
			break
		}

		time.Sleep(time.Second * 10)
	}

	return nil
}
