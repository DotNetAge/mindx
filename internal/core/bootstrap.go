package core

import (
	"fmt"
	"io/fs"

	"github.com/DotNetAge/mindx/internal/i18n"
)

func Bootstrap(embeddedFS fs.FS, workspaceDir string) (*MindxConfig, error) {
	if err := ExtractWorkspace(embeddedFS, workspaceDir); err != nil {
			return nil, fmt.Errorf(i18n.T("error.workspace.init"), err)
	}

	cfg, err := LoadMindxConfig(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf(i18n.T("error.config.load"), err)
	}

	return cfg, nil
}
