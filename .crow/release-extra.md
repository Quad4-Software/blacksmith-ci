> [!CAUTION]
> **Security release — cross-forge account takeover (Critical, CVSS 9.8)**
>
> Crow 5.2.0 through 6.3.0 resolved the local account after forge login without scoping
> the lookup to the authenticating forge. On an instance with more than one forge
> configured, anyone able to register on any connected forge could sign in as an
> arbitrary user, including administrators. `CROW_OPEN=false` did not prevent this.
>
> **Upgrade immediately.** The fix stops new takeovers but does not undo one that already
> succeeded, so also work through the detection query in the advisory:
> [security/advisories/cross-forge-account-takeover.md](https://codefloe.com/crowci/crow/src/tag/v6.4.0/security/advisories/cross-forge-account-takeover.md)
