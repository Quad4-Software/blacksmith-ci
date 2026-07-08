// Copyright 2024 Woodpecker Authors
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

package forgejo

import (
	"io"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/forge/types"
	"codefloe.com/crowci/crow/v6/server/model"
)

const (
	hookEvent       = "X-Forgejo-Event"
	hookPush        = "push"
	hookCreated     = "create"
	hookPullRequest = "pull_request"
	hookRelease     = "release"

	actionOpen   = "opened"
	actionSync   = "synchronized"
	actionClose  = "closed"
	actionEdited = "edited"

	refBranch = "branch"
	refTag    = "tag"
)

// parseHook parses a Forgejo hook from an http.Request and returns
// Repo and Pipeline detail. If a hook type is unsupported nil values are returned.
func parseHook(r *http.Request) (*model.Repo, *model.Pipeline, error) {
	hookType := r.Header.Get(hookEvent)
	log.Trace().Str("hook-type", hookType).Msg("parseHook: received hook")
	switch hookType {
	case hookPush:
		repo, pipeline, err := parsePushHook(r.Body)
		log.Trace().Bool("has-repo", repo != nil).Bool("has-pipeline", pipeline != nil).Err(err).Msg("parseHook: parsePushHook returned")
		return repo, pipeline, err
	case hookCreated:
		repo, pipeline, err := parseCreatedHook(r.Body)
		log.Trace().Bool("has-repo", repo != nil).Bool("has-pipeline", pipeline != nil).Err(err).Msg("parseHook: parseCreatedHook returned")
		return repo, pipeline, err
	case hookPullRequest:
		repo, pipeline, err := parsePullRequestHook(r.Body)
		log.Trace().Bool("has-repo", repo != nil).Bool("has-pipeline", pipeline != nil).Err(err).Msg("parseHook: parsePullRequestHook returned")
		return repo, pipeline, err
	case hookRelease:
		repo, pipeline, err := parseReleaseHook(r.Body)
		log.Trace().Bool("has-repo", repo != nil).Bool("has-pipeline", pipeline != nil).Err(err).Msg("parseHook: parseReleaseHook returned")
		return repo, pipeline, err
	}
	log.Debug().Msgf("unsupported hook type: '%s'", hookType)
	return nil, nil, &types.ErrIgnoreEvent{Event: hookType}
}

// parsePushHook parses a push hook and returns the Repo and Pipeline details.
// If the commit type is unsupported nil values are returned.
func parsePushHook(payload io.Reader) (repo *model.Repo, pipeline *model.Pipeline, err error) {
	push, err := parsePush(payload)
	if err != nil {
		log.Trace().Err(err).Msg("parsePushHook: failed to parse push")
		return nil, nil, err
	}

	log.Trace().Str("ref", push.Ref).Str("ref-type", push.RefType).Msg("parsePushHook: parsed push")

	// ignore push events for tags
	if strings.HasPrefix(push.Ref, "refs/tags/") {
		log.Trace().Str("ref", push.Ref).Msg("parsePushHook: ignoring tag push")
		return nil, nil, nil
	}

	// TODO is this even needed?
	if push.RefType == refBranch {
		log.Trace().Str("ref-type", push.RefType).Msg("parsePushHook: ignoring refType=branch")
		return nil, nil, nil
	}

	repo = toRepo(push.Repo)
	pipeline = pipelineFromPush(push)
	log.Trace().Str("repo", repo.FullName).Str("event", string(pipeline.Event)).Msg("parsePushHook: created pipeline")
	return repo, pipeline, err
}

// parseCreatedHook parses a push hook and returns the Repo and Pipeline details.
// If the commit type is unsupported nil values are returned.
func parseCreatedHook(payload io.Reader) (repo *model.Repo, pipeline *model.Pipeline, err error) {
	push, err := parsePush(payload)
	if err != nil {
		return nil, nil, err
	}

	if push.RefType != refTag {
		return nil, nil, nil
	}

	repo = toRepo(push.Repo)
	pipeline = pipelineFromTag(push)
	return repo, pipeline, nil
}

// parsePullRequestHook parses a pull_request hook and returns the Repo and Pipeline details.
func parsePullRequestHook(payload io.Reader) (*model.Repo, *model.Pipeline, error) {
	var (
		repo     *model.Repo
		pipeline *model.Pipeline
	)

	pr, err := parsePullRequest(payload)
	if err != nil {
		log.Trace().Err(err).Msg("parsePullRequestHook: failed to parse pull request")
		return nil, nil, err
	}

	log.Trace().Str("action", pr.Action).Int64("pr-number", pr.Number).Msg("parsePullRequestHook: parsed PR")

	// Only trigger pipelines for selected event types
	if pr.Action != actionOpen && pr.Action != actionSync && pr.Action != actionClose && pr.Action != actionEdited {
		log.Debug().Msgf("pull_request action is '%s'. Only 'open', 'sync' and 'edited' are supported", pr.Action)
		return nil, nil, nil
	}

	repo = toRepo(pr.Repo)
	pipeline = pipelineFromPullRequest(pr)
	log.Trace().Str("repo", repo.FullName).Str("event", string(pipeline.Event)).Msg("parsePullRequestHook: created pipeline")
	return repo, pipeline, err
}

// parseReleaseHook parses a release hook and returns the Repo and Pipeline details.
func parseReleaseHook(payload io.Reader) (*model.Repo, *model.Pipeline, error) {
	var (
		repo     *model.Repo
		pipeline *model.Pipeline
	)

	release, err := parseRelease(payload)
	if err != nil {
		return nil, nil, err
	}

	repo = toRepo(release.Repo)
	pipeline = pipelineFromRelease(release)
	return repo, pipeline, err
}
