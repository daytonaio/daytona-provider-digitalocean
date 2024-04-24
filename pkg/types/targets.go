package types

import (
	"encoding/json"

	"github.com/daytonaio/daytona/pkg/provider"
)

type TargetOptions struct {
	Region    string  `json:"Region"`               // Region slug
	Size      string  `json:"Size"`                 // Size slug
	DiskSize  int     `json:"DiskSize"`             // DiskSize integer
	Image     string  `json:"Image"`                // Image slug
	AuthToken *string `json:"Auth Token,omitempty"` // Auth token
}

func GetTargetManifest() *provider.ProviderTargetManifest {
	return &provider.ProviderTargetManifest{
		"Region": provider.ProviderTargetProperty{
			Type:         provider.ProviderTargetPropertyTypeString,
			DefaultValue: "fra1",
		},
		"Size": provider.ProviderTargetProperty{
			Type:         provider.ProviderTargetPropertyTypeString,
			DefaultValue: "s-2vcpu-4gb",
		},
		"DiskSize": provider.ProviderTargetProperty{
			Type:         provider.ProviderTargetPropertyTypeInt,
			DefaultValue: "20",
		},
		"Image": provider.ProviderTargetProperty{
			Type:         provider.ProviderTargetPropertyTypeString,
			DefaultValue: "ubuntu-22-04-x64",
		},
		"Auth Token": provider.ProviderTargetProperty{
			Type:        provider.ProviderTargetPropertyTypeString,
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
