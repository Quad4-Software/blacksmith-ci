---
hide:
  - navigation
  # - toc
---

<!-- markdownlint-disable MD041 -->

Crow CI is a _Continuous Integration & Continuous Delivery_ (CI/CD)[^1] application.  
It is designed to be **lightweight**, **fast** and **extensible**.

## Features

- [x] FLOSS License (Apache 2.0)
- [x] 150 MB Memory requirement
- [x] Container and Kubernetes backends
- [x] Multi-forge support (GitHub, GitLab, Forgejo, Gitea, Bitbucket)
- [x] SQLite, Postgres and MySQL support
- [x] Flexible agent handling (Global, org, user)
- [x] DAG pipeline visualization
- [x] OAuth2 support
- [x] Extension/Plugin system
- [x] Integrated secret storage
- [x] Maintenance panel

Coming soon:

- [ ] Multi-forge Login support
- [ ] HA for Crow Server
- [ ] External secret store integration

## Why self-host CI/CD?

There are many CI/CD services these days offered for free.
As with all hosted options which are provided for free, there are limitations and (potential) downsides/restrictions.
Self-hosting offers ways to get rid of these.

Specifically, the following limitations of public CI offerings can be tackled with a self-hosted Crow CI instance:

- Limited build times
- Slow runners
- Non or limited availability of specific architectures/platforms
- Storing (sensitive) secrets on remote platforms/services
- Being platform/forge bound

## Why use Crow CI over other options?

Crow CI gives you **full control** over the selection of servers which are processing your builds, being it the architecture, the size or their location (on-prem, cloud).
There is no limit to the number of builds you can run.
You are in full control!  
In addition, Crow CI is compatible with all common forges (GitHub, GitLab, Gitea, Forgejo, Bitbucket), allowing you to transition between these or use a consistent CI syntax across all of them.

Combined with the [Crow CI Autoscaler](configuration/autoscaler.md), which allows to dynamically add new servers with agents in case all existing ones are busy, you can make flexible use of powerful hardware from cloud providers _only_ when you really need it.
This keeps costs at a minimum while you retain full flexibility within the cloud provider space.

## Containers at the core

Crow CI is a _container-only_ application.
This stands in contrast to other common CI/CD apps like [GitHub Actions](https://docs.github.com/en/actions), [GitLab Runner](https://docs.gitlab.com/runner/), [Jenkins](https://www.jenkins.io/), [Circle CI](https://circleci.com/) or [Drone CI](https://www.drone.io/).
While these can also execute pipelines in containers if desired, their default mode is "host-first", i.e., pipelines typically run directly on a virtual machine and are therefore limited by the operating system of that VM..

In contrast, Crow CI is built to use containers exclusively, making it inherently operating-system agnostic.
This means the underlying OS of the host machine running the pipelines is irrelevant.
The only requirement is that the host must be running Linux.

## Target audience

Due to Crow CI being Apache-2.0 licensed, lightweight and having a very small resource footprint (< 200 MB memory consumption), it is an ideal choice for homelab environments.  
Yet also in corporate environments, Crow CI is an ideal tool to save a lot of money on paying hosted CI/CD services and easily make use of internal CI servers to process builds.
Crow CI can be hosted via common container deployment approaches and also provides a dedicated helm chart for Kubernetes environments.

Crow uses a SQLite by default (perfect for small instances) and has support for Postgres and MySQL to serve larger instances.  
The latter are recommended for instances with multiple (active) users and > 20 active repos[^2].

## History

Crow has been forked from [Woodpecker CI](https://woodpecker-ci.org/) (v3.0.0) in January 2025 from the former Woodpecker maintainer [@pat-s](https://github.com/pat-s).
Motivation for the fork was built around the improvement of infrastructure-related processes (releases, docs, governance) and professionalizing the project ecosystem.

Woodpecker itself is a fork of Drone CI (v0.8.91, April 2019) with the first standalone version being released on September 9th 2019.

### Crow vs. Woodpecker

/// note
This list is provided for informational purposes only and is not intended to devalue Woodpecker.
In fact, Crow welcomes any improvements that are ported to Woodpecker and aims for a harmonious collaboration
///

As of 2025-06, the following unique features exist in Crow:

- Log retention: Each repository can define its own retention policy. A global default of 90 days is used
- DAG support
- Maintenance panel to perform period cleanup operations on the database
- Descriptive error message: Errors are transparently parsed back to the user in the UI. Almost no "An unknown error occurred" messages are shown
- Multi-org agents: Admins can deploy agents which are allowed to process builds from multiple organizations/users
- Agent list: All instance agents are listed in the UI, also for non-admin users.
- New `pull_request_edited` event trigger
- Early fail when "manual" event triggers are not allowed in the repo: Instead of silently returning nothing, Crow shows a descriptive error message to the user
- The agent ID a specific workflow was processed by is shown in the top panel of the log view
- Custom agent labels are shown in the agent list
- Added `CI_COMMIT_PULL_REQUEST_TITLE` env var
- Manual pipeline runs show possible extra variables that have been passed during the UI
- Log view: Top/Down buttons to jump to the top or bottom of the log view
- Agents display their remaining workflow capacity in the agent list
- Added support for multiple match types when specifying `CROW_PLUGINS_PRIVILEGED`: `semver`, `semver-range`, `match` and `regex`
- Ability to cancel single workflows in matrix workflows
- Improved mobile layout view (spacing, alignment)
- Ability to fold the workflow panel and have a full-width log view
- Support for setting custom workflow names for single and matrix pipelines via the `name:` field

Additionally, for the Crow Autoscaler:

- Ability to account for static agents: only deploy new (autoscaled) agents if static agents cannot process the queued builds

## License

Crow CI is licensed under the Apache 2.0 license (following inheritance from Woodpecker and Drone).

<!-- markdownlint-disable -->

[^1]: [This RedHat blog post explains the concept of CI/CD in more detail.](https://www.redhat.com/en/topics/devops/what-is-ci-cd)

[^2]: This is primarily because Crow (still) stores pipeline logs in the DB. A refactoring to storing these outside the DB by default is planned but not yet implemented.

<!-- markdownlint-enable -->

## Logo

The logo was designed and kindly contributed by [Bold Crow AI](https://boldcrow.ai/).
