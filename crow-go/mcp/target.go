// Copyright 2026 Crow CI Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"codefloe.com/crowci/crow/v6/crow-go/crow"
)

// discoveryPageSize is how many recent pipelines are scanned when matching a
// git checkout to its pipelines.
const discoveryPageSize = 50

// shortSHALen is the abbreviated commit length used in messages.
const shortSHALen = 7

// WebURL is a parsed Crow web UI pipeline URL.
type WebURL struct {
	Host    string
	Root    string
	RepoID  int64
	Number  int64
	StepPID int
}

// ParseWebURL parses https://<host>[/<root>]/repos/<repoId>/pipeline/<number>[/<stepPID>].
func ParseWebURL(raw string) (WebURL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return WebURL{}, fmt.Errorf("not a Crow pipeline URL: %q", raw)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	idx := slices.Index(parts, "repos")
	if idx < 0 || len(parts) < idx+4 || parts[idx+2] != "pipeline" {
		return WebURL{}, fmt.Errorf("not a Crow pipeline URL (want .../repos/<id>/pipeline/<number>): %q", raw)
	}

	w := WebURL{Host: u.Hostname()}
	if idx > 0 {
		w.Root = "/" + strings.Join(parts[:idx], "/")
	}
	if w.RepoID, err = strconv.ParseInt(parts[idx+1], 10, 64); err != nil {
		return WebURL{}, fmt.Errorf("invalid repo id in URL %q", raw)
	}
	if w.Number, err = strconv.ParseInt(parts[idx+3], 10, 64); err != nil {
		return WebURL{}, fmt.Errorf("invalid pipeline number in URL %q", raw)
	}
	if len(parts) > idx+4 {
		// Non-numeric segments are sub-views such as "graph" or "config".
		if pid, err := strconv.Atoi(parts[idx+4]); err == nil {
			w.StepPID = pid
		}
	}
	return w, nil
}

// WebLink builds the web UI link for a pipeline, or a step if stepPID > 0.
// An empty root falls back to the path of the instance URL.
func WebLink(inst Instance, root string, repoID, number int64, stepPID int) string {
	base := strings.TrimRight(inst.URL, "/")
	if root != "" {
		if u, err := url.Parse(base); err == nil {
			u.Path = strings.TrimRight(root, "/")
			base = u.String()
		}
	}
	link := fmt.Sprintf("%s/repos/%d/pipeline/%d", base, repoID, number)
	if stepPID > 0 {
		link += fmt.Sprintf("/%d", stepPID)
	}
	return link
}

// TargetInput is the argument block every pipeline-scoped tool accepts.
type TargetInput struct {
	URL      string `json:"url,omitempty" jsonschema:"Crow web URL of a pipeline or step, e.g. https://ci.example.com/repos/3/pipeline/12/4"`
	Instance string `json:"instance,omitempty" jsonschema:"configured instance name; use together with repo"`
	Repo     string `json:"repo,omitempty" jsonschema:"repository id (preferred) or owner/name"`
	Number   int64  `json:"number,omitempty" jsonschema:"pipeline number; defaults to the latest"`
}

// Remote is one git remote of a checkout.
type Remote struct {
	Name     string
	Host     string
	FullName string
}

// Checkout describes the local git checkout used for discovery.
type Checkout struct {
	Remotes []Remote
	Branch  string
	Commit  string
}

// Discoverer inspects the local working directory.
type Discoverer interface {
	Discover(ctx context.Context) (*Checkout, error)
}

// Resolved is the outcome of resolving a TargetInput.
type Resolved struct {
	Instance  Instance
	Root      string
	Client    crow.Client
	Repo      *crow.Repo
	Pipelines []*crow.Pipeline
	StepPID   int
	Note      string

	// FromCheckout is set when the target came from git discovery.
	FromCheckout bool
	// MatchedHead reports whether discovery found pipelines for HEAD.
	MatchedHead bool
}

// Link returns the web link for pipeline p of the resolved repo.
func (r *Resolved) Link(p *crow.Pipeline, stepPID int) string {
	return WebLink(r.Instance, r.Root, r.Repo.ID, p.Number, stepPID)
}

// Resolve turns a TargetInput into a repo and its matching pipelines.
func (r *Registry) Resolve(ctx context.Context, in TargetInput, d Discoverer) (*Resolved, error) {
	res, err := r.ResolveRepo(ctx, in, d)
	if err != nil {
		return nil, err
	}
	// Discovery may legitimately find nothing yet (a fresh push); callers
	// report that rather than silently showing an unrelated pipeline.
	if len(res.Pipelines) > 0 || res.FromCheckout {
		return res, nil
	}

	var p *crow.Pipeline
	if in.Number > 0 {
		p, err = res.Client.Pipeline(res.Repo.ID, in.Number)
	} else {
		p, err = res.Client.PipelineLast(res.Repo.ID, crow.PipelineLastOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("get pipeline: %w", err)
	}
	res.Pipelines = []*crow.Pipeline{p}
	return res, nil
}

// ResolveRepo resolves the repository. For URLs and discovery it also fills
// Pipelines, since those carry the pipeline identity; explicit instance+repo
// input leaves Pipelines empty.
func (r *Registry) ResolveRepo(ctx context.Context, in TargetInput, d Discoverer) (*Resolved, error) {
	switch {
	case in.URL != "":
		return r.resolveURL(in.URL)
	case in.Instance != "" || in.Repo != "":
		return r.resolveExplicit(in)
	case d != nil:
		return r.resolveCheckout(ctx, d)
	default:
		return nil, errors.New("no target: pass url, or instance and repo")
	}
}

func (r *Registry) resolveURL(raw string) (*Resolved, error) {
	w, err := ParseWebURL(raw)
	if err != nil {
		return nil, err
	}
	inst, ok := r.ByHost(w.Host)
	if !ok {
		return nil, fmt.Errorf("host %q is not a configured instance; refusing to contact it", w.Host)
	}
	client, err := r.Client(inst.Name)
	if err != nil {
		return nil, err
	}
	repo, err := client.Repo(w.RepoID)
	if err != nil {
		return nil, fmt.Errorf("get repo %d: %w", w.RepoID, err)
	}
	p, err := client.Pipeline(w.RepoID, w.Number)
	if err != nil {
		return nil, fmt.Errorf("get pipeline %d: %w", w.Number, err)
	}
	return &Resolved{
		Instance:  inst,
		Root:      w.Root,
		Client:    client,
		Repo:      repo,
		Pipelines: []*crow.Pipeline{p},
		StepPID:   w.StepPID,
	}, nil
}

func (r *Registry) resolveExplicit(in TargetInput) (*Resolved, error) {
	if in.Instance == "" || in.Repo == "" {
		if len(r.instances) != 1 || in.Repo == "" {
			return nil, errors.New("pass both instance and repo")
		}
		in.Instance = r.instances[0].Name
	}
	inst, ok := r.ByName(in.Instance)
	if !ok {
		return nil, fmt.Errorf("unknown instance %q", in.Instance)
	}
	client, err := r.Client(inst.Name)
	if err != nil {
		return nil, err
	}
	repo, err := lookupRepo(client, in.Repo)
	if err != nil {
		return nil, err
	}
	return &Resolved{Instance: inst, Client: client, Repo: repo}, nil
}

func lookupRepo(client crow.Client, s string) (*crow.Repo, error) {
	if id, err := strconv.ParseInt(s, 10, 64); err == nil {
		repo, err := client.Repo(id)
		if err != nil {
			return nil, fmt.Errorf("get repo %d: %w", id, err)
		}
		return repo, nil
	}
	repo, err := client.RepoLookup(s)
	if err != nil {
		return nil, fmt.Errorf("look up repo %q: %w", s, err)
	}
	return repo, nil
}

// orderRemotes returns upstream first, then origin, then the rest in order.
func orderRemotes(remotes []Remote) []Remote {
	order := []string{"upstream", "origin"}
	rank := func(name string) int {
		if i := slices.Index(order, name); i >= 0 {
			return i
		}
		return len(order)
	}
	out := slices.Clone(remotes)
	slices.SortStableFunc(out, func(a, b Remote) int { return rank(a.Name) - rank(b.Name) })
	return out
}

func (r *Registry) resolveCheckout(ctx context.Context, d Discoverer) (*Resolved, error) {
	co, err := d.Discover(ctx)
	if err != nil {
		return nil, fmt.Errorf("inspect git checkout: %w", err)
	}

	var (
		res      *Resolved
		hosts    []string
		firstErr error
		anyMatch bool
	)
	for _, rem := range orderRemotes(co.Remotes) {
		hosts = append(hosts, rem.Host)
		inst, ok := r.ByForgeHost(rem.Host)
		if !ok {
			continue
		}
		anyMatch = true
		client, err := r.Client(inst.Name)
		if err != nil {
			return nil, err
		}
		repo, err := lookupRemoteRepo(client, rem)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("look up repo %q on %s: %w", rem.FullName, inst.Name, err)
			}
			continue
		}
		res = &Resolved{Instance: inst, Client: client, Repo: repo}
		break
	}
	if res == nil {
		if !anyMatch {
			return nil, fmt.Errorf("no configured instance serves the git remotes (%s); configured forge hosts: %s",
				strings.Join(slices.Compact(hosts), ", "), strings.Join(r.ForgeHosts(), ", "))
		}
		return nil, firstErr
	}

	list, err := res.Client.PipelineList(res.Repo.ID, crow.PipelineListOptions{
		ListOptions: crow.ListOptions{PerPage: discoveryPageSize},
	})
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}

	// Keep the newest pipeline per event for HEAD: a restart creates a new
	// pipeline on the same commit and supersedes the older run.
	newest := map[string]*crow.Pipeline{}
	var events []string
	for _, p := range list {
		if co.Commit == "" || p.Commit != co.Commit {
			continue
		}
		prev, ok := newest[p.Event]
		if !ok {
			events = append(events, p.Event)
		}
		if !ok || p.Number > prev.Number {
			newest[p.Event] = p
		}
	}
	var matched []*crow.Pipeline
	for _, ev := range events {
		matched = append(matched, newest[ev])
	}
	res.FromCheckout = true
	res.MatchedHead = len(matched) > 0
	switch {
	case res.MatchedHead:
		res.Note = "matched HEAD " + shortSHA(co.Commit)
	case co.Branch == "":
		// Detached HEAD: there is no branch, and pipelines without one (tags)
		// are unrelated to this checkout.
		res.Note = fmt.Sprintf("no pipeline for HEAD %s yet (detached HEAD)", shortSHA(co.Commit))
	default:
		matched = latestForBranch(list, co.Branch)
		res.Note = fmt.Sprintf("no pipeline for HEAD %s yet; showing latest for branch %s", shortSHA(co.Commit), co.Branch)
	}
	sortFailedFirst(matched)
	res.Pipelines = matched
	return res, nil
}

// lookupRemoteRepo finds the repo for a git remote. A name lookup can return
// a stale duplicate from another forge with the same owner/name, so a result
// whose forge host differs from the remote's is replaced by the matching
// active repo from the user's repo list.
func lookupRemoteRepo(client crow.Client, rem Remote) (*crow.Repo, error) {
	repo, err := client.RepoLookup(rem.FullName)
	if err == nil && repoOnHost(repo, rem.Host) {
		return repo, nil
	}

	repos, listErr := client.RepoList(crow.RepoListOptions{})
	if listErr != nil {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%q resolves to a repo on %s, not %s (a stale duplicate?), and the repo list is unavailable (%w); pass url or a numeric repo id instead",
			rem.FullName, forgeHost(repo), rem.Host, listErr)
	}
	for _, r := range repos {
		if r.IsActive && strings.EqualFold(r.FullName, rem.FullName) && repoOnHost(r, rem.Host) {
			return r, nil
		}
	}
	if err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("repo %q is not active for forge %s", rem.FullName, rem.Host)
}

func forgeHost(repo *crow.Repo) string {
	u, err := url.Parse(repo.ForgeURL)
	if err != nil {
		return repo.ForgeURL
	}
	return u.Hostname()
}

// repoOnHost reports whether repo belongs to the forge at host. Repos without
// a forge URL are assumed to match.
func repoOnHost(repo *crow.Repo, host string) bool {
	if repo.ForgeURL == "" {
		return true
	}
	u, err := url.Parse(repo.ForgeURL)
	return err == nil && strings.EqualFold(u.Hostname(), host)
}

// latestForBranch returns the newest push pipeline on branch and the newest
// pull-request pipeline whose source branch is branch. For PR events Crow
// stores the target branch in Branch and "source:target" in Refspec.
func latestForBranch(list []*crow.Pipeline, branch string) []*crow.Pipeline {
	var push, pr *crow.Pipeline
	for _, p := range list {
		switch {
		case isPREvent(p.Event):
			src, _, _ := strings.Cut(p.Refspec, ":")
			if pr == nil && src == branch {
				pr = p
			}
		case p.Branch == branch:
			if push == nil {
				push = p
			}
		}
	}
	var out []*crow.Pipeline
	for _, p := range []*crow.Pipeline{pr, push} {
		if p != nil {
			out = append(out, p)
		}
	}
	return out
}

func isPREvent(event string) bool {
	return strings.HasPrefix(event, "pull_request")
}

func isFailed(status string) bool {
	return status == crow.StatusFailure || status == crow.StatusError || status == crow.StatusKilled
}

func sortFailedFirst(ps []*crow.Pipeline) {
	slices.SortStableFunc(ps, func(a, b *crow.Pipeline) int {
		fa, fb := isFailed(a.Status), isFailed(b.Status)
		switch {
		case fa && !fb:
			return -1
		case fb && !fa:
			return 1
		default:
			return 0
		}
	})
}

func shortSHA(s string) string {
	if len(s) > shortSHALen {
		return s[:shortSHALen]
	}
	return s
}
