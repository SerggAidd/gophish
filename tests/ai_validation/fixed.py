#!/usr/bin/env python3
"""Run controlled NIST Phish Scale checks against GoPhish's evaluate API."""

import argparse
import csv
import getpass
import json
import os
import pathlib
import ssl
import sys
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone

ROOT = pathlib.Path(__file__).resolve().parent
ENDPOINT = "/api/ai/templates/evaluate"


def merge(base, changes):
    result = dict(base)
    for key, value in changes.items():
        result[key] = merge(result[key], value) if isinstance(value, dict) and isinstance(result.get(key), dict) else value
    return result


def selected_cases(suite, ids):
    cases = suite["cases"]
    names = [case["id"] for case in cases]
    if len(names) != len(set(names)):
        raise ValueError("Duplicate case IDs")
    unknown = ids - set(names)
    if unknown:
        raise ValueError("Unknown IDs: " + ", ".join(sorted(unknown)))
    return [case for case in cases if not ids or case["id"] in ids]


def check_value(label, actual, rule):
    if isinstance(rule, int):
        return [] if actual == rule else [f"{label}: expected {rule}, got {actual}"]
    if not isinstance(rule, dict) or not rule or set(rule) - {"min", "max"}:
        raise ValueError(f"Invalid assertion for {label}: {rule!r}")
    if not isinstance(actual, int):
        return [f"{label}: expected a number, got {actual!r}"]
    errors = []
    if "min" in rule and actual < rule["min"]:
        errors.append(f"{label}: expected >= {rule['min']}, got {actual}")
    if "max" in rule and actual > rule["max"]:
        errors.append(f"{label}: expected <= {rule['max']}, got {actual}")
    return errors


def assert_response(response, expected):
    errors = []
    criteria = {item["id"]: item for item in response["criteria"]}
    elements = {item["id"]: item for item in response["premise_alignment"]["elements"]}
    for criterion, ranges in expected.get("criteria", {}).items():
        if criterion not in criteria:
            errors.append(f"criterion {criterion}: absent")
            continue
        for field, rule in ranges.items():
            errors.extend(check_value(f"{criterion}.{field}", criteria[criterion].get(field), rule))
    for element, rule in expected.get("premise", {}).items():
        if element not in elements:
            errors.append(f"premise {element}: absent")
        else:
            errors.extend(check_value(f"premise {element}.score", elements[element].get("score"), rule))
    return errors


def post(url, api_key, payload, insecure, timeout):
    data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(url.rstrip("/") + ENDPOINT, data=data, headers={
        "Authorization": api_key,
        "Content-Type": "application/json",
    }, method="POST")
    context = ssl._create_unverified_context() if insecure else ssl.create_default_context()
    try:
        with urllib.request.urlopen(req, timeout=timeout, context=context) as reply:
            return json.load(reply)
    except urllib.error.HTTPError as exc:
        body = exc.read(1200).decode("utf-8", "replace")
        raise RuntimeError(f"HTTP {exc.code}: {body}") from exc


def comparison_errors(suite, results, case_ids):
    errors = []
    for item in suite.get("comparisons", []):
        a, b = item["lower"], item["higher"]
        if a not in case_ids or b not in case_ids:
            continue
        for repetition in range(min(len(results.get(a, [])), len(results.get(b, [])))):
            if "element" in item:
                low = results[a][repetition]["premise_alignment"]["elements"]
                high = results[b][repetition]["premise_alignment"]["elements"]
                low_score = next(x.get("score") for x in low if x["id"] == item["element"])
                high_score = next(x.get("score") for x in high if x["id"] == item["element"])
            else:
                low = results[a][repetition]["criteria"]
                high = results[b][repetition]["criteria"]
                low_score = next(x["min_value"] for x in low if x["id"] == item["criterion"])
                high_score = next(x["min_value"] for x in high if x["id"] == item["criterion"])
            if low_score is None or high_score is None or not low_score < high_score:
                errors.append(f"{a} < {b} ({item.get('element', item.get('criterion'))}, run {repetition + 1}): {low_score!r} !< {high_score!r}")
    return errors


def observation_rows(results):
    """Keep individual scores and evidence available in a flat CSV for review."""
    for case_id, responses in results.items():
        for run, response in enumerate(responses, 1):
            cues = response["cues"]
            premise = response["premise_alignment"]
            for item in response["criteria"]:
                yield {"case_id": case_id, "run": run, "kind": "criterion", "id": item["id"],
                       "min": item["min_value"], "max": item["max_value"],
                       "category": cues.get("category", ""),
                       "evidence": json.dumps(item.get("evidence", []), ensure_ascii=False),
                       "cue_min": cues["min_count"], "cue_max": cues["max_count"],
                       "premise_min": premise["min_score"], "premise_max": premise["max_score"]}
            for item in premise["elements"]:
                yield {"case_id": case_id, "run": run, "kind": "premise", "id": item["id"],
                       "min": item.get("score", ""), "max": item.get("score", ""),
                       "category": premise.get("category", ""),
                       "evidence": item.get("explanation", ""),
                       "cue_min": cues["min_count"], "cue_max": cues["max_count"],
                       "premise_min": premise["min_score"], "premise_max": premise["max_score"]}


def stability_errors(results):
    errors = []
    for case_id, responses in results.items():
        if len(responses) < 2:
            continue
        def score_map(response):
            return ({x["id"]: (x["min_value"], x["max_value"]) for x in response["criteria"]},
                    {x["id"]: x.get("score") for x in response["premise_alignment"]["elements"]})
        baseline = score_map(responses[0])
        for run, response in enumerate(responses[1:], 2):
            current = score_map(response)
            for kind, before, after in zip(("criterion", "premise"), baseline, current):
                for key in before.keys() | after.keys():
                    if before.get(key) != after.get(key):
                        errors.append(f"{case_id} run 1 vs {run}, {kind} {key}: {before.get(key)} vs {after.get(key)}")
    return errors
