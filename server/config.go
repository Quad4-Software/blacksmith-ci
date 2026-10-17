// Copyright 2018 Drone.IO Inc.
// Copyright 2021 Informatyka Boguslawski sp. z o.o. sp.k., http://www.ib.pl/
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

package server

import (
	"time"

	"codefloe.com/crowci/crow/v6/server/auth/ldap"
	"codefloe.com/crowci/crow/v6/server/auth/oidc"
	"codefloe.com/crowci/crow/v6/server/cache"
	"codefloe.com/crowci/crow/v6/server/logging"
	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/proclog"
	"codefloe.com/crowci/crow/v6/server/pubsub"
	"codefloe.com/crowci/crow/v6/server/queue"
	"codefloe.com/crowci/crow/v6/server/services"
	"codefloe.com/crowci/crow/v6/server/services/buildcache"
	"codefloe.com/crowci/crow/v6/server/services/ciquota"
	"codefloe.com/crowci/crow/v6/server/services/integration"
	"codefloe.com/crowci/crow/v6/server/services/log"
	"codefloe.com/crowci/crow/v6/server/services/notification"
	"codefloe.com/crowci/crow/v6/server/services/permissions"
	"codefloe.com/crowci/crow/v6/server/theme"
)

var Config = struct {
	Services struct {
		Pubsub               pubsub.Publisher
		Queue                queue.Queue
		Logs                 logging.Log
		Membership           cache.MembershipService
		Manager              services.Manager
		LogStore             log.Service
		MaintenanceService   services.MaintenanceService
		MaintenanceScheduler services.MaintenanceScheduler
		Store                services.Store
		IntegrationService   integration.Service
		NotificationService  notification.Service
		CIQuotaReporter      ciquota.Reporter
		ProcLogReceiver      *proclog.Receiver
		ProcLogStream        *proclog.Stream
		ProcLogJanitor       *proclog.Janitor
		// OIDC is the identity provider for single sign-on; nil when not configured.
		OIDC *oidc.Provider
		// LDAP is the directory for username and password sign-in; nil when not configured.
		LDAP *ldap.Provider
		// BuildCache stores workflow caches; nil when caching is disabled.
		BuildCache *buildcache.Service
	}
	Server struct {
		JWTSecret           string
		Key                 string
		Cert                string
		OAuthHost           string
		Host                string
		WebhookHost         string
		Port                string
		PortTLS             string
		AgentToken          string
		StatusContext       string
		StatusContextFormat string
		SessionExpires      time.Duration
		RootPath            string
		CustomCSSFile       string
		CustomJsFile        string
		InstanceID          string // Instance ID for HA deployments
		// ForgeLoginDisabled switches off forge OAuth sign-in; forge accounts can still be linked.
		ForgeLoginDisabled bool
		// TrustedProxies are the IPs and CIDR ranges whose X-Forwarded-For and
		// X-Real-IP headers set the client IP; empty trusts every peer.
		TrustedProxies []string
	}
	Agent struct {
		DisableUserRegisteredAgentRegistration bool
	}
	WebUI struct {
		EnableSwagger    bool
		SkipVersionCheck bool
		CustomThemes     []theme.Theme // admin-provided themes, offered next to the bundled ones
	}
	Queue struct {
		SchedulingStrategy string // active agent scheduling strategy, surfaced in the UI
	}
	Prometheus struct {
		AuthToken string
	}
	Pipeline struct {
		AuthenticatePublicRepos             bool
		DefaultAllowPullRequests            bool
		DefaultCancelPreviousPipelineEvents []model.WebhookEvent
		DefaultApprovalMode                 model.ApprovalMode
		DefaultWorkflowLabels               map[string]string
		DefaultClonePlugin                  string
		CachePlugin                         string
		TrustedClonePlugins                 []string
		Volumes                             []string
		Networks                            []string
		PrivilegedPlugins                   []string
		PrivilegedPluginsMatchType          string
		DefaultTimeout                      int64
		LogsDurationKeep                    string
		LogsPipelinesKeepMin                int64
		MaxTimeout                          int64
		MaxMatrixSize                       int // Maximum number of matrix permutations allowed
		ApprovalGateTimeoutDefault          time.Duration
		ApprovalGateTimeoutMax              time.Duration
		ApprovalGateSweepInterval           time.Duration
		Proxy                               struct {
			No    string
			HTTP  string
			HTTPS string
		}
		ContainerNameScheme string // 'hash' or 'descriptive'
	}
	Maintenance struct {
		LogVacuum struct {
			Enabled  bool
			Schedule string
		}
	}
	Permissions struct {
		Open            bool
		Admins          *permissions.Admins
		Orgs            *permissions.Orgs
		OwnersAllowlist *permissions.OwnersAllowlist
	}
}{}
