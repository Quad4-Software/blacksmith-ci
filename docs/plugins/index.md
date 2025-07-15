---
hide:
  - navigation
---

## <!-- markdownlint-disable MD041 -->

Plugins are special **steps** that perform **pre-defined tasks**.
They can be used for any kind of operation: to deploy code, publish artifacts, send notification, and more.

What is the scope of plugins and for which use cases should they be used?

Plugins are particular useful for the following scenarios:

- Security-sensitive operations
- Simplification of complex tasks

## Security-sensitive commands

When a step gets handed over sensitive secrets such as SSH keys, API tokens and others, one wants to ensure that no other operations than the desired ones do make use of these credentials.
In a normal step, any user could open a pull request, issue an `"echo"` command on the secret and expose it that way (if the secret is allowed to be used in `pull_request` events).
While repo owners can choose to not allow injecting a particular secret in a PR, often enough they are needed to allow for proper checking of a PR.
Here is where plugins come into play:

Plugins cannot be altered in their execution: they always execute a specific set of commands, combined with a fixed image entrypoint, which can only be altered in a limited way through specific plugin settings.
Crow allows [limiting secrets to specific plugins](../usage/secrets.md#filtering).
Thanks to this, a potential misuse/exposure of secrets in a Pull Request scenario is not possible.

## Convenience

Another use case it wrap steps which execute the same set of commands over and over again into a plugin.
The advantage of this is two-fold: a cleaner/shorter YAML config and a (small) execution speedup (as all required dependencies are already installed within the plugin image and the step can start right away if the image is already cached).

## Available plugins

A variety of plugins exist across different sources and projects.

/// note
Because Crow is a fork of Woodpecker/Drone, their respective plugins _should_ be compatible.
Drone has meanwhile diverged a bit and a few of their plugins haven't been very well maintained recently.
Woodpecker plugins should be fully compatible.
///

/// important
Some Drone plugins still require inputs via the `environment:` section, which is not allowed (anymore) in Crow for security reasons.
All inputs must be passed via explicit plugin settings.
If this is the case, the Drone plugin in question cannot be used within Crow.
///

Here is a non-comprehensive list of organizations and repos containing Woodpecker/Drone plugins:

- [Woodpecker CI (GitHub)](https://woodpecker-ci.org/plugins)
- [Appleboy (GitHub)](https://github.com/appleboy?tab=repositories&q=drone-&type=source&language=&sort=)
- [thegeeklab (GitHub)](https://woodpecker-plugins.geekdocs.de/)
- [Woodpecker Community](https://codeberg.org/woodpecker-community)

There are more plugins which exists, however, we cannot list all of them here.

/// important
For the time being, Crow is still using Woodpecker plugins and does not yet offer Crow-owned plugins.
This will (likely) gradually change over time.
///

When considering the use of a plugin, please do your own due diligence with respect to the plugin's overall maintenance status and functionality, especially when handing over sensitive secrets.
In the end, you are trusting the plugin and their maintainers to properly handle your sensitive secrets.

## Creating plugins

The plugin logic can be written in any language.
The only required point is to start the plugin commands through the `ENTRYPOINT` of the final image and to now allow altering the `CMD` statement.

### Settings

Settings is a dedicated key for plugins which should be used to pass options to a plugin.

Behind the scenes, options are converted to env vars with a `PLUGIN_` prefix.
For example, a setting named `url` is converted into the env var `PLUGIN_URL`.

The names are sanitized, i.e. names with a dash are converted to an underscore (`some-setting` becomes `PLUGIN_SOME_SETTINGS`).
CamelCase is not supported and will be converted into all uppercase.

!!! important

    As `environment:` is not allowed, plugin authors need to take care to provide settings for all possible values the plugin should be allowed to consume.
    An alternative is to allow/add an `environment:` setting, yet this would actually allow arbitrary inputs and make the plugin less secure as it would allow for arbitrary inputs.

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
