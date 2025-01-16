package provider

import (
	"github.com/daytonaio/daytona-provider-digitalocean/pkg/provider/util"
	"github.com/daytonaio/daytona-provider-digitalocean/pkg/types"
	provider_util "github.com/daytonaio/daytona/pkg/provider/util"

	"github.com/daytonaio/daytona/pkg/provider"
)

func (p *DigitalOceanProvider) DestroyTarget(targetReq *provider.TargetRequest) (*provider_util.Empty, error) {
	logWriter, cleanupFunc := p.getTargetLogWriter(targetReq.Target.Id, targetReq.Target.Name)
	defer cleanupFunc()

	targetOptions, err := types.ParseTargetOptions(targetReq.Target.TargetConfig.Options)
	if err != nil {
		logWriter.Write([]byte("Error parsing target config options: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	client, err := p.getDoClient(targetOptions)
	if err != nil {
		logWriter.Write([]byte("Failed to get client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	err = util.DeleteDroplet(client, targetReq.Target, true)
	if err != nil {
		logWriter.Write([]byte("Failed to delete droplet: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	return new(provider_util.Empty), nil
}

func (p *DigitalOceanProvider) DestroyWorkspace(workspaceReq *provider.WorkspaceRequest) (*provider_util.Empty, error) {
	logWriter, cleanupFunc := p.getWorkspaceLogWriter(workspaceReq.Workspace.Id, workspaceReq.Workspace.Name)
	defer cleanupFunc()

	dockerClient, err := p.getDockerClient(workspaceReq.Workspace.TargetId)
	if err != nil {
		logWriter.Write([]byte("Failed to get docker client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}

	sshClient, err := p.getSshClient(workspaceReq.Workspace.TargetId)
	if err != nil {
		logWriter.Write([]byte("Failed to get ssh client: " + err.Error() + "\n"))
		return new(provider_util.Empty), err
	}
	defer sshClient.Close()

	return new(provider_util.Empty), dockerClient.DestroyWorkspace(workspaceReq.Workspace, p.getWorkspaceDir(workspaceReq), sshClient)
}
