// Copyright 2026 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration

import (
	"context"
	"fmt"

	"codefloe.com/crowci/crow/v6/server/integrations"
	"codefloe.com/crowci/crow/v6/server/integrations/cache"
	"codefloe.com/crowci/crow/v6/server/model"
)

// Service defines a service for managing integrations and fetching external secrets.
type Service interface {
	// CRUD Operations
	IntegrationFind(userID, integrationID int64) (*model.Integration, error)
	IntegrationFindByID(integrationID int64) (*model.Integration, error)
	IntegrationList(userID int64, opts *model.ListOptions) ([]*model.Integration, error)
	IntegrationListAll(opts *model.ListOptions) ([]*model.Integration, error)
	IntegrationListAccessible(userID int64, orgIDs []int64, repoIDs []int64, opts *model.ListOptions) ([]*model.Integration, error)
	IntegrationCreate(integration *model.Integration) error
	IntegrationUpdate(integration *model.Integration) error
	IntegrationDelete(userID, integrationID int64) error

	// Access Control
	CheckAccess(integration *model.Integration, userID int64, orgID int64, repoID int64) error

	// Provider Operations
	TestConnection(integration *model.Integration) error
	GetProvider(ctx context.Context, integrationID int64, pipelineID int64) (integrations.Provider, error)
	ReleasePipeline(pipelineID int64)

	// Secret Fetching (high-level)
	FetchSecret(ctx context.Context, ref *model.ExternalSecretRef, userID int64, orgID int64, repoID int64, pipelineID int64) (string, error)
	FetchSecretsBatch(ctx context.Context, refs []*model.ExternalSecretRef, userID int64, orgID int64, repoID int64, pipelineID int64) (map[string]string, error)

	// Lifecycle
	Close()
}

// ServiceFactory creates integration service instances.
type ServiceFactory func(store model.IntegrationStore, factory integrations.ProviderFactory) Service

// NewService creates a new integration service with the given store and provider factory.
func NewService(store model.IntegrationStore, factory integrations.ProviderFactory) Service {
	return &service{
		store: store,
		pool:  cache.NewProviderPool(factory),
	}
}

// service implements the Service interface.
type service struct {
	store model.IntegrationStore
	pool  *cache.ProviderPool
}

// IntegrationFind finds an integration by ID, verifying the user owns it.
func (s *service) IntegrationFind(userID, integrationID int64) (*model.Integration, error) {
	integration, err := s.store.IntegrationFind(integrationID)
	if err != nil {
		return nil, err
	}

	if integration.UserID != userID {
		return nil, model.ErrIntegrationNotFound
	}

	return integration, nil
}

// IntegrationFindByID finds an integration by ID without ownership check.
// Used internally when we've already verified access through other means.
func (s *service) IntegrationFindByID(integrationID int64) (*model.Integration, error) {
	return s.store.IntegrationFind(integrationID)
}

// IntegrationList lists integrations owned by a user.
func (s *service) IntegrationList(userID int64, opts *model.ListOptions) ([]*model.Integration, error) {
	return s.store.IntegrationList(userID, opts)
}

// IntegrationListAll lists every integration on the instance regardless of owner.
// Intended for instance admins to audit all configured integrations.
func (s *service) IntegrationListAll(opts *model.ListOptions) ([]*model.Integration, error) {
	return s.store.IntegrationListAll(opts)
}

// IntegrationListAccessible lists integrations accessible to a user based on allowlists.
func (s *service) IntegrationListAccessible(userID int64, orgIDs []int64, repoIDs []int64, opts *model.ListOptions) ([]*model.Integration, error) {
	return s.store.IntegrationListAccessible(userID, orgIDs, repoIDs, opts)
}

// IntegrationCreate creates a new integration after validation.
func (s *service) IntegrationCreate(integration *model.Integration) error {
	if err := integration.Validate(); err != nil {
		return err
	}

	return s.store.IntegrationCreate(integration)
}

// IntegrationUpdate updates an existing integration.
func (s *service) IntegrationUpdate(integration *model.Integration) error {
	if err := integration.Validate(); err != nil {
		return err
	}

	return s.store.IntegrationUpdate(integration)
}

// IntegrationDelete deletes an integration owned by the user.
func (s *service) IntegrationDelete(userID, integrationID int64) error {
	integration, err := s.IntegrationFind(userID, integrationID)
	if err != nil {
		return err
	}

	return s.store.IntegrationDelete(integration)
}

// CheckAccess verifies that the given user/org/repo has access to the integration.
func (s *service) CheckAccess(integration *model.Integration, userID int64, orgID int64, repoID int64) error {
	if !integration.IsActive {
		return fmt.Errorf("%w: integration '%s' (id=%d) is disabled",
			model.ErrIntegrationInactive, integration.Name, integration.ID)
	}

	if !integration.HasAccess(userID, orgID, repoID) {
		//nolint:staticcheck // ST1005: intentionally using markdown formatting with punctuation for user-facing error
		return fmt.Errorf("%w\n\n"+
			"**Repository** (id=%d, org=%d) is not in the allowlist for integration **%s** (id=%d).\n\n"+
			"The integration owner must add this repository, organization, or user to the integration's allowlist.",
			model.ErrIntegrationAccessDenied, repoID, orgID, integration.Name, integration.ID)
	}

	return nil
}

// TestConnection tests the connection to the integration's secret store.
func (s *service) TestConnection(integration *model.Integration) error {
	// Create a temporary provider just for testing
	provider, err := s.pool.GetProvider(context.Background(), integration, 0)
	if err != nil {
		return err
	}

	// Test the connection
	if err := provider.TestConnection(context.Background()); err != nil {
		return err
	}

	// Release the test provider immediately
	s.pool.ReleasePipeline(0)

	return nil
}

// GetProvider returns a cached provider for the given integration and pipeline.
func (s *service) GetProvider(ctx context.Context, integrationID int64, pipelineID int64) (integrations.Provider, error) {
	integration, err := s.store.IntegrationFind(integrationID)
	if err != nil {
		return nil, err
	}

	if !integration.IsActive {
		return nil, model.ErrIntegrationInactive
	}

	return s.pool.GetProvider(ctx, integration, pipelineID)
}

// ReleasePipeline releases all cached providers for a pipeline.
func (s *service) ReleasePipeline(pipelineID int64) {
	s.pool.ReleasePipeline(pipelineID)
}

// FetchSecret fetches a single secret from an external store.
func (s *service) FetchSecret(ctx context.Context, ref *model.ExternalSecretRef, userID int64, orgID int64, repoID int64, pipelineID int64) (string, error) {
	// Get the integration
	integration, err := s.store.IntegrationFind(ref.IntegrationID)
	if err != nil {
		return "", err
	}

	// Check access
	if err := s.CheckAccess(integration, userID, orgID, repoID); err != nil {
		return "", err
	}

	// Get provider from pool
	provider, err := s.pool.GetProvider(ctx, integration, pipelineID)
	if err != nil {
		return "", err
	}

	// Fetch the secret
	return provider.GetSecret(ctx, ref.Path, ref.Key, ref.Version)
}

// resolveRefIntegration resolves an external secret reference to its integration.
// References may target an integration by numeric ID or by name; name lookups are
// scoped to the integrations accessible to the requesting user/org/repo, mirroring
// the access model enforced for ID-based lookups.
func (s *service) resolveRefIntegration(ref *model.ExternalSecretRef, userID int64, orgID int64, repoID int64) (*model.Integration, error) {
	if ref.IntegrationID > 0 {
		return s.store.IntegrationFind(ref.IntegrationID)
	}

	if ref.IntegrationName == "" {
		return nil, fmt.Errorf("external secret reference requires an integration ID or name")
	}

	var orgIDs, repoIDs []int64
	if orgID > 0 {
		orgIDs = []int64{orgID}
	}
	if repoID > 0 {
		repoIDs = []int64{repoID}
	}

	accessible, err := s.store.IntegrationListAccessible(userID, orgIDs, repoIDs, nil)
	if err != nil {
		return nil, err
	}

	for _, integration := range accessible {
		if integration.Name == ref.IntegrationName {
			return integration, nil
		}
	}

	return nil, fmt.Errorf("%w: no accessible integration named %q", model.ErrIntegrationNotFound, ref.IntegrationName)
}

func (s *service) FetchSecretsBatch(ctx context.Context, refs []*model.ExternalSecretRef, userID int64, orgID int64, repoID int64, pipelineID int64) (map[string]string, error) {
	if len(refs) == 0 {
		return make(map[string]string), nil
	}

	// Group refs by integration, resolving name-based references to an integration.
	byIntegration := make(map[int64][]*model.ExternalSecretRef)
	integrationByID := make(map[int64]*model.Integration)
	for _, ref := range refs {
		integration, err := s.resolveRefIntegration(ref, userID, orgID, repoID)
		if err != nil {
			return nil, err
		}
		byIntegration[integration.ID] = append(byIntegration[integration.ID], ref)
		integrationByID[integration.ID] = integration
	}

	results := make(map[string]string, len(refs))

	// Process each integration's secrets
	for integrationID, integrationRefs := range byIntegration {
		integration := integrationByID[integrationID]

		if err := s.CheckAccess(integration, userID, orgID, repoID); err != nil {
			return nil, err
		}

		// Get provider from pool
		provider, err := s.pool.GetProvider(ctx, integration, pipelineID)
		if err != nil {
			return nil, err
		}

		// Convert refs to requests
		requests := make([]integrations.SecretRequest, len(integrationRefs))
		for i, ref := range integrationRefs {
			requests[i] = integrations.NewSecretRequest(ref)
		}

		// Fetch batch
		fetched, err := provider.GetSecretsBatch(ctx, requests)
		if err != nil {
			return nil, err
		}

		// Merge results
		for k, v := range fetched {
			results[k] = v
		}
	}

	return results, nil
}

// Close releases all resources held by the service.
func (s *service) Close() {
	s.pool.Close()
}
