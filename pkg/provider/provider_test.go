package provider_test

import (
	"encoding/json"
	"testing"

	"github.com/daytonaio/daytona/pkg/gitprovider"
	"github.com/daytonaio/daytona/pkg/models"
	daytona_provider "github.com/daytonaio/daytona/pkg/provider"

	"github.com/daytonaio/daytona-provider-digitalocean/pkg/provider"
	"github.com/daytonaio/daytona-provider-digitalocean/pkg/types"
)

var doProvider = &provider.DigitalOceanProvider{}
var targetOptions = &types.TargetOptions{
	Region:    "nyc3",
	Size:      "s-1vcpu-1gb",
	Image:     "ubuntu-20-04-x64",
	AuthToken: &[]string{"DO_AUTH_TOKEN"}[0],
	DiskSize:  20,
}
var optionsString string

var workspace1 = &models.Workspace{
	Id:   "123",
	Name: "test",
	Repository: &gitprovider.GitRepository{
		Id:   "123",
		Url:  "https://github.com/daytonaio/daytona",
		Name: "daytona",
	},
	EnvVars: map[string]string{
		"DAYTONA_TARGET_ID":                "123",
		"DAYTONA_WORKSPACE_ID":             "test",
		"DAYTONA_WORKSPACE_REPOSITORY_URL": "https://github.com/daytonaio/daytona",
		"DAYTONA_SERVER_API_KEY":           "api-key-test",
		"DAYTONA_SERVER_VERSION":           "latest",
		"DAYTONA_SERVER_URL":               "http://localhost:3001",
		"DAYTONA_SERVER_API_URL":           "http://localhost:3000",
	},
}

var target1 = &models.Target{
	Id:   "123",
	Name: "test",
}

func TestCreateTarget(t *testing.T) {
	tgReq := &daytona_provider.TargetRequest{
		Target: target1,
	}

	_, err := doProvider.CreateTarget(tgReq)
	if err != nil {
		t.Errorf("Error creating target: %s", err)
	}
}

func TestDestroyTarget(t *testing.T) {
	tgReq := &daytona_provider.TargetRequest{
		Target: target1,
	}

	_, err := doProvider.DestroyTarget(tgReq)
	if err != nil {
		t.Errorf("Error deleting target: %s", err)
	}
}

func TestCreateWorkspace(t *testing.T) {
	TestCreateTarget(t)

	workspaceReq := &daytona_provider.WorkspaceRequest{
		Workspace: workspace1,
	}

	_, err := doProvider.CreateWorkspace(workspaceReq)
	if err != nil {
		t.Errorf("Error creating workspace: %s", err)
	}
}

func TestStartWorkspace(t *testing.T) {
	workspaceReq := &daytona_provider.WorkspaceRequest{
		Workspace: workspace1,
	}

	_, err := doProvider.StartWorkspace(workspaceReq)
	if err != nil {
		t.Errorf("Error starting workspace: %s", err)
	}
}

func TestStopWorkspace(t *testing.T) {
	workspaceReq := &daytona_provider.WorkspaceRequest{
		Workspace: workspace1,
	}

	_, err := doProvider.StopWorkspace(workspaceReq)
	if err != nil {
		t.Errorf("Error stopping workspace: %s", err)
	}
}

func TestDestroyWorkspace(t *testing.T) {
	workspaceReq := &daytona_provider.WorkspaceRequest{
		Workspace: workspace1,
	}

	_, err := doProvider.DestroyWorkspace(workspaceReq)
	if err != nil {
		t.Errorf("Error deleting workspace: %s", err)
	}

	TestDestroyTarget(t)
}

func init() {
	_, err := doProvider.Initialize(daytona_provider.InitializeProviderRequest{
		BasePath:           "/tmp/workspaces",
		DaytonaDownloadUrl: "https://download.daytona.io/daytona/get-server.sh",
		DaytonaVersion:     "latest",
		ServerUrl:          "",
		ApiUrl:             "",
		ServerPort:         0,
		ApiPort:            0,
		WorkspaceLogsDir:   "/tmp/logs",
		TargetLogsDir:      "/tmp/logs",
	})
	if err != nil {
		panic(err)
	}

	opts, err := json.Marshal(targetOptions)
	if err != nil {
		panic(err)
	}

	optionsString = string(opts)
}
