package util

import (
	"fmt"

	"github.com/daytonaio/daytona/pkg/types"
)

func GetDropletName(project *types.Project) string {
	return fmt.Sprintf("%s-%s", project.WorkspaceId, project.Name)
}
