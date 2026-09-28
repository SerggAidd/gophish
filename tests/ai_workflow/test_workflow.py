"""Offline tests for the API workflow runner; no GoPhish or Ollama required."""

import copy
import unittest

import run_tests as workflow


def evaluation(attachment=0, difficulty="very_difficult"):
    return {
        "criteria": [
            {"id": "attachments", "min_value": attachment, "max_value": attachment},
            {"id": "spoofed_link_domains", "min_value": 0, "max_value": 0},
            {"id": "poses_as_authority", "min_value": 1, "max_value": 1},
        ],
        "premise_alignment": {"elements": [
            {"id": "workplace_relevance", "score": 6},
            {"id": "consequences_for_not_clicking", "score": 2},
        ], "min_score": 18, "max_score": 18},
        "cues": {"min_count": attachment + 2, "max_count": attachment + 2},
        "difficulty": {"detection_difficulty": difficulty, "resolved": True},
    }


EMAIL = {"subject": "Policy review", "text": "Read the policy at {{.URL}}",
         "html": '<a href="{{.URL}}">Read the policy</a>'}


class FakeAPI:
    def __init__(self):
        self.calls = []

    def call(self, path, payload):
        self.calls.append((path, copy.deepcopy(payload)))
        if path.endswith("/generate-and-adjust"):
            return {"email": EMAIL, "evaluation": evaluation(), "status": "reached",
                    "iterations": 0, "history": [{"email": EMAIL, "evaluation": evaluation()}]}
        if path.endswith("/revise"):
            return {**EMAIL, "subject": "Updated policy review"}
        if path.endswith("/change-difficulty"):
            revised = {**payload["input"]["email"], "subject": "Updated policy review request"}
            return {"email": revised, "evaluation": evaluation(difficulty="moderately_difficult"),
                    "status": "reached", "iterations": 1,
                    "history": [{"email": payload["input"]["email"], "evaluation": evaluation()},
                                {"email": revised, "evaluation": evaluation(difficulty="moderately_difficult")}]}
        if path.endswith("/evaluate"):
            attachment = int(payload["evaluation_context"]["attachments"]["usage"] == "used")
            target = payload["generation_context"].get("target_difficulty", "very_difficult")
            return evaluation(attachment, target)
        raise AssertionError(path)


class DriftingAPI(FakeAPI):
    def __init__(self):
        super().__init__()
        self.evaluations = 0

    def call(self, path, payload):
        result = super().call(path, payload)
        if path.endswith("/evaluate"):
            self.evaluations += 1
            if self.evaluations == 2:
                result["criteria"][2]["min_value"] = 0
                result["criteria"][2]["max_value"] = 0
        return result


class WorkflowTests(unittest.TestCase):
    def test_all_original_cases_and_alice_are_bundled(self):
        cases = workflow.load_cases()
        self.assertEqual(len(cases), 28)
        self.assertIn("11_alice_full_workflow", cases)
        targets = {case["generation_context"].get("target_difficulty") for case, _ in cases.values()}
        self.assertTrue(workflow.DIFFICULTIES <= targets)
        for target in workflow.DIFFICULTIES:
            self.assertGreaterEqual(sum(case["generation_context"].get("target_difficulty") == target
                                        for case, _ in cases.values()), 4)

    def test_sending_profile_sender_comes_from_real_from_address(self):
        case, folder = workflow.load_cases(["01_generate_complete_very_difficult"]).popitem()[1]
        profile = [{"name": "Windropolis | IT Desk", "from_address":
                    "IT Service Desk <notifications@windropolis.corporate>"}]
        email, generation, context = workflow.case_input(case, folder, profile)
        self.assertIsNone(email)
        self.assertEqual(generation["target_difficulty"], "very_difficult")
        self.assertEqual(context["simulated_sender"]["email"], "notifications@windropolis.corporate")
        with self.assertRaisesRegex(ValueError, "Sending profile"):
            workflow.case_input(case, folder, [])

    def test_same_email_score_drift_is_reported(self):
        before = workflow.score_signature(evaluation())
        changed = evaluation()
        changed["criteria"][2]["min_value"] = 0
        changed["criteria"][2]["max_value"] = 0
        changed["premise_alignment"]["elements"][0]["score"] = 4
        differences = workflow.signature_differences(before, workflow.score_signature(changed))
        self.assertTrue(any("poses_as_authority" in item for item in differences))
        self.assertTrue(any("workplace_relevance" in item for item in differences))

    def test_revision_rejects_new_external_url(self):
        revised = {**EMAIL, "text": EMAIL["text"] + " https://unexpected.test"}
        with self.assertRaisesRegex(AssertionError, "absolute URL"):
            workflow.check_revision(EMAIL, revised, {})

    def test_initial_generation_rejects_unprovided_destination_and_expected_sender(self):
        generation = {"sender_context": "Simulated sender"}
        context = {"link": {"simulated_url": "https://portal.windropolis.corporate/policy"},
                   "simulated_sender": {"email": "sender@windropo1is.corporate"},
                   "expected_sender": {"email": "sender@windropolis.corporate"}}
        with self.assertRaisesRegex(AssertionError, "invented an absolute URL"):
            workflow.check_generated_identity({**EMAIL, "text": "https://unrelated.test/new"},
                                              generation, context)
        with self.assertRaisesRegex(AssertionError, "evaluator-only"):
            workflow.check_generated_identity({**EMAIL, "text": "sender@windropolis.corporate"},
                                              generation, context)
        workflow.check_generated_identity({**EMAIL, "text": "https://portal.windropolis.corporate/policy"},
                                          generation, context)

    def test_fixture_semantic_assertions_detect_wrong_domain_or_language(self):
        with self.assertRaisesRegex(AssertionError, "sender_domain_spoofing"):
            workflow.check_expectations(evaluation(), EMAIL,
                {"criteria": {"sender_domain_spoofing": {"min": 1}}})
        with self.assertRaisesRegex(AssertionError, "Cyrillic"):
            workflow.check_expectations(evaluation(), EMAIL, {"email_language": "ru"})

    def test_alice_runs_every_api_phase_and_restores_attachment_context(self):
        case, folder = workflow.load_cases(["11_alice_full_workflow"]).popitem()[1]
        profiles = [{"name": "Windropolis | Alice Smith", "from_address": "Alice Smith <alice.smith@windropolis.corporate>"}]
        api = FakeAPI()
        trace = workflow.run_case(api, "11_alice_full_workflow", case, folder, profiles, 2, "")
        self.assertEqual(trace["issues"], [])
        self.assertEqual(set(trace["phases"]), {"generate", "evaluate", "attachment_added",
            "attachment_removed", "revise", "revision_evaluation", "change_difficulty",
            "final_evaluation"})
        additions = [payload for path, payload in api.calls if path.endswith("/evaluate") and
                     payload["evaluation_context"]["attachments"]["usage"] == "used"]
        self.assertEqual(len(additions), 2)
        self.assertEqual(additions[0]["evaluation_context"]["attachments"]["files"][0]["name"],
                         "ui_attachment_smoke_test.txt")
        self.assertEqual(api.calls[-1][1]["evaluation_context"]["attachments"]["usage"], "none")

    def test_drift_is_failure_but_following_alice_steps_continue(self):
        case, folder = workflow.load_cases(["11_alice_full_workflow"]).popitem()[1]
        profiles = [{"name": "Windropolis | Alice Smith", "from_address": "Alice Smith <alice.smith@windropolis.corporate>"}]
        trace = workflow.run_case(DriftingAPI(), "11_alice_full_workflow", case, folder, profiles, 2, "")
        self.assertTrue(any("poses_as_authority" in issue for issue in trace["issues"]))
        self.assertIn("final_evaluation", trace["phases"])

    def test_independent_samples_record_drift_without_failures(self):
        case, folder = workflow.load_cases(["11_alice_full_workflow"]).popitem()[1]
        profiles = [{"name": "Windropolis | Alice Smith", "from_address":
                     "Alice Smith <alice.smith@windropolis.corporate>"}]
        trace = workflow.run_case(DriftingAPI(), "11_alice_full_workflow", case, folder,
                                  profiles, 2, evaluation_mode="sample")
        self.assertEqual(trace["issues"], [])
        self.assertIn("independent_assessment_differences", trace)


if __name__ == "__main__":
    unittest.main()
