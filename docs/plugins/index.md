<!-- markdownlint-disable MD041 -->

# Plugins

Plugins are special pipeline steps that perform **pre-defined tasks**.
They can be used for any kind of operation: to deploy code, publish artifacts, send notification, and more.

Why do plugins exist? What can they do better than just specifying commands in normal steps?

!!! information

    Plugins fulfill the following needs:

    - Secure security-sensitive operations
    - Simplify complicated tasks

## Security-sensitive commands

When handing over sensitive secrets to pipelines such as SSH keys, API tokens and others, users want to ensure that no other operations than the desired ones do make use of these credentials.

Plugins can ensure this as they cannot be altered in their execution: they always execute a specific set of commands within a predefined environment which can only be altered in a limited way through specific plugin settings.

In addition, Crow allows [limiting secrets to specific plugin images](../usage/secrets.md#filtering), avoiding the potential misuse of such in arbitrary steps.

## Convenience

Another use case it wrap steps which execute the same set of commands over and over again into a plugin.
The advantage of this is two-fold: a cleaner/shorter YAML config and a small execution speedup (as all required dependencies are already installed within the plugin image).

## Available plugins

A variety of plugins exist from different sources and projects.

!!! note

    Historically, Drone plugins still exist and should work in theory.
    However, many of these are not well maintained and might not comply anymore with the requirement of not allowing arbitrary env vars in the `environment:` section, which is enforced since Crow/Woodpecker 3.0.

A non-comprehensive list of available plugins:

- [Woodpecker CI (GitHub)](https://woodpecker-ci.org/plugins)
- [Appleboy (GitHub)](https://github.com/appleboy?tab=repositories&q=drone-&type=source&language=&sort=)
- [thegeeklab (GitHub)](https://woodpecker-plugins.geekdocs.de/)

Additional plugins can be found in the wild.
Please make sure to always take a close look at the plugin itself and the overall maintenance state, especially when handing over sensitive secrets.

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
