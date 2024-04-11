package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	internal "github.com/daytonaio/daytona-provider-digitalocean/internal"
	log_writers "github.com/daytonaio/daytona-provider-digitalocean/internal/log"
	"github.com/daytonaio/daytona-provider-digitalocean/pkg/provider/util"
	provider_types "github.com/daytonaio/daytona-provider-digitalocean/pkg/types"
	"golang.org/x/oauth2"

	"github.com/daytonaio/daytona/pkg/logger"
	"github.com/daytonaio/daytona/pkg/provider"
	"github.com/daytonaio/daytona/pkg/types"
	"github.com/digitalocean/godo"

	log "github.com/sirupsen/logrus"
)

type DigitalOceanProvider struct {
	BasePath          *string
	ServerDownloadUrl *string
	ServerVersion     *string
	ServerUrl         *string
	ServerApiUrl      *string
	LogsDir           *string
	OwnProperty       string
}

func (p *DigitalOceanProvider) Initialize(req provider.InitializeProviderRequest) (*types.Empty, error) {
	p.OwnProperty = "my-own-property"

	p.BasePath = &req.BasePath
	p.ServerDownloadUrl = &req.ServerDownloadUrl
	p.ServerVersion = &req.ServerVersion
	p.ServerUrl = &req.ServerUrl
	p.ServerApiUrl = &req.ServerApiUrl
	p.LogsDir = &req.LogsDir

	return new(types.Empty), nil
}

func (p DigitalOceanProvider) GetInfo() (provider.ProviderInfo, error) {
	return provider.ProviderInfo{
		Name:    "digitalocean-provider",
		Version: internal.Version,
	}, nil
}

func (p DigitalOceanProvider) GetTargetManifest() (*provider.ProviderTargetManifest, error) {
	return provider_types.GetTargetManifest(), nil
}

func (p DigitalOceanProvider) GetDefaultTargets() (*[]provider.ProviderTarget, error) {
	info, err := p.GetInfo()
	if err != nil {
		return nil, err
	}

	defaultTargets := []provider.ProviderTarget{
		{
			Name:         "default-target",
			ProviderInfo: info,
			Options:      "{\n\t\"Required String\": \"default-required-string\"\n}",
		},
	}
	return &defaultTargets, nil
}

func (p DigitalOceanProvider) CreateWorkspace(workspaceReq *provider.WorkspaceRequest) (*types.Empty, error) {
	// logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	// if p.LogsDir != nil {
	// 	wsLogWriter := logger.GetWorkspaceLogger(*p.LogsDir, workspaceReq.Workspace.Id)
	// 	logWriter = io.MultiWriter(&log_writers.InfoLogWriter{}, wsLogWriter)
	// 	defer wsLogWriter.Close()
	// }

	// logWriter.Write([]byte("Creating workspace...\n"))

	// logWriter.Write([]byte("Workspace created\n"))

	return new(types.Empty), nil
}

func (p DigitalOceanProvider) StartWorkspace(workspaceReq *provider.WorkspaceRequest) (*types.Empty, error) {
	return new(types.Empty), nil
}

func (p DigitalOceanProvider) StopWorkspace(workspaceReq *provider.WorkspaceRequest) (*types.Empty, error) {
	return new(types.Empty), nil
}

func (p DigitalOceanProvider) DestroyWorkspace(workspaceReq *provider.WorkspaceRequest) (*types.Empty, error) {
	return new(types.Empty), nil
}

func (p DigitalOceanProvider) GetWorkspaceInfo(workspaceReq *provider.WorkspaceRequest) (*types.WorkspaceInfo, error) {
	providerMetadata, err := p.getWorkspaceMetadata(workspaceReq)
	if err != nil {
		return nil, err
	}

	workspaceInfo := &types.WorkspaceInfo{
		Name:             workspaceReq.Workspace.Name,
		ProviderMetadata: providerMetadata,
	}

	projectInfos := []*types.ProjectInfo{}
	for _, project := range workspaceReq.Workspace.Projects {
		projectInfo, err := p.GetProjectInfo(&provider.ProjectRequest{
			TargetOptions: workspaceReq.TargetOptions,
			Project:       project,
		})
		if err != nil {
			return nil, err
		}
		projectInfos = append(projectInfos, projectInfo)
	}
	workspaceInfo.Projects = projectInfos

	return workspaceInfo, nil
}

func (p DigitalOceanProvider) CreateProject(projectReq *provider.ProjectRequest) (*types.Empty, error) {
	logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	if p.LogsDir != nil {
		wsLogWriter := logger.GetWorkspaceLogger(*p.LogsDir, projectReq.Project.WorkspaceId)
		projectLogWriter := logger.GetProjectLogger(*p.LogsDir, projectReq.Project.WorkspaceId, projectReq.Project.Name)
		logWriter = io.MultiWriter(&log_writers.InfoLogWriter{}, wsLogWriter, projectLogWriter)
		defer wsLogWriter.Close()
		defer projectLogWriter.Close()
	}

	// Parse the JSON string into a TargetOptions struct
	targetOptions, err := provider_types.ParseTargetOptions(projectReq.TargetOptions)
	if err != nil {
		log.Fatalf("Error parsing target options: %v", err)
	}

	client, err := p.getClient(targetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to get client: " + err.Error() + "\n"))
		return nil, err
	}

	// Create DigitalOcean droplet
	_, err = util.CreateDroplet(client, projectReq.Project, targetOptions, *p.ServerDownloadUrl)
	if err != nil {
		logWriter.Write([]byte("Failed to create droplet: " + err.Error() + "\n"))
		return nil, err
	}

	logWriter.Write([]byte("Project created\n"))

	return new(types.Empty), nil
}

func (p DigitalOceanProvider) StartProject(projectReq *provider.ProjectRequest) (*types.Empty, error) {
	logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	if p.LogsDir != nil {
		wsLogWriter := logger.GetWorkspaceLogger(*p.LogsDir, projectReq.Project.WorkspaceId)
		projectLogWriter := logger.GetProjectLogger(*p.LogsDir, projectReq.Project.WorkspaceId, projectReq.Project.Name)
		logWriter = io.MultiWriter(&log_writers.InfoLogWriter{}, wsLogWriter, projectLogWriter)
		defer wsLogWriter.Close()
		defer projectLogWriter.Close()
	}

	// Parse the JSON string into a TargetOptions struct
	targetOptions, err := provider_types.ParseTargetOptions(projectReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Error parsing target options: " + err.Error() + "\n"))
		return nil, err
	}

	client, err := p.getClient(targetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to get client: " + err.Error() + "\n"))
		return nil, err
	}

	// Construct the droplet name and get the droplet ID
	droplet, err := util.GetDroplet(client, util.GetDropletName(projectReq.Project))
	if err != nil {
		logWriter.Write([]byte("Failed to get droplet ID: " + err.Error() + "\n"))
		return nil, err
	}

	// Power on DigitalOcean droplet
	err = util.PowerOnDroplet(client, droplet.ID)
	if err != nil {
		logWriter.Write([]byte("Failed to delete droplet: " + err.Error() + "\n"))
		return nil, err
	}

	logWriter.Write([]byte("Droplet powered on.\n"))

	return new(types.Empty), nil
}

func (p DigitalOceanProvider) StopProject(projectReq *provider.ProjectRequest) (*types.Empty, error) {
	logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	if p.LogsDir != nil {
		wsLogWriter := logger.GetWorkspaceLogger(*p.LogsDir, projectReq.Project.WorkspaceId)
		projectLogWriter := logger.GetProjectLogger(*p.LogsDir, projectReq.Project.WorkspaceId, projectReq.Project.Name)
		logWriter = io.MultiWriter(&log_writers.InfoLogWriter{}, wsLogWriter, projectLogWriter)
		defer wsLogWriter.Close()
		defer projectLogWriter.Close()
	}

	// Parse the JSON string into a TargetOptions struct
	targetOptions, err := provider_types.ParseTargetOptions(projectReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Error parsing target options: " + err.Error() + "\n"))
		return nil, err
	}

	client, err := p.getClient(targetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to get client: " + err.Error() + "\n"))
		return nil, err
	}

	// Construct the droplet name and get the droplet ID
	droplet, err := util.GetDroplet(client, util.GetDropletName(projectReq.Project))
	if err != nil {
		logWriter.Write([]byte("Failed to get droplet ID: " + err.Error() + "\n"))
		return nil, err
	}

	// Power off DigitalOcean droplet
	err = util.PowerOffDroplet(client, droplet.ID)
	if err != nil {
		logWriter.Write([]byte("Failed to delete droplet: " + err.Error() + "\n"))
		return nil, err
	}

	logWriter.Write([]byte("Droplet powered off.\n"))

	return new(types.Empty), nil
}

func (p DigitalOceanProvider) DestroyProject(projectReq *provider.ProjectRequest) (*types.Empty, error) {
	logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	if p.LogsDir != nil {
		wsLogWriter := logger.GetWorkspaceLogger(*p.LogsDir, projectReq.Project.WorkspaceId)
		projectLogWriter := logger.GetProjectLogger(*p.LogsDir, projectReq.Project.WorkspaceId, projectReq.Project.Name)
		logWriter = io.MultiWriter(&log_writers.InfoLogWriter{}, wsLogWriter, projectLogWriter)
		defer wsLogWriter.Close()
		defer projectLogWriter.Close()
	}

	// Parse the JSON string into a TargetOptions struct
	targetOptions, err := provider_types.ParseTargetOptions(projectReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Error parsing target options: " + err.Error() + "\n"))
		return nil, err
	}

	client, err := p.getClient(targetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to get client: " + err.Error() + "\n"))
		return nil, err
	}

	// Construct the droplet name and get the droplet ID
	droplet, err := util.GetDroplet(client, util.GetDropletName(projectReq.Project))
	if err != nil {
		logWriter.Write([]byte("Failed to get droplet ID: " + err.Error() + "\n"))
		return nil, err
	}

	// Delete DigitalOcean droplet
	err = util.DeleteDroplet(client, droplet.ID)
	if err != nil {
		logWriter.Write([]byte("Failed to delete droplet: " + err.Error() + "\n"))
		return nil, err
	}

	logWriter.Write([]byte("Project deleted\n"))

	return new(types.Empty), nil
}

func (p DigitalOceanProvider) GetProjectInfo(projectReq *provider.ProjectRequest) (*types.ProjectInfo, error) {
	logWriter := io.MultiWriter(&log_writers.InfoLogWriter{})
	if p.LogsDir != nil {
		wsLogWriter := logger.GetWorkspaceLogger(*p.LogsDir, projectReq.Project.WorkspaceId)
		projectLogWriter := logger.GetProjectLogger(*p.LogsDir, projectReq.Project.WorkspaceId, projectReq.Project.Name)
		logWriter = io.MultiWriter(&log_writers.InfoLogWriter{}, wsLogWriter, projectLogWriter)
		defer wsLogWriter.Close()
		defer projectLogWriter.Close()
	}

	providerMetadata := provider_types.ProjectMetadata{
		Property: projectReq.Project.Name,
	}

	metadataString, err := json.Marshal(providerMetadata)
	if err != nil {
		return nil, err
	}

	// Parse the JSON string into a TargetOptions struct
	targetOptions, err := provider_types.ParseTargetOptions(projectReq.TargetOptions)
	if err != nil {
		logWriter.Write([]byte("Error parsing target options: " + err.Error() + "\n"))
		return nil, err
	}

	client, err := p.getClient(targetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to get client: " + err.Error() + "\n"))
		return nil, err
	}

	droplet, err := util.GetDroplet(client, util.GetDropletName(projectReq.Project))
	if err != nil {
		return nil, fmt.Errorf("error getting droplet status: %v", err)
	}

	isRunning := droplet.Status == "active"

	projectInfo := &types.ProjectInfo{
		Name:             projectReq.Project.Name,
		IsRunning:        isRunning,
		Created:          droplet.Created,
		Started:          "-",
		Finished:         "-",
		ProviderMetadata: string(metadataString),
	}

	return projectInfo, nil
}

func (p DigitalOceanProvider) getWorkspaceMetadata(workspaceReq *provider.WorkspaceRequest) (string, error) {
	metadata := provider_types.WorkspaceMetadata{
		Property: workspaceReq.Workspace.Id,
	}

	jsonContent, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}

	return string(jsonContent), nil
}

func (p DigitalOceanProvider) getClient(targetOptions *provider_types.TargetOptions) (*godo.Client, error) {
	doToken := targetOptions.AuthToken

	if doToken == nil {
		envToken := os.Getenv("DIGITALOCEAN_ACCESS_TOKEN")
		// Get the DigitalOcean token from the environment variable
		if envToken == "" {
			return nil, errors.New("DigitalOcean Token not found")
		}

		doToken = &envToken
	}

	// Create a new DigitalOcean client
	oauthClient := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *doToken}))
	client := godo.NewClient(oauthClient)

	return client, nil
}
