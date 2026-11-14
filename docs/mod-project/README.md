# 0xMetaLabs Fleet research fork

This branch is for internal development, evaluation, and testing. It does not grant
production, distribution, subscription, or hosted-service rights. Upstream MIT and
EE license files remain authoritative and unchanged. Apple MDM vendor approval is
not a Fleet license or Google Android Management API approval.

## Baseline and activation

- Branch: `mod-project`.
- Upstream release: `fleet-v4.94.1`.
- Base commit: `7e7d17b75abd7a81f8320ca75531b47f0933bc1b`.
- Enable local research features: `FLEET_LICENSE_LOCAL_RESEARCH=true`.
- Leave `FLEET_LICENSE_KEY` unset in research mode. No signed license is invented.
- Do not use `--dev` or `--dev_license` to activate this fork.

The research entitlement follows the server's central Premium checks and the UI's
shared feature helper. It has no trial expiry or licensed-host quota and reports
itself as research, not as a paid subscription. Authentication, role permissions,
enrollment trust, certificate validation, and integration configuration still apply.
Making a feature available never automatically locks/wipes devices, enforces disk
encryption, or schedules OS updates. Experimental safety switches remain opt-in.

## Analytics boundary

Fleet product-usage analytics collection and upload are disabled independently of
license tier. Configuration/API/UI must consistently show disabled, including old
installations with analytics previously enabled. No installation newsletter signup
is sent through that analytics path.

Preserved: device inventory and check-ins, osquery results, MDM commands/status,
audit history, Apple APNs, Android Management API and Pub/Sub, app downloads,
vulnerability feeds, and operator-selected metrics, traces, logs, and Sentry.
Default browser avatars, vendor image defaults, email presentation resources,
and agent dialog images no longer cause unsolicited vendor requests. Explicit
operator-selected images remain supported. Marketing-site analytics under
`website/` are not part of the self-hosted dashboard.
This branch does not deploy Fleet's marketing website.

## Apple without Fleet's CSR signer

After Apple issues the actual MDM vendor certificate (approval alone is not the
certificate), mount these files read-only into the Fleet process:

```yaml
mdm:
  apple_mdm_vendor_cert: /run/secrets/mdm-vendor.pem
  apple_mdm_vendor_key: /run/secrets/mdm-vendor.key
  apple_mdm_vendor_chain: /run/secrets/apple-vendor-chain.pem
```

Equivalent environment variables use `FLEET_MDM_` plus the uppercase key. The key
must be unencrypted PKCS1 or PKCS8 PEM protected by secret-file permissions. The
chain contains the issuing Apple intermediate(s) followed by the Apple root, in
order, without duplicating the vendor leaf. Use Apple's official certificate
sources and verify the actual issued certificate's chain; do not use test CAs.

The existing Download CSR UI/GET API signs locally and keeps the original database
APNs private key for renewals. Upload the downloaded request to Apple's Push
Certificates Portal, then upload the resulting APNs certificate through the normal
Fleet UI. If any local signing setting is supplied, signing errors fail closed:
there is no automatic fallback to Fleet's hosted signer. The obsolete POST/email
CSR endpoint is intentionally unavailable in local-signing mode; use the GET flow.

Local cryptographic tests use explicitly synthetic CA chains; they do not prove
Apple portal acceptance. Renew the same APNs certificate using the same Apple
account/topic. Changing push identities can require device re-enrollment.

## Apple app metadata

Upstream already supports direct Apple app metadata using:

```yaml
mdm:
  apple_vpp_app_metadata_api_bearer_token: "<valid Apps and Books developer JWT>"
```

Use Apps and Books for Organizations credentials from the MDM-enabled developer
team, not an arbitrary App Store Connect API key. Upload the organization's Apps
and Books content token normally. The supplied JWT is static: arrange secure
rotation and restart/reload before expiry. Do not commit JWTs, keys or content
tokens. Without direct credentials, upstream's Fleet metadata proxy remains the
default and independently requires legitimate remote service authorization.

## Android direct Google access or own proxy

Direct mode (requires an authorized Google Cloud project and service account):

```yaml
mdm:
  android_google_service_credentials: /run/secrets/android-service-account.json
```

Alternative, independently operated compatible proxy:

```yaml
mdm:
  android_proxy_url: https://mdm.example.com/api/android/
```

These settings are mutually exclusive and available without global developer mode.
Invalid explicit configuration aborts initialization rather than silently routing
back to Fleet. Direct configuration requires a service-account JSON with project
ID. Project APIs, quotas, IAM, Pub/Sub, organization binding, and Google's permissible
usage/approval are still required. Existing Android enterprises belong to their
original project/provider: switching a setting is not a migration. All three
Android background jobs use the selected configuration and isolated authenticated
clients, verifying the existing enterprise before any reconciliation. An
unverified absence fails closed without deleting enrollment or resetting devices.
Automatic cleanup after external enterprise deletion is deliberately removed; an
authorized administrator must explicitly disconnect after verifying the original
provider/credentials.

Configure the standard Android integration/enrollment UI after the backend is
ready. Google Pub/Sub notifications must reach the public HTTPS callback. Upstream's
direct client callback uses the existing token validation path; do not expose it
without that protection. This fork does not claim Google provider approval.
Google's permissible-usage rules also apply; owning a service account is not
proof that an exclusively first-party internal deployment is eligible.
If neither override is set, the normal Fleet Android proxy remains in use.

## Hosted-service limits

Local research entitlements do not authorize Fleet's remote Apple metadata or
other access-controlled APIs. Hosted AI endpoints differ: the self-hosted SQL
policy-description endpoint is publicly callable upstream, while the website's
query generator has its own account controls. Local research changes neither
remote policy; it is not a blanket subscription for hosted AI. Agent update/CDN downloads,
Fleet-maintained app catalogs, GeoIP provisioning, certificate authorities,
identity providers, and other integrations have their own service/credential
requirements. Their operational traffic is not analytics and is not blindly
blocked. This branch is not an air-gapped distribution or proof of zero Fleet-owned
network dependencies. Configure independent services and verify egress before
making that claim.

## Verification and updates

Use the commands and results in `docs/mod-project/VERIFICATION.md`. The dedicated
CI workflow only tests/builds; it does not publish containers or deploy devices.

For future upstream updates:

1. Fetch an explicit stable upstream release and record its immutable commit.
2. Create a temporary update branch from the new release. Replay both mod-project
   commits above the recorded base by intent (see the exact range command in
   `VERIFICATION.md`); never force-push `main` or cherry-pick only the branch tip.
3. Review changes to `LicenseInfo`, license loading/context middleware, frontend
   tier comparisons, telemetry configuration/collection/sending, new capability
   gates, and all Fleet-hosted API calls. New features must not silently fall out
   of the research entitlement or re-enable vendor analytics.
4. Run targeted tests with research unset, then end-to-end server smoke checks
   with research explicitly enabled. Check unauthorized routes still return
   authorization errors. Test old analytics=true config and zero uploader egress.
5. Build server/frontend and exercise disposable Mac/Android enrollment, profile,
   inventory and app workflows with legitimate platform credentials. Do not test
   wipe/lock on production devices. Check certificate and JWT renewal behavior.
6. Record untested external requirements, review the diff, and push only the
   approved `mod-project` ref after verifying the remote SHA.

Do not copy `research` into a signed Fleet token, patch hosted authorization,
remove upstream license notices, or equate a local unit test with a real device
integration test.
