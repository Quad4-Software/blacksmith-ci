// Copyright 2022 Woodpecker Authors
// Copyright 2018 Drone.IO Inc.
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

package main

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"

	"codefloe.com/crowci/crow/v6/server/model"
	maintenanceMocks "codefloe.com/crowci/crow/v6/server/services/maintenance/mocks"
)

func TestSetupKubernetesCleanup(t *testing.T) {
	tests := []struct {
		name       string
		backend    string
		enabledEnv string
		args       []string
		available  bool
		enabled    bool
	}{
		{name: "unconfigured"},
		{name: "enabled flag alone", args: []string{"--maintenance-kubernetes-cleanup-enabled"}, available: true, enabled: true},
		{name: "enabled environment alone", enabledEnv: "true", available: true, enabled: true},
		{name: "backend auto enable", backend: "kubernetes", available: true, enabled: true},
		{name: "explicit flag disables auto enable", backend: "kubernetes", args: []string{"--maintenance-kubernetes-cleanup-enabled=false"}, available: true},
		{name: "explicit environment disables auto enable", backend: "kubernetes", enabledEnv: "false", available: true},
		{name: "age allows manual cleanup", args: []string{"--maintenance-kubernetes-cleanup-age=24h"}, available: true},
		{name: "invalid age", args: []string{"--maintenance-kubernetes-cleanup-enabled", "--maintenance-kubernetes-cleanup-age=invalid"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CROW_BACKEND", tt.backend)
			t.Setenv("WOODPECKER_BACKEND", "")
			t.Setenv("CROW_MAINTENANCE_KUBERNETES_CLEANUP_ENABLED", tt.enabledEnv)
			if tt.enabledEnv == "" {
				require.NoError(t, os.Unsetenv("CROW_MAINTENANCE_KUBERNETES_CLEANUP_ENABLED"))
			}
			t.Setenv("WOODPECKER_MAINTENANCE_KUBERNETES_CLEANUP_ENABLED", "")
			t.Setenv("CROW_MAINTENANCE_KUBERNETES_CLEANUP_AGE", "")
			t.Setenv("CROW_MAINTENANCE_KUBERNETES_CLEANUP_NAMESPACES", "")
			svc := maintenanceMocks.NewMockService(t)
			if tt.available {
				svc.EXPECT().SetKubernetesCleanupFunc(mock.Anything).Once()
				svc.EXPECT().SetKubernetesCleanupEnabled(true).Once()
				svc.EXPECT().GetMaintenanceConfigByAction("kubernetes_cleanup").Return(&model.MaintenanceConfig{}, nil).Once()
				svc.EXPECT().UpdateMaintenanceConfigByAction("kubernetes_cleanup", mock.MatchedBy(func(config *model.MaintenanceConfig) bool {
					return config.Enabled == tt.enabled && config.Schedule == "0 5 * * *"
				})).Return(nil).Once()
			}
			var cleanupFlags []cli.Flag
			for _, flag := range flags {
				switch flag.Names()[0] {
				case "maintenance-kubernetes-cleanup-enabled", "maintenance-kubernetes-cleanup-age", "maintenance-kubernetes-cleanup-namespaces":
					switch flag := flag.(type) {
					case *cli.BoolFlag:
						copy := *flag
						cleanupFlags = append(cleanupFlags, &copy)
					case *cli.StringFlag:
						copy := *flag
						cleanupFlags = append(cleanupFlags, &copy)
					}
				}
			}
			command := &cli.Command{Flags: cleanupFlags, Action: func(_ context.Context, c *cli.Command) error {
				setupKubernetesCleanup(c, svc)
				return nil
			}}
			require.NoError(t, command.Run(t.Context(), append([]string{"server"}, tt.args...)))
		})
	}
}
