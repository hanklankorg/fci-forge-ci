<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# Exact GitHub artifact download

This is an opt-in production adapter/port capability, not a release orchestrator,
authorization receipt or live acceptance result. Legacy name/pattern downloads
retain their existing behavior. Forgejo refuses exact-ID mode; GitLab still has
no programmatic run-artifact downloader and uses its declarative job handoffs.

```sh
forge-ci artifact download \
  --artifact-id 42 --name build-dist \
  --repository owner/repo --run-id 9876 \
  --expected-run-head-sha aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
  --dir downloaded-payload
```

The ID is an immutable artifact identifier, not a run or job ID. ID and run ID
must be positive int64 decimal values. Name, repository, run ID and expected run
head are mandatory binding expectations in this mode. Pattern matching, merging
and blob-store reconstruction are refused. There is no search/list-by-name,
latest-successful lookup or fallback after a failed ID lookup. Existing run and
repository CLI environment defaults still apply; the two new flags have **no
ambient environment defaults**. Trusted callers should pass every expectation
explicitly. Never derive expectations from the downloaded manifest itself.

## What the API actually establishes

The adapter uses the existing authenticated GitHub SDK client for:

1. `GET /repos/{owner}/{repo}`: require the returned full name to match the
   expected repository (case-insensitively) and observe its numeric ID.
2. `GET /repos/{owner}/{repo}/actions/artifacts/{artifact_id}`: require exact
   ID/name, an explicitly unexpired artifact, and `workflow_run` metadata matching
   the expected run/head and observed repository ID. This conservative opt-in
   mode also requires `head_repository_id` to equal the repository ID: fork-head
   artifacts are refused rather than implicitly authorized.
3. Only after binding succeeds, request that ID's ZIP URL. The blob GET uses the
   existing separate credential-free download client. API credentials do not
   reach the presigned URL. Credentialed/non-HTTPS blob URLs are refused on
   every redirect hop before it is sent. An exact-mode client copy retains the
   existing transport, timeout and redirect policy, adds a redirect bound and
   removes cookies, credential headers and signed referrers without modifying
   the shared client. Credential-free HTTPS cross-host redirects are allowed.

The SDK's artifact `workflow_run` has `id`, `repository_id`,
`head_repository_id`, `head_sha` and `head_branch`. The result's optional
`RunArtifactObservation` and CLI outputs record these API observations plus the
resolved repository and optional archive digest:

- `artifact-repository`, `artifact-repository-id`
- `artifact-run-id`, `artifact-run-head-sha`
- `artifact-head-repository-id`, `artifact-head-branch`
- `artifact-archive-digest`

The existing `artifact-id` and `artifact-name` outputs remain available. Empty
archive digest means the API supplied none, not that verification succeeded
against an invented value. When GitHub provides a `sha256:` archive digest, its
shape and the actual compressed ZIP bytes must match before extraction staging.
This digest is **not** the classified release payload's canonical dist digest.

The bounded ZIP spool, safe paths, size/file budgets and private atomic
extraction staging are reused. Exact mode additionally refuses duplicate ZIP
destinations (including aliases such as `file`/`./file` and
`dir/file`/`dir//file`), before extraction staging. Metadata refusal makes no blob request and creates no destination or
staging ancestors. Bad archives/digests install no payload. Existing safe
destination merge semantics remain unchanged; callers requiring a fresh
destination must choose a private new path and retain their admission checks.

## Facts this endpoint does not provide

Artifact metadata does **not** authenticate a producer job ID/name, successful
job result, run attempt, checked-out prepared commit, protected caller ownership
or mandatory selected-policy-test execution. These fields are deliberately not
fabricated in the observation type. Run ID alone does not distinguish reruns.

The API's `workflow_run.head_sha` can be the release-request SHA. A later trusted
prepare stage may approve and create a different release checkout SHA. Pass the
expected **request/run head** to this downloader; verify the **prepared checkout**
through the existing trusted request/tag/plan relationship separately. Downloaded
files asserting a prepared SHA do not authenticate that relationship.

A future trusted workflow must still own its immutable engine/runtime refs,
source authorization, selected mandatory policy checks, producer job and its
`needs` success dependency. The expected artifact ID must travel from that exact
producer upload step through its job output to the trusted consumer (with the
run/attempt and request identity under trusted workflow control), not through a
consumer manifest or arbitrary dispatch input. The consumer must refuse failed
producer/policy prerequisites before using the artifact. These caller-controlled
relationships are the available joint binding; this API is not a substitute
producer-job attestation. No new OIDC issuer, crypto receipt or trust pin is
introduced to paper over the missing job endpoint.

The existing read-only Actions credential path is used (`GH_TOKEN` or
`GITHUB_TOKEN`); give it only the Actions read access and repository metadata
visibility needed for those calls. No token flag, logged signed URL, signing key
or publication credential is added. Tests use in-memory SDK HTTP responses and
inert ZIP fixtures, not GitHub or any external network.

This unit does not wire a consumer producer, classified-file admission, signing,
image/flavor fan-in or publication workflow. Those integrations remain separate
review gates; full asset parity and live acceptance are not claimed.
