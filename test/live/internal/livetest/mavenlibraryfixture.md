# Real Maven library: Forgejo stable signed-file acceptance

Run inside the existing Forge Lab session using the engine's current focused
driver. The existing `LAB_TARGETS_FILE`, runner configuration and
`FORGE_CI_LIVE_SIGNER_IMAGE` apply. No separate SDK, host Maven execution or
project signing configuration is needed.

Set `FORGE_CI_LIVE_CONSUMER` to the absolute clean original library checkout.
Set `FORGE_CI_LIVE_CONSUMER_SETTINGS` to newline-delimited settings:

```text
version=2.7.0
MAVEN_LIBRARY_IMAGE=<runtime Maven/JDK21 image pinned with @sha256:...>
MAVEN_LIBRARY_PROFILE=central-release
RELEASE_PLATFORMS=linux/amd64
```

Then run the existing driver from the engine checkout:

```bash
FORGE_CI_LIVE_CONSUMER_VISIBILITY=private \
FORGE_CI_LIVE_SNAPSHOT_EVIDENCE_DIR=/absolute/external/evidence \
bash scripts/ci/run-live-tests.sh focused '^TestForgejoMavenLibraryConsumer_Stable$'
```

Choose an absolute evidence directory outside both the engine and consumer
checkouts. The shared harness creates missing directories privately and retains
each failed runner job's complete log (up to 8 MiB) before scratch teardown.
Target credentials and supplied fixture secrets are redacted before writing;
the console keeps its 4,000-byte tail and prints the retained file's path.
`FORGE_CI_LIVE_SNAPSHOT_EVIDENCE` remains the fallback environment variable.

The filter runs one scratch lane through the ordinary focused profile. Set
`FORGE_CI_LIVE_CONSUMER_VISIBILITY=private` for a private consumer, or `public`
for a public consumer. Unset or `both` retains this case's single public lane;
it does not run a public/private matrix. The engine-serving repository remains
public under the existing `ReadLayerAnonymously` infrastructure exception;
only the consumer's visibility follows the selector. The local staging index
remains private. It admits the clean source through
`OpenConsumer`, overlays native CI and changelog files only in its private Git
index, and pushes a parentless fixture commit. The source POM, Java, quality
rules and test gates remain the admitted originals. `PlanConsumer` rebinds every
native action/workflow reference to the engine under test. The existing trial
helpers establish disposable requester, SSH, GPG and Cosign authority. Maven runs
only in the runner's unsigned producer job, using `build-type: lib` and the
project-owned profile; prepare performs the normal Maven version bump in the
scratch release commit. Signer keys remain in the native isolated signing jobs.
Artifact identity is read from the admitted revision's regular Git POM blob
(mode 100644 or 100755), never from a symlink-following worktree read.

The verdict requires the successful native release workflow, signed release Git
history, verified Cosign signatures binding all four nonempty downloaded POM,
main JAR, sources JAR and Javadoc JAR assets, matching release POM version,
Java 21 classfile major version 65, Java sources and generated Javadoc content.
The current-driver lock and normal scratch cleanup/retention rules apply.

This verdict is a Forgejo signed **file release**, not Maven Central or package
registry acceptance. Fresh registry/classifier resolution, the consumer MAC0
oracle, an independent dev lane, PR acceptance and provisional Nano findings
remain separate work. `Mac0Smoke.java` is supplied as a consumer-specific oracle
but this stable case does not execute it.

## Reactor boundary (native acceptance OPEN)

Maven reactor support is offered by the production library build (including
sibling bootstrap install), version bump (`versions:set -DprocessAllModules=true`)
and explicit jar/pom prebuilt export. It is not established by this single-library
native witness. `TestStageMavenBuildPreservesReactorPOMAndClassifierBytes` exercises
an aggregator and two children with a sibling dependency, stable/SNAPSHOT inherited
coordinates, exact POM/classifier bytes and publication inventory/checksums offline.
It executes no Maven and proves neither compilation nor version synchronization.

The current stable and development adapters refuse an aggregator before changing
their private index (`TestMavenLibraryFixturesRefuseReactorWithoutChangingSource`).
Their root `target/` handoff, one-artifact release inventory, development identity,
fresh registry readback and package cleanup are deliberately single-library owners.
A native reactor fixture needs a separately reviewed multi-publication oracle and
complete module handoff/cleanup at those existing owners; merely allowing extra
release assets or treating the parent POM as the library would weaken acceptance.
The stable selector above and the single-library development selector in
`mavenlibrarydevelopment.md` retain their scope and their refusal. Development
has a separate, opted-in reactor selector,
`TestForgejoMavenReactorConsumer_Development`, with per-module handoff,
readback and cleanup; it has not run natively, and there is no stable reactor
selector.

## Native checkpoint — 2026-10-30

Public Forgejo run **314** passed with an external Maven library's original source at
`25a3e92ceec72b869e2d04e4cbdebd9660ad8b7a` and release version `4.0.1`.
The native signed-file verdict above passed; this does not establish Maven
Central or stable package-registry publication. Independent snapshot registry
consumption passed separately in development run **315**, described in
`mavenlibrarydevelopment.md`.
