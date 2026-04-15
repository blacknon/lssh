Secret Providers
================

Secret providers resolve `*_ref` values at connection time.

- [`provider-secret-onepassword`](./provider-secret-onepassword/README.md)
- [`provider-secret-bitwarden`](./provider-secret-bitwarden/README.md)
- [`provider-secret-os-keychain`](./provider-secret-os-keychain/README.md)
- [`provider-secret-custom-script`](./provider-secret-custom-script/README.md)

## Role

Secret providers exist to resolve sensitive values as late as possible, ideally just before the value is needed for an actual connection or provider request.

This keeps long-lived config files readable while avoiding direct storage of every secret in plain text.

## What Secret Providers Should Do

- Resolve a reference into a usable value.
- Keep secret lookup logic localized to the provider.
- Return errors that clearly distinguish:
  - missing secret
  - access denied
  - backend unavailable
  - malformed reference

Typical resolved values include:

- passwords
- API tokens
- secret keys
- passphrases
- other connection-time sensitive fields

## What Secret Providers Should Not Do

- Discover hosts or infrastructure targets.
- Choose transport behavior for commands.
- Become a generic templating engine unrelated to secret resolution.
- Depend on broad side effects in the local environment unless that is the explicit contract of the backend.

## Timing Expectations

A secret provider should resolve values as close to use time as possible.

That design helps with:

- reducing secret exposure duration
- supporting secret rotation
- separating inventory data from auth material

## Design Notes For New Secret Providers

When adding a new secret provider, it is useful to define:

1. What the reference format looks like.
2. Which fields can use that reference.
3. Whether the backend is interactive, local-only, or network-backed.
4. What errors the user should see when the lookup fails.
5. Whether caching is safe, necessary, or undesirable.

Secret providers are easiest to use when their reference format is small, explicit, and easy to debug.
