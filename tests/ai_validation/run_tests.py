#!/usr/bin/env python3
"""Run Windropolis fixed controls and full AI workflows with one case catalog."""

import argparse
import csv
from datetime import datetime, timezone
import getpass
import hashlib
import json
import os
from pathlib import Path
import secrets
import sys
import time
from urllib import parse

if __package__:
    from . import fixed, workflow
else:
    import fixed
    import workflow

ROOT = Path(__file__).resolve().parent
CASES = ROOT / "cases"
PROMPT_FILES = ("ai/generator.go", "ai/semantic_prompts.go", "ai/premise_prompts.go",
                "ai/semantic_stability.go", "ai/difficulty_agent.go", "ai/client.go")


def json_hash(value):
    data = json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(data.encode("utf-8")).hexdigest()


def prompt_fingerprint():
    root = ROOT.parent.parent
    digest = hashlib.sha256()
    for name in PROMPT_FILES:
        source = root / name
        digest.update(name.encode("utf-8") + b"\x00" + source.read_bytes())
    return digest.hexdigest()


def load_cases(group="all", ids=None):
    selected = {}
    for track in ("fixed", "workflow"):
        if group not in ("all", track):
            continue
        for path in sorted((CASES / track).glob("*/case.json")):
            data = json.loads(path.read_text(encoding="utf-8"))
            name = path.parent.name
            if data.get("id") != name or data.get("track") != track or name in selected:
                raise ValueError(f"Invalid case identity at {path}")
            if track == "fixed":
                if not {"email", "generation_context", "evaluation_context"} <= set(data["input"]):
                    raise ValueError(f"{name}: fixed input is incomplete")
                workflow.check_email(data["input"]["email"])
            else:
                if not {"generation_context", "evaluation_context"} <= set(data):
                    raise ValueError(f"{name}: workflow context is incomplete")
                workflow.case_input(data, path.parent, [], "") if data.get("sending_profile") == "Unknown / not selected" else validate_files(data, path.parent)
            selected[name] = (data, path.parent)
    if not selected:
        raise ValueError(f"No cases in {CASES}")
    unknown = set(ids or []) - selected.keys()
    if unknown:
        raise ValueError("Unknown case IDs: " + ", ".join(sorted(unknown)))
    return {key: value for key, value in selected.items() if not ids or key in ids}


def validate_files(case, folder):
    for item in (case["evaluation_context"].get("attachments") or {}).get("files", []):
        filename = Path(item["name"])
        if filename.name != item["name"] or not (folder / "attachments" / filename).is_file():
            raise ValueError(f"{folder.name}: attachment missing or invalid: {filename}")
    if case.get("manual_email"):
        for field in ("text_file", "html_file"):
            filename = Path(case["manual_email"][field])
            if filename.name != str(filename):
                raise ValueError(f"{folder.name}: unsafe email path")
            (folder / filename).read_text(encoding="utf-8")
    if case.get("attachment_roundtrip"):
        filename = Path(case["attachment_roundtrip"]["file"])
        if filename.parts[0] != "attachments" or ".." in filename.parts:
            raise ValueError(f"{folder.name}: unsafe attachment roundtrip path")
        (folder / filename).read_bytes()
    if not case.get("manual_email") and case["generation_context"].get("target_difficulty") not in workflow.DIFFICULTIES:
        raise ValueError(f"{folder.name}: generation requires a target difficulty")


class RecordingAPI:
    """Record every AI request/response; never record the admin key or SMTP list."""

    def __init__(self, api):
        self.api = api
        self.events = []

    def call(self, path, payload=None):
        if not path.startswith("/api/ai/templates/"):
            return self.api.call(path, payload)
        event = {"endpoint": path, "started_utc": datetime.now(timezone.utc).isoformat(),
                 "input_hash": json_hash(payload), "input": payload}
        start = time.monotonic()
        try:
            response = self.api.call(path, payload)
            event["response"] = response
            return response
        except Exception as exc:
            event["error"] = str(exc)
            raise
        finally:
            event["duration_seconds"] = round(time.monotonic() - start, 3)
            self.events.append(event)


def validate_profiles(cases, profiles, alice_profile=""):
    expected = {profile["name"]: profile for profile in
                json.loads((ROOT / "profiles.json").read_text(encoding="utf-8"))}
    for name, (case, _) in cases.items():
        if case["track"] != "workflow" or case.get("sending_profile") == "Unknown / not selected":
            continue
        planned = expected.get(case.get("sending_profile"))
        if planned is None:
            raise ValueError(f"{name}: unknown profile in profiles.json")
        sender = workflow.simulated_sender(case, profiles, alice_profile)
        if sender["email"].lower() != workflow.parseaddr(planned["from_address"])[1].lower():
            raise ValueError(f"{name}: Sending Profile From differs from profiles.json")


def observation_rows(case_id, run, target, events):
    for event in events:
        response = event.get("response", {})
        evaluation = response.get("evaluation", response) if isinstance(response, dict) else {}
        if not isinstance(evaluation, dict) or not {"criteria", "cues", "difficulty"} <= evaluation.keys():
            continue
        for item in evaluation["criteria"]:
            yield {"case_id": case_id, "run": run, "target": target,
                   "evaluation_mode": (event["input"] or {}).get("evaluation_mode", "internal"),
                   "duration_seconds": event["duration_seconds"], "endpoint": event["endpoint"],
                   "input_hash": event["input_hash"], "id": item["id"], "kind": "criterion",
                   "min": item["min_value"], "max": item["max_value"],
                   "evidence": json.dumps(item.get("evidence", []), ensure_ascii=False),
                   "evaluation_repair": item.get("evaluation_repair", ""),
                   "cue_min": evaluation["cues"]["min_count"], "cue_max": evaluation["cues"]["max_count"],
                   "difficulty": evaluation["difficulty"].get("detection_difficulty", "")}
        for item in evaluation["premise_alignment"]["elements"]:
            yield {"case_id": case_id, "run": run, "target": target,
                   "evaluation_mode": (event["input"] or {}).get("evaluation_mode", "internal"),
                   "duration_seconds": event["duration_seconds"], "endpoint": event["endpoint"],
                   "input_hash": event["input_hash"], "id": item["id"], "kind": "premise",
                   "min": item.get("score"), "max": item.get("score"),
                   "evidence": item.get("explanation", ""),
                   "evaluation_repair": "",
                   "cue_min": evaluation["cues"]["min_count"], "cue_max": evaluation["cues"]["max_count"],
                   "difficulty": evaluation["difficulty"].get("detection_difficulty", "")}


def review_packet(case, trace, events, blind_id):
    """Only the email and real campaign context are visible to reviewers."""
    if "generate" not in trace.get("phases", {}):
        return None
    sampled = next((event["input"] for event in events if event["endpoint"].endswith("/evaluate")
                    and "response" in event), None)
    if sampled is None:
        return None
    generation = copy_review_generation(case["generation_context"])
    return {"blind_id": blind_id, "email": sampled["email"],
            "generation_context": generation, "evaluation_context": sampled["evaluation_context"]}


def copy_review_generation(generation):
    return {key: value for key, value in generation.items()
            if key not in ("target_difficulty", "additional_instructions")}


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--url", default=os.getenv("GOPHISH_URL", "https://127.0.0.1:3333"))
    parser.add_argument("--insecure", action="store_true", help="Accept a self-signed localhost HTTPS certificate")
    parser.add_argument("--group", choices=("all", "fixed", "workflow"), default="all")
    parser.add_argument("--mode", choices=("regression", "experiment"), default="regression")
    parser.add_argument("--repeats", type=int, default=2,
                        help="Cached checks in regression; independent draws in experiment")
    parser.add_argument("--only", default="", help="Comma-separated case IDs")
    parser.add_argument("--timeout", type=int, default=600, help="Seconds per API request")
    parser.add_argument("--output", type=Path, default=ROOT / "results")
    parser.add_argument("--study-id", default="", help="Study identifier (required in experiment mode)")
    parser.add_argument("--model-id", default="", help="Actual Ollama model and version (required in experiment mode)")
    parser.add_argument("--alice-profile", default="", help="Optional override for Alice's exact profile name")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args(argv)
    url = parse.urlsplit(args.url)
    if url.scheme not in ("http", "https") or not url.netloc or url.username or url.password or url.path not in ("", "/"):
        parser.error("--url must be an HTTP(S) origin without credentials or path")
    if args.insecure and (url.scheme != "https" or url.hostname not in ("localhost", "127.0.0.1", "::1")):
        parser.error("--insecure is allowed only for local HTTPS")
    if args.repeats < 1 or args.timeout < 1:
        parser.error("--repeats and --timeout must be positive")
    if args.mode == "experiment" and not args.dry_run and (not args.study_id or not args.model_id):
        parser.error("experiment mode requires --study-id and --model-id")
    if args.mode == "experiment" and not args.dry_run and args.output.exists() and any(args.output.iterdir()):
        parser.error("Use a new, empty --output directory for each experiment")
    ids = [x.strip() for x in args.only.split(",") if x.strip()]
    try:
        cases = load_cases(args.group, ids)
    except (ValueError, KeyError, OSError, AssertionError) as exc:
        parser.error(str(exc))
    if args.dry_run:
        for name, (case, _) in cases.items():
            print(f"{name} [{case['track']}]: {case.get('name', case.get('purpose', ''))}")
        print(f"{len(cases)} validated cases; no server request was sent")
        return 0

    key = os.getenv("GOPHISH_API_KEY") or getpass.getpass("GoPhish API key: ")
    if not key.strip():
        parser.error("API key is required")
    api = workflow.API(args.url, key, args.insecure, args.timeout)
    profiles = []
    if any(case["track"] == "workflow" for case, _ in cases.values()):
        try:
            profiles = api.call("/api/smtp/")
            if not isinstance(profiles, list):
                raise ValueError("GET /api/smtp/ did not return a list")
            validate_profiles(cases, profiles, args.alice_profile)
        except (RuntimeError, ValueError, KeyError, TypeError) as exc:
            parser.error(f"Sending Profile preflight failed: {exc}")
    args.output.mkdir(parents=True, exist_ok=True)
    rows, observations, fixed_results = [], [], {}
    blinded_map, annotations = [], []
    for case_id, (case, folder) in cases.items():
        count = args.repeats if case["track"] == "fixed" or args.mode == "experiment" else 1
        for run in range(1, count + 1):
            recorder = RecordingAPI(api)
            errors, deviations, trace = [], [], {}
            try:
                if case["track"] == "fixed":
                    evaluation = recorder.call("/api/ai/templates/evaluate", {
                        **case["input"], "evaluation_mode": "sample" if args.mode == "experiment" else "reuse"})
                    if not isinstance(evaluation, dict) or not {"criteria", "cues", "premise_alignment", "difficulty"} <= evaluation.keys():
                        raise AssertionError("Incomplete evaluation response")
                    violations = fixed.assert_response(evaluation, case.get("expected", {}))
                    if args.mode == "experiment":
                        deviations.extend(violations)
                    else:
                        errors.extend(violations)
                    fixed_results.setdefault(case_id, []).append(evaluation)
                    trace = {"evaluation": evaluation}
                else:
                    trace = workflow.run_case(recorder, case_id, case, folder, profiles,
                                              1 if args.mode == "experiment" else args.repeats,
                                              args.alice_profile, "sample" if args.mode == "experiment" else "reuse")
                    errors.extend(trace["issues"])
            except (AssertionError, ValueError, KeyError, TypeError, RuntimeError, OSError) as exc:
                errors.append(str(exc))
            result = {"case_id": case_id, "track": case["track"], "run": run,
                      "case_hash": json_hash(case), "mode": args.mode,
                      "status": "FAIL" if errors else "PASS", "errors": errors,
                      "deviations": deviations, "trace": trace, "events": recorder.events}
            target = args.output / "cases" / case_id
            target.mkdir(parents=True, exist_ok=True)
            (target / f"run-{run:03}.json").write_text(
                json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
            if args.mode == "experiment" and case_id.startswith("GEN-") and not errors:
                blind_id = secrets.token_hex(8)
                packet = review_packet(case, trace, recorder.events, blind_id)
                if packet is not None:
                    review_dir = args.output / "review_packets"
                    review_dir.mkdir(exist_ok=True)
                    (review_dir / f"{blind_id}.json").write_text(
                        json.dumps(packet, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
                    blinded_map.append({"blind_id": blind_id, "case_id": case_id, "run": run,
                                        "target": case["generation_context"]["target_difficulty"]})
                    annotations.extend({"blind_id": blind_id, "annotator_id": annotator,
                                        "cue_min": "", "cue_max": "", "workplace_process": "",
                                        "workplace_relevance": "", "situational_alignment": "",
                                        "consequences_for_not_clicking": "", "prior_training_or_exposure": "",
                                        "difficulty": "", "unresolved_reason": "", "notes": ""}
                                       for annotator in ("A", "B"))
            rows.append({"case_id": case_id, "track": case["track"], "run": run,
                         "status": result["status"], "details": "; ".join(errors),
                         "deviations": "; ".join(deviations)})
            observations.extend(observation_rows(case_id, run,
                                case.get("generation_context", {}).get("target_difficulty", "")
                                if case["track"] == "workflow" else "", recorder.events))
            print(f"[{result['status']}] {case_id} run {run}" + (": " + "; ".join(errors) if errors else ""), flush=True)

    comparisons = json.loads((CASES / "comparisons.json").read_text(encoding="utf-8"))
    if args.mode == "regression":
        more = fixed.comparison_errors({"comparisons": comparisons}, fixed_results, set(fixed_results))
        more += fixed.stability_errors(fixed_results)
        for issue in more:
            rows.append({"case_id": "cross_case", "track": "fixed", "run": "",
                         "status": "FAIL", "details": issue, "deviations": ""})
            print("[FAIL] " + issue, flush=True)
    metadata = {"timestamp_utc": datetime.now(timezone.utc).isoformat(), "mode": args.mode,
                "study_id": args.study_id or None, "model_id": args.model_id or None,
                "prompt_files_sha256": prompt_fingerprint(), "url": args.url,
                "repeats": args.repeats, "case_ids": list(cases), "checks": rows,
                "note": "Model ID is supplied by the operator; generated targets are hypotheses, not human detection data."}
    (args.output / "report.json").write_text(json.dumps(metadata, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    if blinded_map:
        (args.output / "blind_key.json").write_text(
            json.dumps(blinded_map, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        with (args.output / "annotation_template.csv").open("w", encoding="utf-8", newline="") as file:
            writer = csv.DictWriter(file, fieldnames=list(annotations[0]))
            writer.writeheader()
            writer.writerows(annotations)
    with (args.output / "report.csv").open("w", encoding="utf-8", newline="") as file:
        writer = csv.DictWriter(file, fieldnames=("case_id", "track", "run", "status", "details", "deviations"))
        writer.writeheader()
        writer.writerows(rows)
    with (args.output / "observations.csv").open("w", encoding="utf-8", newline="") as file:
        fields = ("case_id", "run", "target", "evaluation_mode", "duration_seconds",
                  "endpoint", "input_hash", "id", "kind", "min", "max", "evidence", "evaluation_repair",
                  "cue_min", "cue_max", "difficulty")
        writer = csv.DictWriter(file, fieldnames=fields)
        writer.writeheader()
        writer.writerows(observations)
    failures = sum(row["status"] == "FAIL" for row in rows)
    print(f"{len(rows) - failures} passed, {failures} failed. Reports: {args.output}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
