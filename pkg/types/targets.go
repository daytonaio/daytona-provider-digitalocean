package types

import (
	"encoding/json"

	"github.com/daytonaio/daytona/pkg/models"
)

type TargetOptions struct {
	Region    string  `json:"Region"`               // Region slug
	Size      string  `json:"Size"`                 // Size slug
	DiskSize  int     `json:"Disk Size"`            // Disk Size integer
	Image     string  `json:"Image"`                // Image slug
	AuthToken *string `json:"Auth Token,omitempty"` // Auth token
}

func GetTargetConfigManifest() *models.TargetConfigManifest {
	return &models.TargetConfigManifest{
		"Region": models.TargetConfigProperty{
			Type:         models.TargetConfigPropertyTypeString,
			DefaultValue: "fra1",
		},
		"Size": models.TargetConfigProperty{
			Type:         models.TargetConfigPropertyTypeString,
			DefaultValue: "s-2vcpu-4gb",
		},
		"Disk Size": models.TargetConfigProperty{
			Type:         models.TargetConfigPropertyTypeInt,
			DefaultValue: "20",
		},
		"Image": models.TargetConfigProperty{
			Type:         models.TargetConfigPropertyTypeString,
			DefaultValue: "docker-20-04",
		},
		"Auth Token": models.TargetConfigProperty{
			Type:        models.TargetConfigPropertyTypeString,
			InputMasked: true,
			Description: "If empty, token will be fetched from the DIGITALOCEAN_ACCESS_TOKEN environment variable.",
		},
	}
}

func ParseTargetOptions(optionsJson string) (*TargetOptions, error) {
	var targetOptions TargetOptions
	err := json.Unmarshal([]byte(optionsJson), &targetOptions)
	if err != nil {
		return nil, err
	}

	return &targetOptions, nil
}
