#!/usr/bin/env python3
"""Playwright smoke test for Windropolis Alice's complete template workflow."""

import argparse
import json
from pathlib import Path
import sys
import uuid

if __package__:
    from .workflow import FIXTURES, check_email, check_result, cue_value
else:
    from workflow import FIXTURES, check_email, check_result, cue_value

ROOT = Path(__file__).resolve().parent


def expect_api(page, endpoint, action, timeout=900000, evaluation_mode=None):
    with page.expect_response(lambda response: endpoint in response.url and
                              response.request.method == "POST", timeout=timeout) as waiting:
        action()
    response = waiting.value
    if not response.ok:
        raise AssertionError(f"{endpoint}: HTTP {response.status}: {response.text()[:400]}")
    if evaluation_mode is not None and response.request.post_data_json.get("evaluation_mode") != evaluation_mode:
        raise AssertionError(f"{endpoint}: expected evaluation_mode={evaluation_mode!r}")
    return response.json()


def fill_generation(page, generation):
    for key in ("target_audience", "recipient_role", "organization_context",
                "sender_context", "additional_instructions"):
        page.locator("#ai_" + key).fill(generation.get(key, ""))
    page.locator("#ai_scenario").select_option(generation["scenario"])
    if generation["scenario"] == "custom":
        page.locator("#ai_custom_scenario").fill(generation["custom_scenario"])
    page.locator("#ai_language").select_option(generation["language"])
    page.locator("#ai_difficulty").select_option(generation["target_difficulty"])


def fill_evaluation(page, context, sender_email):
    page.locator("#evaluation_prior_training").select_option(context["prior_training_exposure"])
    page.wait_for_function("() => document.querySelectorAll('#evaluation_sending_profile option').length > 1")
    options = page.locator("#evaluation_sending_profile option").all()
    matching = [option.get_attribute("value") for option in options if
                sender_email.lower() in option.inner_text().lower()]
    if len(matching) != 1:
        raise AssertionError(f"Select one existing sending profile for {sender_email}; found {len(matching)}")
    page.locator("#evaluation_sending_profile").select_option(matching[0])
    page.locator("#evaluation_simulated_sender_name").fill(context["expected_sender"]["display_name"])
    expected = context["expected_sender"]
    page.locator("#evaluation_expected_sender_name").fill(expected["display_name"])
    page.locator("#evaluation_expected_sender_email").fill(expected["email"])
    page.locator("#evaluation_situation_context").fill(context["situation_context"])
    page.locator("#evaluation_link_usage").select_option(context["link"]["usage"])
    if context["link"]["usage"] == "used":
        page.locator("#evaluation_simulated_url").fill(context["link"]["simulated_url"])
        page.locator("#evaluation_expected_domain").fill(context["link"]["expected_domain"])
    if context["attachments"]["usage"] == "none":
        page.locator("#evaluation_no_attachments").check()


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--url", default="https://127.0.0.1:3333", help="GoPhish admin origin")
    parser.add_argument("--headless", action="store_true", help="Use saved login from browser_profile")
    parser.add_argument("--keep-template", action="store_true", help="Retain the test template after verification")
    parser.add_argument("--output", type=Path, default=ROOT / "results")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args(argv)
    folder = FIXTURES / "11_alice_full_workflow"
    case = json.loads((folder / "case.json").read_text(encoding="utf-8"))
    generation, context = case["generation_context"], case["evaluation_context"]
    if args.dry_run:
        print("Windropolis Alice: generate -> fresh re-evaluation -> attach -> remove -> reattach -> remove -> revise -> change difficulty -> confirm -> fresh evaluation -> fresh re-evaluation -> save -> reopen")
        return 0

    try:
        from playwright.sync_api import sync_playwright
    except ImportError as exc:
        parser.error("Install Playwright: python3 -m pip install playwright && python3 -m playwright install chromium")

    args.output.mkdir(parents=True, exist_ok=True)
    template_name = "Windropolis AI Alice " + uuid.uuid4().hex[:10]
    trace = {"template_name": template_name, "phases": {}}
    with sync_playwright() as playwright:
        browser = playwright.chromium.launch_persistent_context(
            str(ROOT / "browser_profile"), headless=args.headless,
            ignore_https_errors=args.url.startswith("https://127.0.0.1:") or
                                args.url.startswith("https://localhost:"),
            viewport={"width": 1366, "height": 900})
        browser.set_default_timeout(30000)
        page = browser.pages[0] if browser.pages else browser.new_page()
        try:
            page.goto(args.url.rstrip("/") + "/templates", wait_until="domcontentloaded")
            if page.get_by_role("button", name="New Template").count() == 0:
                if args.headless:
                    raise AssertionError("Sign in once using the visible browser first")
                input("Sign in to GoPhish in the opened browser, then press Enter here: ")
                page.goto(args.url.rstrip("/") + "/templates", wait_until="domcontentloaded")

            page.get_by_role("button", name="New Template").click()
            page.locator('#modal button[data-target="#GenerateWithAIModal"]').click()
            fill_generation(page, generation)
            page.locator("#aiGenerateButton").click()
            fill_evaluation(page, context, case["sending_profile_from"])
            result = expect_api(page, "/api/ai/templates/generate-and-adjust",
                                lambda: page.locator("#evaluationContextSubmitButton").click())
            check_result(result, generation["target_difficulty"])
            trace["phases"]["generate"] = result
            page.locator("#aiGenerationResult").wait_for(state="visible")
            assert page.locator("#ai_result_subject").input_value() == result["email"]["subject"]

            refreshed = expect_api(page, "/api/ai/templates/evaluate",
                                   lambda: page.locator("#aiReevaluateButton").click(),
                                   evaluation_mode="refresh")
            trace["phases"]["generation_fresh_evaluation"] = refreshed
            page.locator("#aiGenerationResult").wait_for(state="visible")

            filename = folder / case["attachment_roundtrip"]["file"]
            added = expect_api(page, "/api/ai/templates/evaluate",
                               lambda: page.locator("#aiResultAttachmentUpload").set_input_files(str(filename)))
            if cue_value(added, "attachments")[0] < 1:
                raise AssertionError("Adding an attachment did not increase its cue")
            trace["phases"]["attachment_added"] = added
            page.locator("#aiGenerationResult").wait_for(state="visible")
            removed = expect_api(page, "/api/ai/templates/evaluate",
                                 lambda: page.locator("#aiResultAttachmentsSummary .ai-remove-attachment").click())
            if cue_value(removed, "attachments") != (0, 0):
                raise AssertionError("Removed attachment still contributes a cue")
            trace["phases"]["attachment_removed"] = removed
            page.locator("#aiGenerationResult").wait_for(state="visible")
            second_added = expect_api(page, "/api/ai/templates/evaluate",
                                      lambda: page.locator("#aiResultAttachmentUpload").set_input_files(str(filename)))
            trace["phases"]["attachment_restored"] = second_added
            page.locator("#aiGenerationResult").wait_for(state="visible")
            second_removed = expect_api(page, "/api/ai/templates/evaluate",
                                        lambda: page.locator("#aiResultAttachmentsSummary .ai-remove-attachment").click())
            trace["phases"]["attachment_removed_again"] = second_removed
            page.locator("#aiGenerationResult").wait_for(state="visible")

            page.locator("#aiReviseButton").click()
            page.locator("#email_revision_feedback").fill(case["manual_revision_feedback"])
            revised = expect_api(page, "/api/ai/templates/revise",
                                 lambda: page.locator("#emailRevisionSubmitButton").click())
            check_email(revised)
            trace["phases"]["revise"] = revised
            page.locator("#aiGenerationResult").wait_for(state="visible")
            assert page.locator("#ai_result_subject").input_value() == revised["subject"]

            page.locator("#aiChangeDifficultyButton").click()
            difficulty = case["change_difficulty"]
            page.locator("#change_difficulty_target").select_option(difficulty["target"])
            page.locator("#change_difficulty_feedback").fill(difficulty["feedback"])
            page.locator("#change_difficulty_iterations").select_option(str(difficulty["max_iterations"]))
            adjusted = expect_api(page, "/api/ai/templates/change-difficulty",
                                  lambda: page.locator("#changeDifficultySubmitButton").click())
            check_result(adjusted, difficulty["target"])
            trace["phases"]["change_difficulty"] = adjusted
            page.locator("#aiGenerationResult").wait_for(state="visible")

            page.locator("#aiConfirmButton").click()
            page.locator("#modal").wait_for(state="visible")
            final = adjusted["email"]
            assert page.locator("#subject").input_value() == final["subject"]
            assert page.locator("#text_editor").input_value() == final["text"]

            page.locator("#evaluateEmailButton").click()
            page.locator("#evaluation_target_audience").wait_for(state="visible")
            if page.locator("#evaluation_target_audience").input_value() != generation["target_audience"]:
                raise AssertionError("Audience was not prefilled after Confirm")
            if page.locator("#evaluation_expected_sender_email").input_value() != context["expected_sender"]["email"]:
                raise AssertionError("Expected sender was not prefilled after Confirm")
            for selector, expected in (
                ("#evaluation_prior_training", context["prior_training_exposure"]),
                ("#evaluation_simulated_url", context["link"]["simulated_url"]),
                ("#evaluation_expected_domain", context["link"]["expected_domain"]),
                ("#evaluation_situation_context", context["situation_context"]),
            ):
                if page.locator(selector).input_value() != expected:
                    raise AssertionError(f"{selector} was not prefilled after Confirm")
            selected = page.locator("#evaluation_sending_profile option:checked").inner_text()
            if case["sending_profile_from"] not in selected:
                raise AssertionError("Alice's Sending Profile was not preselected")
            evaluated = expect_api(page, "/api/ai/templates/evaluate",
                                   lambda: page.locator("#evaluationContextSubmitButton").click(),
                                   evaluation_mode="refresh")
            trace["phases"]["evaluate_after_confirm"] = evaluated
            page.locator("#evaluationResultContent").wait_for(state="visible")
            reevaluated = expect_api(page, "/api/ai/templates/evaluate",
                                    lambda: page.locator("#reevaluateEvaluatedEmailButton").click(),
                                    evaluation_mode="refresh")
            trace["phases"]["result_fresh_evaluation"] = reevaluated
            page.locator("#evaluationResultContent").wait_for(state="visible")
            page.locator("#evaluationResultCloseButton").click()

            page.locator("#name").fill(template_name)
            html_before_save = page.evaluate("() => CKEDITOR.instances.html_editor.getData()")
            saved = expect_api(page, "/api/templates/", lambda: page.locator("#modalSubmit").click())
            trace["phases"]["saved_id"] = saved.get("id")
            page.reload(wait_until="domcontentloaded")
            row = page.locator("#templateTable tbody tr").filter(has_text=template_name)
            row.wait_for(state="visible")
            row.locator('button[title="Edit Template"]').click()
            if page.locator("#subject").input_value() != final["subject"] or page.locator("#text_editor").input_value() != final["text"]:
                raise AssertionError("Saved template changed after reopening")
            html_after_reopen = page.evaluate("() => CKEDITOR.instances.html_editor.getData()")
            if html_after_reopen.strip() != html_before_save.strip():
                raise AssertionError("Saved HTML changed after reopening")
            page.locator('#modal button.close').click()

            if not args.keep_template:
                row = page.locator("#templateTable tbody tr").filter(has_text=template_name)
                row.locator('button[title="Delete Template"]').click()
                page.get_by_role("button", name="Delete " + template_name).click()
                trace["template_deleted"] = True
            print("[PASS] Alice full browser workflow")
        except Exception:
            page.screenshot(path=str(args.output / "alice_failure.png"), full_page=True)
            raise
        finally:
            (args.output / "alice_browser.json").write_text(
                json.dumps(trace, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
            browser.close()
    return 0


if __name__ == "__main__":
    sys.exit(main())
