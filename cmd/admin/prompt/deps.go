package prompt

import (
	toolingdao "github.com/flarebyte/baldrick-rebec/internal/dao/tooling"
	responsesvc "github.com/flarebyte/baldrick-rebec/internal/service/responses"
	factorypkg "github.com/flarebyte/baldrick-rebec/internal/service/responses/factory"
)

// Deps holds injectable dependencies for the prompt run command.
type Deps struct {
	ToolDAO          toolingdao.ToolDAO
	VaultDAO         toolingdao.VaultDAO
	LLMFactory       factorypkg.LLMFactory
	ResponsesService responsesvc.ResponsesService
}

var deps = Deps{}

// SetDeps allows main or tests to provide custom dependencies.
func SetDeps(d Deps) {
	deps = d
}

func ensureDefaults() {
	// ToolDAO must be provided by caller (e.g., command initializes PG adapter).
	// Fallback to mock for tests if not set.
	if deps.ToolDAO == nil {
		deps.ToolDAO = toolingdao.NewMockToolDAO(nil)
	}
	if deps.VaultDAO == nil {
		deps.VaultDAO = toolingdao.NewMockVaultDAO(nil)
	}
	if deps.LLMFactory == nil {
		deps.LLMFactory = factorypkg.New()
	}
	if deps.ResponsesService == nil {
		deps.ResponsesService = responsesvc.New()
	}
}
