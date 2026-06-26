# QueueStorm Investigator

Deterministic Go API for the SUST CSE Carnival 2026 preliminary challenge.

The service analyzes one support ticket at a time, compares the complaint with the supplied transaction history, classifies and routes the case, and returns the exact JSON response shape required by the problem statement.

## Stack

- Go 1.22+ using the standard `net/http` server.
- No database.
- No LLM or paid AI API dependency.
- Deterministic rules for evidence matching, classification, routing, severity, safety, and response templates.

Rule-based logic is intentional here: the rubric heavily rewards evidence reasoning, schema correctness, safety, latency, and reliability. Avoiding model calls removes quota, cost, network, prompt-injection, and nondeterministic-output risk.

## Endpoints

### `GET /health`

Returns readiness JSON:

```json
{"status":"ok"}
```

### `POST /analyze-ticket`

Accepts the official request schema and returns the official response schema.

```bash
curl -s -X POST http://localhost:8000/analyze-ticket \
  -H 'Content-Type: application/json' \
  --data @testdata/sample_request.json
```

## Local Run

```bash
go test ./...
go run ./cmd/server
```

The server binds to `0.0.0.0:${PORT}`. If `PORT` is unset it uses `8000`.

Smoke test:

```bash
curl -i http://localhost:8000/health
```

## Docker

```bash
docker build -t queuestorm-investigator .
docker run -p 8000:8000 --env-file .env.example queuestorm-investigator
```

## VM / CI-CD Deployment

This repo includes GitHub Actions deployment to the team VM:

- Host: `168.144.42.224`
- User: `root`
- App port: `8000`
- Secret required in GitHub: `SSH_PASSWORD`
- Workflow: `.github/workflows/deploy.yml`
- Systemd unit: `deploy/queuestorm-investigator.service`

On every push to `main`, CI/CD will:

1. Run `go test -count=1 ./...`.
2. Build a static Linux `amd64` binary.
3. Upload the release to `/opt/queuestorm-investigator/releases/<git-sha>`.
4. Point `/opt/queuestorm-investigator/current` to the new release.
5. Install/reload the `queuestorm-investigator.service` systemd unit.
6. Restart the service.
7. Smoke-test local VM endpoints:
   - `http://127.0.0.1:8000/health`
   - `http://127.0.0.1:8000/analyze-ticket`

Manual VM checks after deployment:

```bash
systemctl status queuestorm-investigator --no-pager
curl -fsS http://168.144.42.224:8000/health
curl -fsS -X POST http://168.144.42.224:8000/analyze-ticket   -H 'Content-Type: application/json'   --data @testdata/sample_request.json
```

Submit `http://168.144.42.224:8000` as the base URL if the VM firewall exposes port `8000`. Judges should be able to call `/health` and `/analyze-ticket` without login.

## Sample Tests

The official public sample pack is stored in two places:

- `docs/SUST_Preli_Sample_Cases.json` for reference.
- `testdata/sample_cases.json` for automated tests.

The test suite checks all 10 public samples on these key fields:

- `relevant_transaction_id`
- `evidence_verdict`
- `case_type`
- `severity`
- `department`
- `human_review_required`

Run:

```bash
go test ./...
```

## Safety Logic

The API never asks customers for PIN, OTP, password, full card number, or secret credentials. It also avoids unauthorized refund, reversal, account-unblock, or recovery promises.

Safety behavior is deterministic:

- Phishing, OTP, PIN, password, fake support, suspicious link, and account-blocking pressure route to `fraud_risk`.
- Complaint text is untrusted data only. Prompt-injection phrases never control verdicts, routing, schema, or output wording.
- Customer replies use pre-audited templates.
- A final sanitizer runs on all text fields before the JSON response is returned.

Safe refund wording uses phrases like:

> any eligible amount will be returned through official channels

## Models

No external model is used in this implementation. No `OPENAI_API_KEY`, `GEMINI_API_KEY`, model download, GPU, or network call is required.

## Known Limitations

- The analyzer is intentionally rule-based and may miss unusual phrasing outside the covered English, Bangla, and Banglish patterns.
- Time reasoning is approximate; transaction matching relies more on amount, type, counterparty, status, and duplicate/recipient patterns.
- Bangla templates are strongest for phishing and agent cash-in cases; uncertain mixed-language cases use safe English fallback.
- Hidden tests may include edge cases beyond the public samples, so the rules are pattern-based rather than sample-ID based.

## Documentation

Source challenge documents and the implementation plan are in `docs/`:

- `docs/PLAN.md`
- `docs/Evaluation_Rubric.md`
- `docs/Team_instruction_manual.md`
- `docs/problem_statement_extracted.txt`
- Original PDFs
