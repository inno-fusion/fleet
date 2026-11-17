# Fleet v4.95.0 mod release

## Release layout

- Upstream: `fleet-v4.95.0` at `5790b6cafd149ad082d5a33a65e6ea016212560d`.
- Fork version branch: `v4.95.0`. Maintenance ref: `cherry`.
- Fork source tag: `v4.95.0-mod`. Do not move upstream tags or earlier `-mod` tags.
- Server image: `ghcr.io/inno-fusion/fleet:v4.95.0` and `:latest` for Linux
  ARM64 and AMD64. Use the verified immutable manifest digest for deployment.
- Server/CLI version: `4.95.0-mod`, with the exact version-branch commit embedded.
- The server binary embeds the dashboard. There is no separate frontend container.
- MySQL and Redis are dependencies, not modified applications: use maintained,
  pinned official images and persistent MySQL storage.
- Archives contain matching Linux servers and Linux/macOS/Windows `fleetctl`.
  macOS CLI archives contain a universal binary with ad-hoc signing, not an Apple
  notarization claim. Other OS binaries are cross-built, not native-OS tested.
- Include upstream MIT and EE license texts with binaries/images. The research
  opt-in does not change their terms or grant production/distribution rights.

Publication scope is private GHCR only. Do not attach binary archives or Mac
apps to releases in the public source repository. Local archives remain available
for internal evaluation. License texts are files, not UI banners or runtime gates.
The image deliberately uses the OCI `url` label rather than `source` so first
publication does not automatically inherit access from the public source repo.
Read back package visibility and confirm unauthenticated manifest access is denied
after publication. Never change this package to public as part of an update.

The image opts into the local research tier by default. Set
`FLEET_LICENSE_LOCAL_RESEARCH=false` to exercise normal signed-license/free
behavior. Do not set a signed license key simultaneously with research mode.
Device inventory, MDM operational traffic and configured integrations remain
available. This is not an air-gapped build.

## Reproduce server and CLI assets

On a clean macOS checkout with repository-supported Go/Node, Yarn, Zig, Xcode
command-line tools:

```bash
python3 tools/mod-project/build-release.py --version 4.95.0 --output "$HOME/fleet-release-output"
docker buildx build --platform linux/arm64,linux/amd64 \
  --build-arg RELEASE_VERSION=4.95.0-mod \
  --build-arg SOURCE_REVISION="$(git rev-parse HEAD)" \
  -t ghcr.io/inno-fusion/fleet:v4.95.0 \
  -t ghcr.io/inno-fusion/fleet:latest \
  --push "$HOME/fleet-release-output"
```

Server cross-builds require CGO and static musl linking for SQLite/FTS5; do not
substitute `CGO_ENABLED=0` just because that binary compiles. Fleet's vulnerability
queries use SQLite at runtime. Zig supplies architecture-matched C toolchains.
The native client components have their own upstream versions, separate from
Fleet server's 4.95.0. Do not stamp Orbit/Desktop with the server version.

The builder creates a fresh detached worktree at the committed source revision,
installs locked dependencies there, and regenerates all dashboard/bindata assets.
It never embeds caller-side untracked/ignored files or resumes arbitrary existing
generated assets. Every binary must report the exact clean VCS revision and target
build flags before packaging. The temporary source worktree is removed afterward.

## Apple components and installation boundary

`fleetctl` is the administrator CLI, not the endpoint agent. Apple MDM is native:
manual enrollment uses a server-generated configuration profile. iPhone/iPad do
not need a custom forked desktop application. On a Mac, fleetd consists of
Orbit plus osquery, with optional Fleet Desktop for end-user UI. A generic Orbit
or Desktop archive is not an enrollment installer and must not be installed
alone in place of a configured fleetd package.

Generate a real Mac installer only after the final HTTPS URL, server certificate
trust, and the chosen fleet's enrollment secret are known. Keep that installer
private: it contains enrollment configuration/secrets. Never upload a personalized
installer to a public GitHub release. Custom Orbit/Desktop distribution also
requires correctly configured signed TUF metadata/update paths or updates may
replace local components; a GitHub asset alone does not configure that channel.
Apple Developer ID signing, installer signing and notarization are distinct.
An unsigned/ad-hoc artifact must not be described as notarized.

## First deployment, then first Apple device

No deployment or enrollment is performed by publishing this release.

1. Choose a host and permanent public HTTPS name; preserve that URL and the
   server private key across restarts/upgrades. Confirm whether the first device
   is a Mac, iPhone or iPad and whether it already has another MDM profile.
2. Provision isolated MySQL/Redis and a persistent Fleet server private key.
   Run `fleet prepare db --no-prompt` using the exact image and config, then
   `fleet serve`. Mount keys/config read-only; do not place secrets in an image.
3. Complete initial admin setup, set the public Fleet URL, verify authentication,
   backups, and HTTPS certificate trust before enabling Apple MDM.
4. Use the approved Apple MDM vendor capability to issue the actual vendor
   signing certificate and its key/chain. Mount them using the documented
   `FLEET_MDM_APPLE_MDM_VENDOR_CERT`, `_KEY`, and `_CHAIN` settings.
5. Download the locally signed CSR, submit it to Apple's Push Certificates
   Portal, then upload the APNs certificate. Preserve that Apple account/topic
   and renew the same identity annually. Approval alone is not the certificate.
6. If using Apple Business Manager automated enrollment, add the proper ABM
   server token and assign the device; manual enrollment does not require ADE.
7. Enroll one disposable/pilot device. On Mac generate a fleetd package privately
   with matching `fleetctl`; inspect its URL/certificate/enrollment settings first.
   Verify MDM profile, APNs command delivery and inventory before adding policies.
   Do not test lock, wipe, disk encryption or enforced updates on the first pass.

Actual Apple portal acceptance, APNs traffic and hardware enrollment require
real credentials and a pilot device; synthetic CSR tests do not establish them.

## Future updates

Start from an explicit upstream tag and replay every fork commit by intent:
`git cherry-pick fleet-v<OLD>..origin/cherry` on a new branch made from
`fleet-v<NEW>` (`cherry` points at the deployed version branch). Never cherry-pick
only the latest commit. `build-release.py` refuses a `--version` that is not the
upstream tag the commit is built on, and records that tag and commit in
`build-provenance.json`. Keep raw license checks, privacy defaults, Android
API/background routing and non-destructive provider guards in the audit. Compare
`tools/mod-project/Dockerfile`'s base image with upstream's `tools/fleet-docker/Dockerfile`.
Rebuild/test the actual container and verify all remote branch, image and asset hashes
before moving the `cherry` ref.

Orbit and Fleet Desktop for Macs are built, signed, notarized and published to the
private TUF server separately (operator scripts outside this repository), from the
same version branch. When upstream ships a new Orbit, rebuild them from that branch
so the agent-side presentation changes reach devices.
