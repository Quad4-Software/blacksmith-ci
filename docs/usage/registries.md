---
hide:
  - toc
---

<!-- markdownlint-disable MD041 -->

Container registry credentials can be added in the repository settings.
This allows to authenticate against the respective registry and pull private images while also avoiding rate-limiting.

Registry hostnames are matched to each image in the YAML config.
If the hostname matches, the stored registry credentials are used to authenticate and pull the image.

/// important
Credentials are used by the agent and are never exposed to build containers.
///

/// info
The implicit default when not specifying a registry is always `docker.io`.
///

Examples:

- Image `quay.io/foo/bar` has hostname `gcr.io`
- Image `foo/bar` has hostname `docker.io`
- Image `qux.com:8000/foo/bar` has hostname `qux.com:8000`

Hostname matching logic:

- Hostname `quay.io` matches `image: quay.io/foo/bar`
- Hostname `docker.io` matches `image: golang`
- Hostname `docker.io` matches `image: library/golang`
- Hostname `docker.io` matches `image: bradrydzewski/golang`
- Hostname `docker.io` matches `image: bradrydzewski/golang:latest`
