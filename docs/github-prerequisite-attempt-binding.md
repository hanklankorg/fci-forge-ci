<!-- SPDX-FileCopyrightText: 2026 Digg - Agency for Digital Government -->
<!-- SPDX-License-Identifier: CC0-1.0 -->

# GitHub prerequisite attempt capture (foundation only)

The existing native release jobs now expose additive `binding-run-id` and
`binding-run-attempt` outputs. No inputs, defaults, conditions, permissions,
secret mappings, commands or publishing routes change. No native job enforces
these outputs yet. This is not classified-route activation.

| Orchestrator `needs` producer | Actual capturing job | Forwarding |
| --- | --- | --- |
| `parse-config` | `release-orchestrator.yml:parse-config` | Direct job outputs |
| `validate-prerequisites` | `validate-release-prerequisites.yml:validate` | `workflow_call.outputs` from `jobs.validate.outputs` |
| `execute-prepare-stage` | `release-prepare-stage.yml:version-bump` | `workflow_call.outputs` from `jobs.version-bump.outputs`, alongside `release-sha` |
| `validate-release-tag` | `release-orchestrator.yml:validate-release-tag` | Direct job outputs |

Capture uses the producing job's own `github.run_id` / `github.run_attempt`.
Job output expressions are evaluated at the end of that job's execution, not
by a downstream receiver. A successful job retained by a failed-job-only rerun
retains its old attempt binding. Reusable workflow outputs only forward that
producer output: neither the calling job nor `summarize-prepare-stage` computes
a replacement attempt label. A fresh receiver must refuse the old attempt.
These are GitHub control-plane semantics; offline tests do not establish hosted
rerun acceptance. Hosted validation remains pending, with no live run here.

The inert template in
`test/workflowguard/testdata/prerequisite-attempt-binding.yml` shows a receiving
job's runtime refusal gate. It requires exact success and nonempty matching
run/attempt before the inert effect standing in for later secret mapping.
It is not a public workflow. A future explicitly selected trusted route must
repeat it for **each** required direct `needs` producer and place all private
secret mappings and trusted side effects after admission. Do not use a false
job condition as silent acceptance, `always()` or `continue-on-error` to bypass
refusal, dispatch-supplied bindings, or an aggregator's current context.

These outputs bind only the named jobs, not every transitive preparation task.
For example, a freshly rerun `version-bump` does not prove its retained changelog
or version-patch producers reran. A route requiring those tasks to be fresh must
capture and validate them separately before asserting whole-chain freshness.
Likewise prerequisite validation is not the proposed classified selected-test
policy job: that policy must capture its own execution when implemented.

Consumers must inspect `needs.<job>.result == 'success'` as well as the bindings;
outputs alone do not prove success. No artifact API response proves attempt,
job execution, successful policy, or prepared checkout. Trust is in the protected
workflow control plane and fixed output provenance, not payload declarations or
hostile-runner attestation. Administrator protection, the reviewed immutable
workflow/runtime/binary tuple, source authorization and file-only admission
remain independent requirements. No CLI changes or binary rebuild are needed
for this output-only foundation; deployment must still use the reviewed actual
committed workflow source. No production activation or full parity is claimed.
