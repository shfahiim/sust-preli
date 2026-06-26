#!/usr/bin/env python3
"""Run all cases from docs/SUST_Preli_Sample_Cases.json against a live or local API."""

from __future__ import annotations

import argparse
import json
import sys
import urllib.error
import urllib.request

KEY_FIELDS = [
    "relevant_transaction_id",
    "evidence_verdict",
    "case_type",
    "severity",
    "department",
    "human_review_required",
]

SAMPLE_PACK = "docs/SUST_Preli_Sample_Cases.json"


def analyze(base_url: str, payload: dict) -> tuple[int, dict]:
    body = json.dumps(payload).encode()
    req = urllib.request.Request(
        f"{base_url.rstrip('/')}/analyze-ticket",
        data=body,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return resp.status, json.loads(resp.read().decode())
    except urllib.error.HTTPError as exc:
        raw = exc.read().decode()
        try:
            return exc.code, json.loads(raw)
        except json.JSONDecodeError:
            return exc.code, {"error": raw or str(exc)}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--base-url",
        default="http://localhost:8000",
        help="API base URL (default: http://localhost:8000)",
    )
    parser.add_argument(
        "--write-results",
        action="store_true",
        help="Write sample_outputs/official_sample_run_results.json",
    )
    args = parser.parse_args()

    with open(SAMPLE_PACK, encoding="utf-8") as f:
        pack = json.load(f)

    passed = 0
    results = []
    for case in pack["cases"]:
        status, got = analyze(args.base_url, case["input"])
        want = case["expected_output"]
        mismatches = []
        if status == 200:
            for field in KEY_FIELDS:
                if got.get(field) != want.get(field):
                    mismatches.append(
                        {"field": field, "got": got.get(field), "want": want.get(field)}
                    )
        ok = status == 200 and not mismatches
        if ok:
            passed += 1
        mark = "PASS" if ok else "FAIL"
        print(f"{mark} {case['id']}: http={status}", end="")
        if mismatches:
            print(f" mismatches={mismatches}")
        else:
            print()
        results.append(
            {
                "id": case["id"],
                "label": case["label"],
                "source": SAMPLE_PACK,
                "endpoint": f"{args.base_url.rstrip('/')}/analyze-ticket",
                "pass": ok,
                "mismatches": mismatches,
                "input": case["input"],
                "output": got,
            }
        )

    print(f"\nSUMMARY: {passed}/{len(results)} passed")

    if args.write_results:
        primary = results[0]
        with open("sample_outputs/sample_case_001_request.json", "w", encoding="utf-8") as f:
            json.dump(primary["input"], f, indent=2, ensure_ascii=False)
            f.write("\n")
        with open("sample_outputs/sample_case_001_response.json", "w", encoding="utf-8") as f:
            json.dump(primary["output"], f, indent=2, ensure_ascii=False)
            f.write("\n")
        with open("sample_outputs/official_sample_run_results.json", "w", encoding="utf-8") as f:
            json.dump(
                {
                    "meta": {
                        "source_file": SAMPLE_PACK,
                        "endpoint_base_url": args.base_url.rstrip("/"),
                        "cases_run": len(results),
                        "cases_passed_key_fields": passed,
                    },
                    "cases": results,
                },
                f,
                indent=2,
                ensure_ascii=False,
            )
            f.write("\n")
        print("Wrote sample_outputs/")

    return 0 if passed == len(results) else 1


if __name__ == "__main__":
    sys.exit(main())
