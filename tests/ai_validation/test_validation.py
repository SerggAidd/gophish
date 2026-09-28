"""Offline checks of scenario coverage, API modes, and report provenance."""

import copy
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from ai_validation import fixed, run_tests, workflow


def evaluation():
    return {"criteria": [
                {"id": "attachments", "min_value": 0, "max_value": 0},
                {"id": "spoofed_link_domains", "min_value": 0, "max_value": 0},
                {"id": "sender_domain_spoofing", "min_value": 0, "max_value": 0},
             ],
            "cues": {"min_count": 0, "max_count": 0},
            "premise_alignment": {"elements": [
                {"id": "workplace_relevance", "score": 6},
                {"id": "consequences_for_not_clicking", "score": 2},
            ], "min_score": 18, "max_score": 18},
            "difficulty": {"resolved": True, "detection_difficulty": "very_difficult"},
            "context_complete": True}


class TestAPI:
    __test__ = False
    calls = []

    def __init__(self, *args):
        pass

    def call(self, path, payload=None):
        self.calls.append((path, copy.deepcopy(payload)))
        if path == "/api/smtp/":
            profiles = json.loads((run_tests.ROOT / "profiles.json").read_text(encoding="utf-8"))
            return [{"name": p["name"], "from_address": p["from_address"]} for p in profiles]
        if path.endswith("/generate-and-adjust"):
            mail = {"subject": "A routine notice", "text": "Please read this notice.",
                    "html": "<p>Please read this notice.</p>"}
            return {"email": mail, "evaluation": evaluation(), "status": "reached",
                    "iterations": 0, "history": [{"email": mail, "evaluation": evaluation()}]}
        if path.endswith("/evaluate"):
            return evaluation()
        raise AssertionError(path)


class ValidationTests(unittest.TestCase):
    def test_html_must_have_readable_message_text(self):
        with self.assertRaisesRegex(AssertionError, "HTML has no readable message text"):
            workflow.check_email({"subject": "Review", "text": "Please review the policy.",
                                  "html": '<table style="color:#333">'})
        with self.assertRaisesRegex(AssertionError, "HTML has no readable message text"):
            workflow.check_email({"subject": "Review", "text": "Please review the policy.",
                                  "html": ""}, require_html=True)
        workflow.check_email({"subject": "Review", "text": "Please review the policy.",
                              "html": "<table><tr><td>Please review the policy.</td></tr></table>"})

    def test_observations_preserve_model_repair_metadata(self):
        response = evaluation()
        response["criteria"][0]["evaluation_repair"] = "focused_response"
        event = {"response": response, "input": {"evaluation_mode": "sample"},
                 "duration_seconds": 1.2, "endpoint": "/api/ai/templates/evaluate", "input_hash": "test"}
        rows = list(run_tests.observation_rows("NIST-022", 1, "", [event]))
        self.assertEqual(rows[0]["evaluation_repair"], "focused_response")
        self.assertEqual(rows[1]["evaluation_repair"], "")
        self.assertEqual(rows[-1]["evaluation_repair"], "")

    def test_catalog_covers_all_targets_with_separate_case_files(self):
        cases = run_tests.load_cases()
        self.assertEqual(len(cases), 61)
        self.assertEqual(sum(d["track"] == "fixed" for d, _ in cases.values()), 33)
        self.assertEqual(sum(d["track"] == "workflow" for d, _ in cases.values()), 28)
        for category in workflow.DIFFICULTIES:
            self.assertGreaterEqual(sum(d.get("generation_context", {}).get("target_difficulty") == category
                                        for d, _ in cases.values()), 4)
        for case, folder in cases.values():
            self.assertEqual(folder.name, case["id"])
            text = json.dumps(case, ensure_ascii=False).lower()
            self.assertNotIn("innocorp", text)
            self.assertNotIn("alice@company.test", text)
            self.assertNotIn("qa - ", text)

    def test_profile_preflight_detects_wrong_sender(self):
        cases = run_tests.load_cases("workflow", ["GEN-VD-01-benefits"])
        profiles = TestAPI().call("/api/smtp/")
        run_tests.validate_profiles(cases, profiles)
        modified = copy.deepcopy(profiles)
        next(profile for profile in modified if profile["name"] == "Windropolis | HR")["from_address"] = "Wrong <wrong@windropolis.corporate>"
        with self.assertRaisesRegex(ValueError, "From"):
            run_tests.validate_profiles(cases, modified)

    def test_independent_evaluations_write_separate_records(self):
        TestAPI.calls = []
        with tempfile.TemporaryDirectory() as directory, \
             patch.object(workflow, "API", TestAPI), \
             patch.dict(os.environ, {"GOPHISH_API_KEY": "fake-offline-key"}):
            status = run_tests.main(["--mode", "experiment", "--group", "workflow",
                                     "--only", "GEN-VD-01-benefits", "--repeats", "2",
                                     "--study-id", "offline-smoke", "--model-id", "fake",
                                     "--output", directory])
            self.assertEqual(status, 0)
            outputs = sorted((Path(directory) / "cases" / "GEN-VD-01-benefits").glob("*.json"))
            self.assertEqual(len(outputs), 2)
            for output in outputs:
                record = json.loads(output.read_text(encoding="utf-8"))
                self.assertEqual(record["status"], "PASS")
                self.assertEqual(record["events"][-1]["input"]["evaluation_mode"], "sample")
                self.assertNotIn("fake-offline-key", output.read_text(encoding="utf-8"))
            report = json.loads((Path(directory) / "report.json").read_text(encoding="utf-8"))
            self.assertEqual(report["model_id"], "fake")
            self.assertEqual(len([p for p, _ in TestAPI.calls if p.endswith("/generate-and-adjust")]), 2)
            packets = list((Path(directory) / "review_packets").glob("*.json"))
            self.assertEqual(len(packets), 2)
            for packet_file in packets:
                packet = json.loads(packet_file.read_text(encoding="utf-8"))
                self.assertNotIn("target_difficulty", packet["generation_context"])
                self.assertNotIn("evaluation", packet)
            annotations = (Path(directory) / "annotation_template.csv").read_text(encoding="utf-8")
            self.assertEqual(len(annotations.splitlines()), 5)  # header and A/B for each run
            self.assertNotIn("GEN-VD", annotations)

    def test_fixed_contrasts_compare_criteria_in_addition_to_premise(self):
        lower, higher = evaluation(), evaluation()
        higher["criteria"][2]["min_value"] = 1
        self.assertEqual(fixed.comparison_errors({"comparisons": [
            {"lower": "a", "higher": "b", "criterion": "sender_domain_spoofing"}]},
            {"a": [lower], "b": [higher]}, {"a", "b"}), [])

    def test_fixed_control_reuses_snapshot_and_keeps_observation_separate(self):
        TestAPI.calls = []
        with tempfile.TemporaryDirectory() as directory, \
             patch.object(workflow, "API", TestAPI), \
             patch.dict(os.environ, {"GOPHISH_API_KEY": "fake-offline-key"}):
            self.assertEqual(run_tests.main(["--group", "fixed", "--only", "NIST-029",
                                             "--repeats", "2", "--output", directory]), 0)
            requests = [data for path, data in TestAPI.calls if path.endswith("/evaluate")]
            self.assertEqual([data["evaluation_mode"] for data in requests], ["reuse", "reuse"])
            target = Path(directory) / "cases" / "NIST-029"
            self.assertEqual(len(list(target.glob("run-*.json"))), 2)

    def test_experimental_mismatch_is_recorded_as_deviation(self):
        with tempfile.TemporaryDirectory() as directory, \
             patch.object(workflow, "API", TestAPI), \
             patch.dict(os.environ, {"GOPHISH_API_KEY": "fake-offline-key"}):
            self.assertEqual(run_tests.main(["--group", "fixed", "--only", "NIST-026",
                                             "--mode", "experiment", "--repeats", "1",
                                             "--study-id", "pilot", "--model-id", "fake",
                                             "--output", directory]), 0)
            run = json.loads((Path(directory) / "cases" / "NIST-026" / "run-001.json").read_text(encoding="utf-8"))
            self.assertEqual(run["events"][0]["input"]["evaluation_mode"], "sample")
            self.assertTrue(run["deviations"])
            self.assertEqual(run["status"], "PASS")


if __name__ == "__main__":
    unittest.main()
