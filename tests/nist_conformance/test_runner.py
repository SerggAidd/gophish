"""Offline checks for the API test runner's assertion and comparison logic."""

import unittest

import run_tests


class RunnerTests(unittest.TestCase):
    def response(self, criterion=1, consequence=2):
        return {
            "criteria": [{"id": "missing_branding", "min_value": criterion, "max_value": criterion}],
            "premise_alignment": {"elements": [{"id": "consequences_for_not_clicking", "score": consequence}]},
        }

    def test_merging_overrides_preserves_unrelated_context(self):
        base = {"email": {"text": "original", "html": "<p>original</p>"}}
        merged = run_tests.merge(base, {"email": {"text": "changed"}})
        self.assertEqual(merged["email"]["html"], "<p>original</p>")
        self.assertEqual(base["email"]["text"], "original")

    def test_individual_criteria_and_premise(self):
        expected = {"criteria": {"missing_branding": {"min_value": 1, "max_value": 1}},
                    "premise": {"consequences_for_not_clicking": {"min": 2, "max": 4}}}
        self.assertEqual(run_tests.assert_response(self.response(), expected), [])
        self.assertEqual(len(run_tests.assert_response(self.response(0, 6), expected)), 3)

    def test_pairwise_comparison_detects_reversal(self):
        suite = {"comparisons": [{"lower": "low", "higher": "high", "element": "consequences_for_not_clicking"}]}
        self.assertEqual(run_tests.comparison_errors(suite, {"low": [self.response()], "high": [self.response(1, 6)]}, {"low", "high"}), [])
        self.assertEqual(len(run_tests.comparison_errors(suite, {"low": [self.response(1, 6)], "high": [self.response()]}, {"low", "high"})), 1)

    def test_repeated_semantic_drift(self):
        self.assertEqual(run_tests.stability_errors({"x": [self.response(), self.response()]}), [])
        self.assertEqual(len(run_tests.stability_errors({"x": [self.response(), self.response(0, 4)]})), 2)


if __name__ == "__main__":
    unittest.main()
