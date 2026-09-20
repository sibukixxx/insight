# Publication review and approved artifacts

Open a project → **Research publications**. Create a run from its completed analysis or select an existing run, then read the report and current artifact. Choose a contribution and explicitly record each human judgment. Saving the review calculates the mechanical checklist and displays blocking reasons. A successful review stops at `PUBLICATION_READY`.

**Mark approved artifact as published** records a separate human action. It does not post to note, SNS, or another service. Published and rejected iterations cannot be reviewed again; use a new iteration for new research. Re-reviewing a ready iteration rechecks all gates and removes approval if any gate fails.

## Shared downstream contract

`GET /api/research-runs/{runID}/approved-artifact.json` returns:

- `reference`: `sha256:` followed by the SHA-256 digest of the stored compact artifact JSON bytes.
- `approvedAt`: the human review's assessment timestamp.
- `artifact`: the versioned `insight-lab.research-artifact` v1 payload as approved, including promotion status, gate input, evidence, limitations and provenance.

Consumers of Public Report, note and SNS must pin the same reference and retain the approved payload, rather than rebuilding it from the current report or analysis. The nested promotion state remains `PUBLICATION_READY` even after publication; the run separately records `PUBLISHED`. This endpoint returns the latest iteration's current approval, not a historical content-addressed store. Re-review can replace it, and a new unapproved iteration returns HTTP 409. Legacy ready records without an approval snapshot also return 409 and require a new review.

`artifact.json` is the live research export. Its `promotion` and `promotionGateInput` fields are additive v1 fields. `PromotionGateInput` retains its existing persisted PascalCase keys; the checklist and assessment use their existing lower-camel keys. Approved snapshots are stored in the existing iteration JSON, requiring no database migration.

Provenance comes from the analysis associated with the current iteration's insights, not the most recent project analysis. Missing or mixed analysis references fail closed. Counter-evidence search requires recorded search coverage or actual counter-evidence; falsification criteria alone are insufficient.

## Verification

Run `make vet test test-golden` and `node --check internal/web/dist/app.js`.

For manual browser verification, open **Research publications**, submit a review with human approval unchecked and confirm blocking reasons and unavailable approved download. Review a fully supported research run with all relevant judgments, confirm `PUBLICATION_READY`, download the approved payload, then mark it published and confirm the reference and payload remain unchanged. Errors must appear inline, and failed actions must remain retryable.
