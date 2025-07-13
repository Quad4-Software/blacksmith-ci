<!-- markdownlint-disable MD041 -->
# Advanced

/// note
Also have a look at the ["Advanced Usage"](https://woodpecker-ci.org/docs/usage/advanced-usage) section from the Woodpecker CI documentation.
///

## Rerunning pipeline locally

The JSON representation of a pipeline can be downloaded via the API.
This allows to rerun the pipeline locally using the CLI.
In certain cases this can help in debugging a failed pipeline, for example if issues cannot easily be reproduced.

/// important
The JSON metadata file does not contain sensitive content like secrets.
In addition, volumes are not reflected and can obviously not be remounted with the same content when rerunning locally.
///

/// note
This feature is quite unstable and has many pitfalls.
Use with caution and expect unexpected behavior.
///
