---
title: Plugins
hide:
  - navigation
---

Plugins are special **steps** that perform **pre-defined tasks**.  
They can be used for any kind of operation: to deploy code, publish artifacts, send notification, and more.

Plugins are particular useful for the following scenarios:

- Security-sensitive operations
- Simplification of complex tasks

They can be viewed as "command" snippets that focus on doing one specific task that would otherwise require many individual and semi-complex commands.

Plugins can be written in any language.
Preferably, a language should be used which creates a single binary as this one can then be used together with a lightweight container image, speeding up download times due to a small image footprint.

## Security-sensitive operations

When a step gets handed over sensitive secrets such as SSH keys, API tokens and others, one wants to ensure that no other operations than the desired ones do make use of these credentials.  
In a normal step, any user could open a pull request, issue an `"echo"` command on the secret and expose it that way (if the secret is allowed to be used in `pull_request` events).
While repo owners can choose to not allow injecting a particular secret in a PR, often enough they are needed to allow for proper checking of a PR.
Here is where plugins come into play:

Plugins cannot be altered in their execution: they always execute a specific set of commands, combined with a fixed image entrypoint, which can only be altered in a limited way through specific plugin settings.  
Crow allows [limiting secrets to specific plugins](../usage/secrets.md#filtering).
Thanks to this, a potential misuse/exposure of secrets in a Pull Request scenario is not possible.

## Available plugins

A variety of plugins exist which are spread across forges, users and projects.
As Crow CI has inherited the codebase from Drone CI and Woodpecker, their plugins can generally be used as well.  
While Woodpecker plugins are fully compatible, many Drone plugins still rely to hand over important config values via `environment:`, which is prohibited in Crow.

A curated set of plugins is available in the [crow-plugins](https://codeberg.org/crow-plugins) organization.  
Other popular organizations and repositories containing Woodpecker/Drone plugins include:

- [Woodpecker CI (GitHub)](https://woodpecker-ci.org/plugins)
- [Appleboy (GitHub)](https://github.com/appleboy?tab=repositories&q=drone-&type=source&language=&sort=)
- [thegeeklab (GitHub)](https://woodpecker-plugins.geekdocs.de/)
- [Woodpecker Community](https://codeberg.org/woodpecker-community)

The following table (grouped by official/others, then sorted alphabetically) gives a general overview of available plugins and their location.
It does not aim to be complete.
If you want your own plugin (or any missing one) to be listed, please create a Pull Request.

| Name                                               | Description                                               | Repository                                                                                                            |
| -------------------------------------------------- | --------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| Ansible :octicons-verified-16:{ .icon-blue }       | Execute Ansible playbooks                                 | [crow-plugins/ansible](https://codeberg.org/crow-plugins/ansible)                                                     |
| docker-buildx :octicons-verified-16:{ .icon-blue } | Build multi-arch container images via `buildx`            | [crow-plugins/docker-buildx](https://codeberg.org/crow-plugins/docker-buildx)                                         |
| -------------------------------------------------- | --------------------------------------------------------- | ------------------------------------------------------                                                                |
| BlueSky                                            | Post on BlueSky                                           | [woodpecker-plugins/bluesky-post](https://codeberg.org/woodpecker-plugins/bluesky-post)                               |
| Git Release                                        | Create releases on Git forges                             | [woodpecker-plugins/release](https://codeberg.org/woodpecker-plugins/release)                                         |
| Git SSH                                            | Push to Git repos via SSH                                 | [appleboy/drone-ssh](https://github.com/appleboy/drone-ssh)                                                           |
| GPG Artifact Sign                                  | Sign artifacts via `gpg`                                  | [thegeeklab/wp-gpgsign](https://github.com/thegeeklab/wp-gpgsign)                                                     |
| Mastodon                                           | Post on Mastodon                                          | [woodpecker-plugins/mastodon-post](https://codeberg.org/woodpecker-plugins/mastodon-post)                             |
| Matrix                                             | Send messages to Matrix                                   | [thegeeklab/wp-matrix](https://github.com/thegeeklab/wp-matrix)                                                       |
| Nix remote builder                                 | Build Nix flakes through the Nix store of a remote system | [woodpecker-community/nix-remote-builder-plugin](https://codeberg.org/woodpecker-community/nix-remote-builder-plugin) |
| OpenTofu                                           | Execute OpenTofu actions                                  | [thegeeklab/wp-opentofu](https://github.com/thegeeklab/wp-opentofu)                                                   |
| Peckify                                            | Formats code with Prettier and auto-update pull requests  | [woodpecker-community/peckify](https://codeberg.org/woodpecker-community/peckify)                                     |
| Portainer Update                                   | Trigger Portainer service updates                         | [woodpecker-community/portainer-service-update](https://codeberg.org/woodpecker-community/portainer-service-update)   |
| S3                                                 | Execute S3 actions                                        | [woodpecker-plugins/plugin-s3](https://codeberg.org/woodpecker-plugins/plugin-s3)                                     |
| S3-action                                          | Execute S3 actions                                        | [thegeeklab/wp-s3-action](https://github.com/thegeeklab/wp-s3-action)                                                 |
| Sccache                                            | Use `sccache` as compiler cache                           | [crow-plugins/sscache](https://codeberg.org/crow-plugins/sscache)                                                     |
| Surge Preview                                      | Pull request website previews via `surge.sh`              | [woodpecker-plugins/plugin-surge-preview](https://codeberg.org/woodpecker-plugins/plugin-surge-preview)               |
| Trigger                                            | Trigger pipelines                                         | [woodpecker-plugins/trigger](https://codeberg.org/woodpecker-plugins/trigger)                                         |

There are more plugins which exists, however, we cannot list all of them here.

/// important
For the time being, Crow is still using Woodpecker plugins and does not yet offer Crow-owned plugins.
This will (likely) gradually change over time.
///

When considering the use of a plugin, please do your own due diligence with respect to the plugin's overall maintenance status and functionality, especially when handing over sensitive secrets.
In the end, you are trusting the plugin and their maintainers to properly handle your sensitive secrets.

The documentation differs among plugins.
Some have it directly in the README.md of a repository, others use a `docs.md` file or a dedicated documentation website.
Consult the individual plugin repos for more information.

## Creating plugins

The plugin logic can be written in any language.
The only required point is to start the plugin commands through the `ENTRYPOINT` of the final image and to now allow altering the `CMD` statement.

### Settings

Settings is a dedicated key for plugins which should be used to pass options to a plugin.

Behind the scenes, options are converted to env vars with a `PLUGIN_` prefix.
For example, a setting named `url` is converted into the env var `PLUGIN_URL`.

The names are sanitized, i.e. names with a dash are converted to an underscore (`some-setting` becomes `PLUGIN_SOME_SETTINGS`).
CamelCase is not supported and will be converted into all uppercase.

/// important
As `environment:` is not allowed, plugin authors need to take care to provide settings for all possible values the plugin should be allowed to consume.
An alternative is to allow/add an `environment:` setting, yet this would actually allow arbitrary inputs and make the plugin less secure as it would allow for arbitrary inputs.
///

#### Simple settings

Boolean, numeric and single string inputs will all be converted into a string in the resulting env var.

- `some-bool: false` -> `PLUGIN_SOME_BOOL="false"`
- `some-String: hello` -> `PLUGIN_SOME_STRING="hello"`
- `anInteger: 3` -> `PLUGIN_ANINT="3"`

#### Complex settings

Individual settings can be composed of multiple different types:

```yaml
settings:
  complex:
    abc: 2
    list:
      - 2
      - 3
```

Such settings are first converted to a JSON representation and then passed along.
In this example, the key would `PLUGIN_COMPLEX` with a value of `{"abc": "2", "list": [ "2", "3" ]}`.

### Secrets

There's nothing special to take care of for secrets.
All inputs should be parsable via the `from_secret` syntax, similar as for normal steps.
