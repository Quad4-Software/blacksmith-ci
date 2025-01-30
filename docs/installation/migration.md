<!-- markdownlint-disable MD041 -->

## Woodpecker

Crow CI is currently a soft-fork of Woodpecker CI.
This means it is compatible with Woodpecker in terms of

- API usage
- Plugin usage
- most configuration options

Crow CI 3.0 has support for Woodpecker-prefixed env vars (`WOODPECKER_`) which should allow a seamless migration from Woodpecker (3.0).
Moving on from 3.0, it can't be guaranteed that all new Woodpecker features and env vars will be supported or available at all.

Crow CI has it's own env vars prefixed with `CROW_`.
For now, these are identical to the Woodpecker ones.
Additional/alternate names might be introduced over time.

## Drone

As Woodpecker itself is a fork of Drone and aims to keep compatibility for Drone (mainly to allow Drone plugin usage), the same applies to Crow CI.
Though it should be kept in mind that the initial fork of Drone was in 2019 and since then Woodpecker has evolved continuously while both Drone and its plugins only received maintenance updates, if at all.
If you are still using Drone CI and are thinking about switching, now would be a good time!
