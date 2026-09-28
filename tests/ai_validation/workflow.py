#!/usr/bin/env python3
"""Exercise AI template workflows without creating campaigns or sending mail."""

import argparse
import copy
import csv
import getpass
from html.parser import HTMLParser
import json
import os
from pathlib import Path
import re
import ssl
import sys
from datetime import datetime, timezone
from email.utils import parseaddr
from urllib import error, parse, request

ROOT = Path(__file__).resolve().parent
FIXTURES = ROOT / "cases" / "workflow"
ABSOLUTE_URL = re.compile(r"https?://[^\s\"'<>]+", re.IGNORECASE)
DIFFICULTIES = {"very_difficult", "moderately_difficult", "moderately_to_least_difficult", "least_difficult"}


class API:
    def __init__(self, origin, key, insecure=False, timeout=180):
        self.origin = origin.rstrip("/")
        self.key = key
        self.timeout = timeout
        self.tls = ssl._create_unverified_context() if insecure else ssl.create_default_context()

    def call(self, path, payload=None):
        body = None if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
        headers = {"Authorization": self.key}
        if body is not None:
            headers["Content-Type"] = "application/json"
        req = request.Request(self.origin + path, data=body, headers=headers,
                              method="GET" if body is None else "POST")
        try:
            with request.urlopen(req, timeout=self.timeout, context=self.tls) as reply:
                return json.load(reply)
        except error.HTTPError as exc:
            detail = exc.read(600).decode("utf-8", "replace")
            raise RuntimeError(f"{path}: HTTP {exc.code}: {detail}") from exc


def load_cases(ids=None):
    cases = {}
    for path in sorted(FIXTURES.glob("*/case.json")):
        case_id = path.parent.name
        cases[case_id] = (json.loads(path.read_text(encoding="utf-8")), path.parent)
    if not cases:
        raise ValueError("No fixtures found")
    unknown = set(ids or []) - cases.keys()
    if unknown:
        raise ValueError("Unknown case IDs: " + ", ".join(sorted(unknown)))
    return {key: value for key, value in cases.items() if not ids or key in ids}


def simulated_sender(case, profiles, alice_profile=""):
    name = case.get("sending_profile") or ""
    email = case.get("sending_profile_from")
    if name == "Unknown / not selected":
        return None
    if alice_profile and email == "alice.smith@windropolis.corporate":
        name = alice_profile
    matches = [profile for profile in profiles if profile.get("name") == name] if name else [
        profile for profile in profiles if
        parseaddr(profile.get("from_address", ""))[1].lower() == (email or "").lower()]
    if len(matches) != 1:
        raise ValueError(f"Sending profile for {name or email!r}: expected exactly one match, got {len(matches)}")
    display, address = parseaddr(matches[0].get("from_address", ""))
    if not address:
        raise ValueError(f"Sending profile {name or email!r} has no valid From address")
    if email and address.lower() != email.lower():
        raise ValueError(f"Selected sending profile has From {address!r}, expected {email!r}")
    return {"display_name": display, "email": address}


def case_input(case, folder, profiles, alice_profile=""):
    generation = copy.deepcopy(case["generation_context"])
    context = copy.deepcopy(case["evaluation_context"])
    sender = simulated_sender(case, profiles, alice_profile)
    if sender is not None:
        context["simulated_sender"] = sender
    manual = case.get("manual_email")
    email = None
    if manual:
        email = {"subject": manual["subject"],
                 "text": (folder / manual["text_file"]).read_text(encoding="utf-8"),
                 "html": (folder / manual["html_file"]).read_text(encoding="utf-8")}
    for item in context.get("attachments", {}).get("files", []):
        if not (folder / "attachments" / item["name"]).is_file():
            raise ValueError(f"Missing fixture attachment: {folder / 'attachments' / item['name']}")
    roundtrip = case.get("attachment_roundtrip")
    if roundtrip and not (folder / roundtrip["file"]).is_file():
        raise ValueError(f"Missing roundtrip attachment: {roundtrip['file']}")
    if not email and generation.get("target_difficulty") not in DIFFICULTIES:
        raise ValueError("Generation case requires a target difficulty")
    return email, generation, context


def score_signature(evaluation):
    """Ignore prose/evidence drift; compare the values shown as scores in the UI."""
    return {
        "criteria": {item["id"]: (item["min_value"], item["max_value"])
                     for item in evaluation["criteria"]},
        "premise": {item["id"]: item.get("score")
                    for item in evaluation["premise_alignment"]["elements"]},
        "cue_range": (evaluation["cues"]["min_count"], evaluation["cues"]["max_count"]),
        "premise_range": (evaluation["premise_alignment"]["min_score"],
                          evaluation["premise_alignment"]["max_score"]),
        "difficulty": (evaluation["difficulty"].get("detection_difficulty"),
                       evaluation["difficulty"]["resolved"]),
    }


def signature_differences(before, after):
    errors = []
    for group in before.keys() | after.keys():
        left, right = before.get(group), after.get(group)
        if isinstance(left, dict) and isinstance(right, dict):
            for key in left.keys() | right.keys():
                if left.get(key) != right.get(key):
                    errors.append(f"{group}.{key}: {left.get(key)} -> {right.get(key)}")
        elif left != right:
            errors.append(f"{group}: {left} -> {right}")
    return sorted(errors)


def cue_value(evaluation, criterion):
    matches = [item for item in evaluation["criteria"] if item["id"] == criterion]
    if len(matches) != 1:
        raise AssertionError(f"Expected exactly one {criterion} criterion")
    return matches[0]["min_value"], matches[0]["max_value"]


def check_expectations(evaluation, email, expected):
    for criterion, bounds in expected.get("criteria", {}).items():
        minimum, maximum = cue_value(evaluation, criterion)
        if "min" in bounds and minimum < bounds["min"]:
            raise AssertionError(f"{criterion}: expected min >= {bounds['min']}, got {minimum}")
        if "max" in bounds and maximum > bounds["max"]:
            raise AssertionError(f"{criterion}: expected max <= {bounds['max']}, got {maximum}")
    if "context_complete" in expected and evaluation.get("context_complete") != expected["context_complete"]:
        raise AssertionError("Incorrect context completeness")
    for field in expected.get("missing_context", []):
        if field not in evaluation.get("missing_context", []):
            raise AssertionError(f"Expected missing context field {field!r}")
    if expected.get("email_language") == "ru" and not re.search(r"[А-Яа-яЁё]", email["subject"] + email["text"]):
        raise AssertionError("Russian-language generation contains no Cyrillic text")


def check_email(email, require_html=False):
    if not isinstance(email, dict) or not all(isinstance(email.get(k), str) for k in ("subject", "text", "html")):
        raise AssertionError("Email must contain string subject, text, and html fields")
    if not email["subject"].strip() or not (email["text"].strip() or email["html"].strip()):
        raise AssertionError("Generated email subject/body is empty")
    if require_html and not email["html"].strip():
        raise AssertionError("Generated email HTML has no readable message text")
    if email["html"].strip():
        reader = _HTMLReadableText()
        reader.feed(email["html"])
        if not "".join(reader.parts).strip():
            raise AssertionError("Generated email HTML has no readable message text")


class _HTMLReadableText(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.parts = []
        self.hidden = 0

    def handle_starttag(self, tag, attrs):
        if tag in {"script", "style", "template"}:
            self.hidden += 1

    def handle_endtag(self, tag):
        if tag in {"script", "style", "template"} and self.hidden:
            self.hidden -= 1

    def handle_data(self, data):
        if not self.hidden:
            self.parts.append(data)


def check_revision(original, revised, context):
    check_email(revised, require_html=True)
    original_content = "\n".join(original.values()).lower()
    revised_content = "\n".join(revised.values()).lower()
    for url in set(ABSOLUTE_URL.findall(revised_content)) - set(ABSOLUTE_URL.findall(original_content)):
        raise AssertionError(f"Revision invented an absolute URL: {url}")
    for phrase in ("phishing simulation", "phishing exercise", "training exercise",
                   "this is phishing", "do not click", "verify the sender"):
        if phrase in revised_content and phrase not in original_content:
            raise AssertionError(f"Revision disclosed the simulation: {phrase}")
    expected = context.get("expected_sender") or {}
    simulated = context.get("simulated_sender") or {}
    expected_email = expected.get("email", "").lower()
    if expected_email and expected_email != simulated.get("email", "").lower():
        if expected_email in revised_content and expected_email not in original_content:
            raise AssertionError("Revision leaked evaluator-only expected sender")


def check_generated_identity(email, generation, context):
    """Catch new hardcoded destinations and evaluator-only sender identities."""
    content = "\n".join(email.values()).lower()
    source = "\n".join(str(value) for value in generation.values()).lower()
    simulated_url = (context.get("link") or {}).get("simulated_url", "").lower()
    allowed = set(ABSOLUTE_URL.findall(source)) | ({simulated_url} if simulated_url else set())
    for url in set(ABSOLUTE_URL.findall(content)) - allowed:
        raise AssertionError(f"Generated email invented an absolute URL: {url}")
    expected = (context.get("expected_sender") or {}).get("email", "").lower()
    simulated = (context.get("simulated_sender") or {}).get("email", "").lower()
    if expected and expected != simulated and expected in content and expected not in source:
        raise AssertionError("Generated email exposed evaluator-only expected sender")


def assert_snapshot(api, email, generation, context, repeats, label, baseline=None, issues=None,
                    evaluation_mode="reuse"):
    payload = {"email": email, "generation_context": generation, "evaluation_context": context}
    results = []
    for run in range(repeats):
        evaluation = api.call("/api/ai/templates/evaluate", {**payload, "evaluation_mode": evaluation_mode})
        results.append(evaluation)
        if baseline is not None and run == 0 and evaluation_mode == "reuse":
            differences = signature_differences(score_signature(baseline), score_signature(evaluation))
            if differences:
                message = f"{label}: result changed on immediate re-evaluation: {'; '.join(differences)}"
                if issues is None:
                    raise AssertionError(message)
                issues.append(message)
        if run and evaluation_mode == "reuse":
            differences = signature_differences(score_signature(results[0]), score_signature(evaluation))
            if differences:
                message = f"{label}: repeated unchanged evaluation drifted: {'; '.join(differences)}"
                if issues is None:
                    raise AssertionError(message)
                issues.append(message)
    return results[0]


def check_result(result, target):
    if result.get("status") not in {"reached", "possible_unconfirmed", "not_reached"}:
        raise AssertionError(f"Unexpected status: {result.get('status')!r}")
    if result["iterations"] < 0 or len(result["history"]) != result["iterations"] + 1:
        raise AssertionError("Accepted iteration/history accounting is inconsistent")
    check_email(result["email"], require_html=True)
    if result["status"] == "reached" and result["evaluation"]["difficulty"].get("detection_difficulty") != target:
        raise AssertionError("Agent claimed the wrong target difficulty")
    for step in result["history"]:
        check_email(step["email"], require_html=True)


def run_case(api, case_id, case, folder, profiles, repeats, alice_profile="", evaluation_mode="reuse"):
    email, generation, context = case_input(case, folder, profiles, alice_profile)
    trace = {"case": case_id, "phases": {}, "issues": []}
    if email is None:
        target = generation["target_difficulty"]
        result = api.call("/api/ai/templates/generate-and-adjust", {
            "generation_context": generation, "evaluation_context": context,
            "max_iterations": case.get("max_iterations", 3)})
        check_result(result, target)
        check_generated_identity(result["history"][0]["email"], generation, context)
        for before, after in zip(result["history"], result["history"][1:]):
            check_revision(before["email"], after["email"], context)
        email = result["email"]
        trace["phases"]["generate"] = result

    check_email(email)
    first = assert_snapshot(api, email, generation, context, repeats, "initial",
                            trace["phases"].get("generate", {}).get("evaluation"), trace["issues"],
                            evaluation_mode)
    trace["phases"]["evaluate"] = first
    if evaluation_mode == "sample" and trace["phases"].get("generate"):
        trace["independent_assessment_differences"] = signature_differences(
            score_signature(trace["phases"]["generate"]["evaluation"]), score_signature(first))
    check_expectations(first, email, case.get("api_expect", {}))
    if context["link"]["usage"] == "none" and cue_value(first, "spoofed_link_domains") != (0, 0):
        raise AssertionError("No-link context must have zero spoofed-link cues")
    if context["attachments"]["usage"] == "none" and cue_value(first, "attachments") != (0, 0):
        raise AssertionError("No-attachment context must have zero attachment cues")

    if case.get("attachment_roundtrip"):
        filename = Path(case["attachment_roundtrip"]["file"]).name
        with_file = copy.deepcopy(context)
        with_file["attachments"] = {"usage": "used", "files": [{"name": filename, "type": "text/plain"}]}
        added = assert_snapshot(api, email, generation, with_file, repeats, "attachment added",
                                issues=trace["issues"], evaluation_mode=evaluation_mode)
        if cue_value(added, "attachments")[0] < 1:
            raise AssertionError("Attachment was not detected after adding it")
        trace["phases"]["attachment_added"] = added
        removed = assert_snapshot(api, email, generation, context, repeats, "attachment removed",
                                  issues=trace["issues"], evaluation_mode=evaluation_mode)
        if cue_value(removed, "attachments") != (0, 0):
            raise AssertionError("Removed attachment still contributes a cue")
        trace["phases"]["attachment_removed"] = removed

    if case.get("manual_revision_feedback"):
        revised = api.call("/api/ai/templates/revise", {
            "email": email, "feedback": case["manual_revision_feedback"],
            **{key: generation.get(key, "") for key in
               ("target_audience", "recipient_role", "organization_context", "sender_context",
                "scenario", "custom_scenario", "language", "target_difficulty")},
            "evaluation_context": context})
        check_revision(email, revised, context)
        email = revised
        trace["phases"]["revise"] = revised
        trace["phases"]["revision_evaluation"] = assert_snapshot(
            api, email, generation, context, repeats, "revised email", issues=trace["issues"],
            evaluation_mode=evaluation_mode)

    if case.get("change_difficulty"):
        config = case["change_difficulty"]
        result = api.call("/api/ai/templates/change-difficulty", {
            "input": {"email": email, "generation_context": generation, "evaluation_context": context},
            "target": config["target"], "user_feedback": config.get("feedback", ""),
            "max_iterations": config.get("max_iterations", 3)})
        check_result(result, config["target"])
        for before, after in zip(result["history"], result["history"][1:]):
            check_revision(before["email"], after["email"], context)
        trace["phases"]["change_difficulty"] = result
        generation["target_difficulty"] = config["target"]
        email = result["email"]
        trace["phases"]["final_evaluation"] = assert_snapshot(
            api, email, generation, context, repeats, "changed difficulty",
            result["evaluation"], trace["issues"], evaluation_mode)
    return trace
