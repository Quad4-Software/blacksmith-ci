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
	"testing"

	"github.com/google/go-github/v92/github"
	"github.com/stretchr/testify/assert"

	"codefloe.com/crowci/crow/v6/server/model"
)

func Test_convertStatus(t *testing.T) {
	assert.Equal(t, statusSuccess, convertStatus(model.StatusSuccess))
	assert.Equal(t, statusPending, convertStatus(model.StatusPending))
	assert.Equal(t, statusPending, convertStatus(model.StatusRunning))
	assert.Equal(t, statusFailure, convertStatus(model.StatusFailure))
	assert.Equal(t, statusError, convertStatus(model.StatusKilled))
	assert.Equal(t, statusError, convertStatus(model.StatusError))
}

func Test_convertDesc(t *testing.T) {
	assert.Equal(t, descSuccess, convertDesc(model.StatusSuccess))
	assert.Equal(t, descPending, convertDesc(model.StatusPending))
	assert.Equal(t, descPending, convertDesc(model.StatusRunning))
	assert.Equal(t, descFailure, convertDesc(model.StatusFailure))
	assert.Equal(t, descError, convertDesc(model.StatusKilled))
	assert.Equal(t, descError, convertDesc(model.StatusError))
}

func Test_convertRepoList(t *testing.T) {
	from := []*github.Repository{
		{
			Private:  new(false),
			FullName: new("octocat/hello-world"),
			Name:     new("hello-world"),
			Owner: &github.User{
				AvatarURL: new("http://..."),
				Login:     new("octocat"),
			},
			HTMLURL:  new("https://github.com/octocat/hello-world"),
			CloneURL: new("https://github.com/octocat/hello-world.git"),
			Permissions: &github.RepositoryPermissions{
				Push:  new(true),
				Pull:  new(true),
				Admin: new(true),
			},
		},
	}

	to := convertRepoList(from)
	assert.Equal(t, "http://...", to[0].Avatar)
	assert.Equal(t, "octocat/hello-world", to[0].FullName)
	assert.Equal(t, "octocat", to[0].Owner)
	assert.Equal(t, "hello-world", to[0].Name)
}

func Test_convertRepo(t *testing.T) {
	from := github.Repository{
		FullName:      new("octocat/hello-world"),
		Name:          new("hello-world"),
		HTMLURL:       new("https://github.com/octocat/hello-world"),
		CloneURL:      new("https://github.com/octocat/hello-world.git"),
		DefaultBranch: new("develop"),
		Private:       new(true),
		Owner: &github.User{
			AvatarURL: new("http://..."),
			Login:     new("octocat"),
		},
		Permissions: &github.RepositoryPermissions{
			Push:  new(true),
			Pull:  new(true),
			Admin: new(true),
		},
	}

	to := convertRepo(&from)
	assert.Equal(t, "http://...", to.Avatar)
	assert.Equal(t, "octocat/hello-world", to.FullName)
	assert.Equal(t, "octocat", to.Owner)
	assert.Equal(t, "hello-world", to.Name)
	assert.Equal(t, "develop", to.Branch)
	assert.True(t, to.IsSCMPrivate)
	assert.Equal(t, "https://github.com/octocat/hello-world.git", to.Clone)
	assert.Equal(t, "https://github.com/octocat/hello-world", to.ForgeURL)
}

func Test_convertPerm(t *testing.T) {
	from := &github.Repository{
		Permissions: &github.RepositoryPermissions{
			Admin: new(true),
			Push:  new(true),
			Pull:  new(true),
		},
	}

	to := convertPerm(from.GetPermissions())
	assert.True(t, to.Push)
	assert.True(t, to.Pull)
	assert.True(t, to.Admin)
}

func Test_convertTeam(t *testing.T) {
	from := &github.Organization{
		Login:     new("octocat"),
		AvatarURL: new("http://..."),
	}
	to := convertTeam(from)
	assert.Equal(t, "octocat", to.Login)
	assert.Equal(t, "http://...", to.Avatar)
}

func Test_convertTeamList(t *testing.T) {
	from := []*github.Organization{
		{
			Login:     new("octocat"),
			AvatarURL: new("http://..."),
		},
	}
	to := convertTeamList(from)
	assert.Equal(t, "octocat", to[0].Login)
	assert.Equal(t, "http://...", to[0].Avatar)
}

func Test_convertRepoHook(t *testing.T) {
	t.Run("should convert a repository from webhook", func(t *testing.T) {
		from := &github.PushEventRepository{Owner: &github.User{}}
		from.Owner.Login = new("octocat")
		from.Owner.Name = new("octocat")
		from.Name = new("hello-world")
		from.FullName = new("octocat/hello-world")
		from.Private = new(true)
		from.HTMLURL = new("https://github.com/octocat/hello-world")
		from.CloneURL = new("https://github.com/octocat/hello-world.git")
		from.DefaultBranch = new("develop")

		repo := convertRepoHook(from)
		assert.Equal(t, *from.Owner.Login, repo.Owner)
		assert.Equal(t, *from.Name, repo.Name)
		assert.Equal(t, *from.FullName, repo.FullName)
		assert.Equal(t, *from.Private, repo.IsSCMPrivate)
		assert.Equal(t, *from.HTMLURL, repo.ForgeURL)
		assert.Equal(t, *from.CloneURL, repo.Clone)
		assert.Equal(t, *from.DefaultBranch, repo.Branch)
	})
}

func Test_parsePullHook(t *testing.T) {
	from := &github.PullRequestEvent{
		Action: new(actionOpen),
		PullRequest: &github.PullRequest{
			State:   new(stateOpen),
			HTMLURL: new("https://github.com/octocat/hello-world/pulls/42"),
			Number:  new(42),
			Title:   new("Updated README.md"),
			Base: &github.PullRequestBranch{
				Ref: new("main"),
			},
			Head: &github.PullRequestBranch{
				Ref: new("changes"),
				SHA: new("f72fc19"),
				Repo: &github.Repository{
					CloneURL: new("https://github.com/octocat/hello-world-fork"),
				},
			},
			User: &github.User{
				Login:     new("octocat"),
				AvatarURL: new("https://avatars1.githubusercontent.com/u/583231"),
			},
		}, Sender: &github.User{
			Login: new("octocat"),
		},
	}
	pull, _, pipeline := parsePullHook(from, true)
	assert.NotNil(t, pull)
	assert.Equal(t, model.EventPull, pipeline.Event)
	assert.Equal(t, *from.PullRequest.Base.Ref, pipeline.Branch)
	assert.Equal(t, "refs/pull/42/merge", pipeline.Ref)
	assert.Equal(t, "changes:main", pipeline.Refspec)
	assert.Equal(t, *from.PullRequest.Head.SHA, pipeline.Commit)
	assert.Equal(t, *from.PullRequest.Title, pipeline.Message)
	assert.Equal(t, *from.PullRequest.Title, pipeline.Title)
	assert.Equal(t, *from.PullRequest.User.Login, pipeline.Author)
	assert.Equal(t, *from.PullRequest.User.AvatarURL, pipeline.Avatar)
	assert.Equal(t, *from.Sender.Login, pipeline.Sender)
}

func Test_parseDeployHook(t *testing.T) {
	from := &github.DeploymentEvent{Deployment: &github.Deployment{}, Sender: &github.User{}}
	from.Deployment.Description = new(":shipit:")
	from.Deployment.Environment = new("production")
	from.Deployment.Task = new("deploy")
	from.Deployment.ID = new(int64(42))
	from.Deployment.Ref = new("main")
	from.Deployment.SHA = new("f72fc19")
	from.Deployment.URL = new("https://github.com/octocat/hello-world")
	from.Sender.Login = new("octocat")
	from.Sender.AvatarURL = new("https://avatars1.githubusercontent.com/u/583231")

	_, pipeline := parseDeployHook(from)
	assert.Equal(t, model.EventDeploy, pipeline.Event)
	assert.Equal(t, "main", pipeline.Branch)
	assert.Equal(t, "refs/heads/main", pipeline.Ref)
	assert.Equal(t, *from.Deployment.SHA, pipeline.Commit)
	assert.Equal(t, *from.Deployment.Description, pipeline.Message)
	assert.Equal(t, *from.Deployment.URL, pipeline.ForgeURL)
	assert.Equal(t, *from.Sender.Login, pipeline.Author)
	assert.Equal(t, *from.Sender.AvatarURL, pipeline.Avatar)
}

func Test_parsePushHook(t *testing.T) {
	t.Run("convert push from webhook", func(t *testing.T) {
		from := &github.PushEvent{Sender: &github.User{}, Repo: &github.PushEventRepository{}, HeadCommit: &github.HeadCommit{Author: &github.CommitAuthor{}}}
		from.Sender.Login = new("octocat")
		from.Sender.AvatarURL = new("https://avatars1.githubusercontent.com/u/583231")
		from.Repo.CloneURL = new("https://github.com/octocat/hello-world.git")
		from.HeadCommit.Author.Email = new("github.Ptr(octocat@github.com")
		from.HeadCommit.Message = new("updated README.md")
		from.HeadCommit.URL = new("https://github.com/octocat/hello-world")
		from.HeadCommit.ID = new("f72fc19")
		from.Ref = new("refs/heads/main")

		_, pipeline, _, _ := parsePushHook(from)
		assert.Equal(t, model.EventPush, pipeline.Event)
		assert.Equal(t, "main", pipeline.Branch)
		assert.Equal(t, "refs/heads/main", pipeline.Ref)
		assert.Equal(t, *from.HeadCommit.ID, pipeline.Commit)
		assert.Equal(t, *from.HeadCommit.Message, pipeline.Message)
		assert.Equal(t, *from.HeadCommit.URL, pipeline.ForgeURL)
		assert.Equal(t, *from.Sender.Login, pipeline.Author)
		assert.Equal(t, *from.Sender.AvatarURL, pipeline.Avatar)
		assert.Equal(t, *from.HeadCommit.Author.Email, pipeline.Email)
	})

	t.Run("convert tag from webhook", func(t *testing.T) {
		from := &github.PushEvent{}
		from.Ref = new("refs/tags/v1.0.0")

		_, pipeline, _, _ := parsePushHook(from)
		assert.Equal(t, model.EventTag, pipeline.Event)
		assert.Equal(t, "refs/tags/v1.0.0", pipeline.Ref)
	})

	t.Run("convert tag's base branch to pipeline's branch ", func(t *testing.T) {
		from := &github.PushEvent{}
		from.Ref = new("refs/tags/v1.0.0")
		from.BaseRef = new("refs/heads/main")

		_, pipeline, _, _ := parsePushHook(from)
		assert.Equal(t, model.EventTag, pipeline.Event)
		assert.Equal(t, "main", pipeline.Branch)
	})

	t.Run("not convert tag's base_ref from webhook if not prefixed with 'ref/heads/'", func(t *testing.T) {
		from := &github.PushEvent{}
		from.Ref = new("refs/tags/v1.0.0")
		from.BaseRef = new("refs/refs/main")

		_, pipeline, _, _ := parsePushHook(from)
		assert.Equal(t, model.EventTag, pipeline.Event)
		assert.Equal(t, "refs/tags/v1.0.0", pipeline.Branch)
	})
}
