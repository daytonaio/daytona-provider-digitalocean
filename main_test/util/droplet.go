package util

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
)

type TargetOptions struct {
	Name     string `json:"name"`
	Region   string `json:"region"`
	Size     string `json:"size"`
	Image    string `json:"image"`
	UserData string `json:"userData"`
}

type DropletInfo struct {
	Name     string
	PublicIP string
}

func (d *DropletInfo) String() string {
	return fmt.Sprintf("Created droplet:\n  Name = %s\n  Public IP = %s", d.Name, d.PublicIP)
}

func CreateDroplet(optionsJson string) (*DropletInfo, error) {
	// Parse the JSON string into a TargetOptions struct
	var targetOptions TargetOptions
	err := json.Unmarshal([]byte(optionsJson), &targetOptions)
	if err != nil {
		log.Fatalf("Error parsing target options: %v", err)
	}

	// Get the DigitalOcean token from the environment variable
	doToken := os.Getenv("DIGITALOCEAN_ACCESS_TOKEN")
	if doToken == "" {
		log.Fatal("DIGITALOCEAN_ACCESS_TOKEN environment variable is not set")
	}

	// Create a new DigitalOcean client
	oauthClient := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: doToken}))
	client := godo.NewClient(oauthClient)

	// Generate instance object
	instance := &godo.DropletCreateRequest{
		Name:   targetOptions.Name,
		Region: targetOptions.Region,
		Size:   targetOptions.Size,
		Image: godo.DropletCreateImage{
			Slug: targetOptions.Image,
		},
		UserData: targetOptions.UserData,
		Tags:     []string{"daytona"},
	}

	// Create the droplet
	droplet, _, err := client.Droplets.Create(context.Background(), instance)
	if err != nil {
		log.Fatalf("Error creating droplet: %v", err)
	}

	// Poll the droplet's status until it becomes active
	for {
		droplet, _, err = client.Droplets.Get(context.Background(), droplet.ID)
		if err != nil {
			log.Fatalf("Error getting droplet: %v", err)
		}

		if droplet.Status == "active" {
			break
		}

		time.Sleep(time.Second * 10)
	}

	// Extract the droplet's name and public IP
	name := droplet.Name
	var publicIP string
	for _, network := range droplet.Networks.V4 {
		if network.Type == "public" {
			publicIP = network.IPAddress
			break
		}
	}

	// Create a new DropletInfo object
	dropletInfo := &DropletInfo{
		Name:     name,
		PublicIP: publicIP,
	}

	return dropletInfo, nil
}
