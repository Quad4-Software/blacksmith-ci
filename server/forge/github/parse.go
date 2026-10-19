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

package github

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-github/v93/github"
	"github.com/rs/zerolog/log"

	"codefloe.com/crowci/crow/v6/server/forge/types"
	"codefloe.com/crowci/crow/v6/server/model"
)

const (
	hookField = "payload"

	actionOpen      = "opened"
	actionClose     = "closed"
	actionSync      = "synchronize"
	actionReleased  = "released"
	actionEdited    = "edited"
	actionLabeled   = "labeled"
	actionUnlabeled = "unlabeled"

	stateOpen  = "open"
	stateClose = "closed"

	// PayloadPreviewLen is the max length of payload to include in error logs.
	payloadPreviewLen = 200
)

// parseHook parses a GitHub hook from an http.Request request and returns
// Repo and Pipeline detail. If a hook type is unsupported nil values are returned.
func parseHook(r *http.Request, merge bool) (_ *github.PullRequest, _ *model.Repo, _ *model.Pipeline, currCommit, prevCommit string, _ error) {
	contentType := r.Header.Get("Content-Type")
	eventType := github.WebHookType(r)

	log.Trace().
		Str("content-type", contentType).
		Str("event-type", eventType).
		Msg("parseHook: starting to parse webhook")

	// Read the body first to prevent FormValue from consuming it.
	// This handles the case where Content-Type is form-urlencoded but the body
	// is actually raw JSON (which can happen if GitHub ignores the ContentType setting).
	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Msg("parseHook: failed to read request body")
		return nil, nil, nil, "", "", err
	}

	var raw []byte
	var fromFormValue bool

	// Check if payload is in form field (for application/x-www-form-urlencoded)
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		// Parse the body as form data
		values, err := url.ParseQuery(string(rawBody))
		if err == nil && values.Get(hookField) != "" {
			raw = []byte(values.Get(hookField))
			fromFormValue = true
		}
	}

	// If we didn't get payload from form, use raw body (for application/json or other)
	if len(raw) == 0 {
		raw = rawBody
	}

	log.Trace().
		Bool("from-form", fromFormValue).
		Int("payload-len", len(raw)).
		Msg("parseHook: read payload")

	if len(raw) == 0 {
		log.Error().
			Str("content-type", contentType).
			Bool("from-form", fromFormValue).
			Msg("parseHook: empty payload received")
		return nil, nil, nil, "", "", fmt.Errorf("empty webhook payload (content-type: %s, from-form: %v)", contentType, fromFormValue)
	}

	payload, err := github.ParseWebHook(eventType, raw)
	if err != nil {
		log.Error().
			Err(err).
			Str("event-type", eventType).
			Int("payload-len", len(raw)).
			Str("payload-preview", truncatePayload(raw, payloadPreviewLen)).
			Msg("parseHook: failed to parse webhook payload")
		return nil, nil, nil, "", "", err
	}

	switch hook := payload.(type) {
	case *github.PushEvent:
		repo, pipeline, curr, prev := parsePushHook(hook)
		return nil, repo, pipeline, curr, prev, nil
	case *github.DeploymentEvent:
		repo, pipeline := parseDeployHook(hook)
		return nil, repo, pipeline, "", "", nil
	case *github.PullRequestEvent:
		pr, repo, pipeline := parsePullHook(hook, merge)
		return pr, repo, pipeline, "", "", nil
	case *github.ReleaseEvent:
		repo, pipeline := parseReleaseHook(hook)
		return nil, repo, pipeline, "", "", nil
	default:
		return nil, nil, nil, "", "", &types.ErrIgnoreEvent{Event: github.Stringify(hook)}
	}
}

// parsePushHook parses a push hook and returns the Repo and Pipeline details.
// If the commit type is unsupported nil values are returned.
func parsePushHook(hook *github.PushEvent) (_ *model.Repo, _ *model.Pipeline, curr, prev string) {
	if hook.Deleted != nil && *hook.Deleted {
		return nil, nil, "", ""
	}

	pipeline := &model.Pipeline{
		Event:    model.EventPush,
		Commit:   hook.GetHeadCommit().GetID(),
		Ref:      hook.GetRef(),
		ForgeURL: hook.GetHeadCommit().GetURL(),
		Branch:   strings.ReplaceAll(hook.GetRef(), "refs/heads/", ""),
		Message:  hook.GetHeadCommit().GetMessage(),
		Email:    hook.GetHeadCommit().GetAuthor().GetEmail(),
		Avatar:   hook.GetSender().GetAvatarURL(),
		Author:   hook.GetSender().GetLogin(),
		Sender:   hook.GetSender().GetLogin(),
	}
	repo := convertRepoHook(hook.GetRepo())

	if len(pipeline.Author) == 0 {
		pipeline.Author = hook.GetHeadCommit().GetAuthor().GetLogin()
	}
	if strings.HasPrefix(pipeline.Ref, "refs/tags/") {
		// just kidding, this is actually a tag event. Why did this come as a push
		// event we'll never know!
		pipeline.Event = model.EventTag
		// For tags, if the base_ref (tag's base branch) is set, we're using it
		// as pipeline's branch so that we can filter events base on it
		if strings.HasPrefix(hook.GetBaseRef(), "refs/heads/") {
			pipeline.Branch = strings.ReplaceAll(hook.GetBaseRef(), "refs/heads/", "")
		}
		return repo, pipeline, "", ""
	}

	return repo, pipeline, hook.GetHeadCommit().GetID(), hook.GetBefore()
}

// parseDeployHook parses a deployment and returns the Repo and Pipeline details.
// If the commit type is unsupported nil values are returned.
func parseDeployHook(hook *github.DeploymentEvent) (*model.Repo, *model.Pipeline) {
	pipeline := &model.Pipeline{
		Event:      model.EventDeploy,
		Commit:     hook.GetDeployment().GetSHA(),
		ForgeURL:   hook.GetDeployment().GetURL(),
		Message:    hook.GetDeployment().GetDescription(),
		Ref:        hook.GetDeployment().GetRef(),
		Branch:     hook.GetDeployment().GetRef(),
		Avatar:     hook.GetSender().GetAvatarURL(),
		Author:     hook.GetSender().GetLogin(),
		Sender:     hook.GetSender().GetLogin(),
		DeployTo:   hook.GetDeployment().GetEnvironment(),
		DeployTask: hook.GetDeployment().GetTask(),
	}
	// if the ref is a sha or short sha we need to manually construct the ref.
	if strings.HasPrefix(pipeline.Commit, pipeline.Ref) || pipeline.Commit == pipeline.Ref {
		pipeline.Branch = hook.GetRepo().GetDefaultBranch()
		pipeline.Ref = fmt.Sprintf("refs/heads/%s", pipeline.Branch)
	}
	// if the ref is a branch we should make sure it has refs/heads prefix
	if !strings.HasPrefix(pipeline.Ref, "refs/") { // branch or tag
		pipeline.Ref = fmt.Sprintf("refs/heads/%s", pipeline.Branch)
	}

	return convertRepo(hook.GetRepo()), pipeline
}

// parsePullHook parses a pull request hook and returns the Repo and Pipeline
// details.
func parsePullHook(hook *github.PullRequestEvent, merge bool) (*github.PullRequest, *model.Repo, *model.Pipeline) {
	action := hook.GetAction()
	if action != actionOpen && action != actionSync && action != actionClose && action != actionEdited && action != actionLabeled && action != actionUnlabeled {
		return nil, nil, nil
	}

	event := model.EventPull
	if hook.GetPullRequest().GetState() == stateClose {
		if hook.GetPullRequest().GetMerged() {
			event = model.EventPullMerged
		} else {
			event = model.EventPullClosed
		}
	}

	if action == actionEdited || action == actionLabeled || action == actionUnlabeled {
		event = model.EventPullEdited
	}

	fromFork := hook.GetPullRequest().GetHead().GetRepo().GetID() != hook.GetPullRequest().GetBase().GetRepo().GetID()

	pipeline := &model.Pipeline{
		Event:    event,
		Commit:   hook.GetPullRequest().GetHead().GetSHA(),
		ForgeURL: hook.GetPullRequest().GetHTMLURL(),
		Ref:      fmt.Sprintf(headRefs, hook.GetPullRequest().GetNumber()),
		Branch:   hook.GetPullRequest().GetBase().GetRef(),
		Message:  hook.GetPullRequest().GetTitle(),
		Author:   hook.GetPullRequest().GetUser().GetLogin(),
		Avatar:   hook.GetPullRequest().GetUser().GetAvatarURL(),
		Title:    hook.GetPullRequest().GetTitle(),
		Sender:   hook.GetSender().GetLogin(),
		Refspec: fmt.Sprintf(refSpec,
			hook.GetPullRequest().GetHead().GetRef(),
			hook.GetPullRequest().GetBase().GetRef(),
		),
		PullRequestLabels: convertLabels(hook.GetPullRequest().Labels),
		FromFork:          fromFork,
	}
	if merge {
		pipeline.Ref = fmt.Sprintf(mergeRefs, hook.GetPullRequest().GetNumber())
	}

	return hook.GetPullRequest(), convertRepo(hook.GetRepo()), pipeline
}

// parseReleaseHook parses a release hook and returns the Repo and Pipeline
// details.
func parseReleaseHook(hook *github.ReleaseEvent) (*model.Repo, *model.Pipeline) {
	if hook.GetAction() != actionReleased {
		return nil, nil
	}

	name := hook.GetRelease().GetName()
	if name == "" {
		name = hook.GetRelease().GetTagName()
	}

	pipeline := &model.Pipeline{
		Event:        model.EventRelease,
		ForgeURL:     hook.GetRelease().GetHTMLURL(),
		Ref:          fmt.Sprintf("refs/tags/%s", hook.GetRelease().GetTagName()),
		Branch:       hook.GetRelease().GetTargetCommitish(), // cspell:disable-line
		Message:      fmt.Sprintf("created release %s", name),
		Author:       hook.GetRelease().GetAuthor().GetLogin(),
		Avatar:       hook.GetRelease().GetAuthor().GetAvatarURL(),
		Sender:       hook.GetSender().GetLogin(),
		IsPrerelease: hook.GetRelease().GetPrerelease(),
	}

	return convertRepo(hook.GetRepo()), pipeline
}

// truncatePayload returns a truncated string representation of the payload for logging.
func truncatePayload(payload []byte, maxLen int) string {
	if len(payload) <= maxLen {
		return string(payload)
	}
	return string(payload[:maxLen]) + "..."
}
