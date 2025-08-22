<!-- markdownlint-disable MD041 -->

# CLI

The CLI can be installed through the provided assets of the respective releases on Codeberg.

No binaries from HEAD are being built.
However, container images are provided for selected commits during development, which might aid as a possible workaround for urgent bug fixes between releases.

/// info
A homebrew tap for simplified installation on macOS is planned.
///

## Naming convention

The container image is named `crow-cli` to align it with the image names of the server and agent and avoid confusion.
However, the CLI executable itself is named `crow`.
This is also the entrypoint of the container image.

## Container

Here is an exemplary use of the CLI's container image:

```sh
docker run --pull always -e CROW_TOKEN=$CROW_TOKEN -e CROW_SERVER=$CROW_SERVER --rm -it codeberg.org/crowci/crow-cli:dev info
```

/// note
This assumes that the environment variables `CROW_TOKEN` and `CROW_SERVER` are set.
///
