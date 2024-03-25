package provider_test

import (
	"encoding/json"
	"testing"

	daytona_provider "github.com/daytonaio/daytona/pkg/provider"
	"github.com/daytonaio/daytona/pkg/types"

	"daytonaio/daytona-digitalocean-provider/pkg/provider"
	provider_types "daytonaio/daytona-digitalocean-provider/pkg/types"
)

var sampleProvider = &provider.DigitalOceanProvider{}
var targetOptions = &provider_types.TargetOptions{
	RequiredString: "default-required-string",
}
var optionsString string

var project1 = &types.Project{
	Name: "test",
	Repository: &types.Repository{
		Id:     "123",
		Url:    "https://github.com/daytonaio/daytona",
		Name:   "daytona",
		Branch: "main",
	},
	WorkspaceId: "123",
}

var workspace = &types.Workspace{
	Id:     "123",
	Name:   "test",
	Target: "local",
	Projects: []*types.Project{
		project1,
	},
}

func TestCreateWorkspace(t *testing.T) {
	wsReq := &daytona_provider.WorkspaceRequest{
		TargetOptions: optionsString,
		Workspace:     workspace,
	}

	_, err := sampleProvider.CreateWorkspace(wsReq)
	if err != nil {
		t.Errorf("Error creating workspace: %s", err)
	}
}

func TestGetWorkspaceInfo(t *testing.T) {
	wsReq := &daytona_provider.WorkspaceRequest{
		TargetOptions: optionsString,
		Workspace:     workspace,
	}

	workspaceInfo, err := sampleProvider.GetWorkspaceInfo(wsReq)
	if err != nil || workspaceInfo == nil {
		t.Errorf("Error getting workspace info: %s", err)
	}

	var workspaceMetadata provider_types.WorkspaceMetadata
	err = json.Unmarshal([]byte(workspaceInfo.ProviderMetadata), &workspaceMetadata)
	if err != nil {
		t.Errorf("Error unmarshalling workspace metadata: %s", err)
	}

	if workspaceMetadata.Property != wsReq.Workspace.Id {
		t.Errorf("Expected network id %s, got %s", wsReq.Workspace.Id, workspaceMetadata.Property)
	}
}

func TestDestroyWorkspace(t *testing.T) {
	wsReq := &daytona_provider.WorkspaceRequest{
		TargetOptions: optionsString,
		Workspace:     workspace,
	}

	_, err := sampleProvider.DestroyWorkspace(wsReq)
	if err != nil {
		t.Errorf("Error deleting workspace: %s", err)
	}
}

func TestCreateProject(t *testing.T) {
	TestCreateWorkspace(t)

	projectReq := &daytona_provider.ProjectRequest{
		TargetOptions: optionsString,
		Project:       project1,
	}

	_, err := sampleProvider.CreateProject(projectReq)
	if err != nil {
		t.Errorf("Error creating project: %s", err)
	}
}

func TestDestroyProject(t *testing.T) {
	projectReq := &daytona_provider.ProjectRequest{
		TargetOptions: optionsString,
		Project:       project1,
	}

	_, err := sampleProvider.DestroyProject(projectReq)
	if err != nil {
		t.Errorf("Error deleting project: %s", err)
	}

	TestDestroyWorkspace(t)
}

func init() {
	_, err := sampleProvider.Initialize(daytona_provider.InitializeProviderRequest{
		BasePath:          "/tmp/workspaces",
		ServerDownloadUrl: "https://download.daytona.io/daytona/install.sh",
		ServerVersion:     "latest",
		ServerUrl:         "",
		ServerApiUrl:      "",
		LogsDir:           "/tmp/logs",
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
