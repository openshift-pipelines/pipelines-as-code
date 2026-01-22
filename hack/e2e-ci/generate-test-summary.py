#!/usr/bin/env python3
"""Generate GitHub Actions job summary from JUnit XML test results with controller logs."""

import glob
import html
import json
import os
import sys
import xml.etree.ElementTree as ET
from datetime import datetime, timedelta, timezone


def format_duration(seconds):
    """Format duration in a human-readable way."""
    if seconds < 1:
        return f"{seconds * 1000:.0f}ms"
    elif seconds < 60:
        return f"{seconds:.1f}s"
    else:
        minutes = int(seconds // 60)
        secs = seconds % 60
        return f"{minutes}m {secs:.0f}s"


def parse_log_timestamp(ts):
    """Parse timestamp from log entry to datetime object (converted to local time)."""
    if isinstance(ts, (int, float)):
        return datetime.fromtimestamp(ts)
    elif isinstance(ts, str):
        try:
            is_utc = ts.endswith("Z")
            ts_clean = ts.rstrip("Z")
            # Remove timezone offset if present (e.g., +00:00)
            if "+" in ts_clean:
                ts_clean = ts_clean.split("+")[0]
                is_utc = True  # Has explicit timezone, treat as UTC for simplicity

            dt = datetime.fromisoformat(ts_clean)
            # Ensure timezone-naive
            if dt.tzinfo is not None:
                dt = dt.replace(tzinfo=None)
            # Convert UTC to local time if the timestamp had 'Z' suffix
            if is_utc:
                local_offset = datetime.now(timezone.utc).astimezone().utcoffset()
                if local_offset:
                    dt = dt + local_offset
            return dt
        except ValueError:
            pass
    return None


def parse_controller_logs(logs_file):
    """Parse controller logs file and return list of log entries with parsed timestamps."""
    if not logs_file or not os.path.exists(logs_file):
        return []

    logs = []
    try:
        with open(logs_file) as f:
            for line in f:
                line = line.strip()
                if not line or not line.startswith("{"):
                    continue
                try:
                    entry = json.loads(line)
                    entry["_parsed_ts"] = parse_log_timestamp(entry.get("ts"))
                    logs.append(entry)
                except json.JSONDecodeError:
                    continue
    except (OSError, IOError) as e:
        print(f"Warning: Could not read logs file {logs_file}: {e}", file=sys.stderr)
        return []

    return logs


def parse_test_json_output(json_files):
    """Parse gotestsum JSON output files to extract test output by test name.

    The JSON file contains lines like:
    {"Time":"...","Action":"output","Package":"test","Test":"TestFoo","Output":"line\n"}
    {"Time":"...","Action":"pass","Package":"test","Test":"TestFoo","Elapsed":1.5}
    """
    test_outputs = {}  # test_name -> list of output lines

    for json_file in json_files:
        if not os.path.exists(json_file):
            continue

        try:
            with open(json_file) as f:
                for line in f:
                    line = line.strip()
                    if not line:
                        continue
                    try:
                        entry = json.loads(line)
                        action = entry.get("Action")
                        test_name = entry.get("Test")

                        # Only capture output actions for specific tests
                        if action == "output" and test_name:
                            output = entry.get("Output", "")
                            if test_name not in test_outputs:
                                test_outputs[test_name] = []
                            test_outputs[test_name].append(output)
                    except json.JSONDecodeError:
                        continue
        except (OSError, IOError) as e:
            print(
                f"Warning: Could not read JSON file {json_file}: {e}", file=sys.stderr
            )
            continue

    # Join output lines for each test
    return {name: "".join(lines) for name, lines in test_outputs.items()}


def filter_logs_by_time(logs, start_time, end_time, levels=None):
    """Filter logs by time range and optionally by level."""
    if levels is None:
        levels = {"error", "warn"}

    filtered = []
    for entry in logs:
        entry_ts = entry.get("_parsed_ts")
        if not entry_ts:
            continue

        # Check time range
        if start_time and entry_ts < start_time:
            continue
        if end_time and entry_ts > end_time:
            continue

        # Check level
        entry_level = entry.get("level", "info").lower()
        if levels and entry_level not in levels:
            continue

        filtered.append(entry)

    return filtered


def format_log_entry_for_summary(entry):
    """Format a single log entry for the summary output, including all fields."""
    ts = entry.get("_parsed_ts")
    ts_str = ts.strftime("%H:%M:%S.%f")[:-3] if ts else "??:??:??"
    level = entry.get("level", "info").upper()
    msg = entry.get("msg", "")
    caller = entry.get("caller", "")

    # Collect all additional fields (excluding internal/common ones)
    skip_keys = {"ts", "level", "msg", "caller", "_parsed_ts", "logger", "commit"}
    extra_fields = []
    for key, value in entry.items():
        if key not in skip_keys and value:
            # Format the value (handle nested objects)
            if isinstance(value, dict):
                value = json.dumps(value)
            extra_fields.append(f"{key}={value}")

    # Build the log line
    base_line = f"[{ts_str}] [{level}] {caller}: {msg}"
    if extra_fields:
        return f"{base_line} | {' '.join(extra_fields)}"
    return base_line


def parse_junit_xml(xml_files):
    """Parse JUnit XML files and extract test results with timing info."""
    total_tests = 0
    total_failures = 0
    total_errors = 0
    total_skipped = 0
    total_time = 0.0
    failed_tests = []
    all_tests = []
    suite_start_time = None

    for xml_file in xml_files:
        try:
            tree = ET.parse(xml_file)
            root = tree.getroot()

            # Handle both single testsuite and testsuites root elements
            if root.tag == "testsuites":
                testsuites = root.findall("testsuite")
            else:
                testsuites = [root]

            for testsuite in testsuites:
                total_tests += int(testsuite.get("tests", 0))
                total_failures += int(testsuite.get("failures", 0))
                total_errors += int(testsuite.get("errors", 0))
                total_skipped += int(testsuite.get("skipped", 0))
                total_time += float(testsuite.get("time", 0))

                # Try to get suite start timestamp
                ts_str = testsuite.get("timestamp")
                if ts_str and not suite_start_time:
                    try:
                        # Format: 2026-01-22T06:11:50Z or similar
                        ts_clean = ts_str.rstrip("Z")
                        if "+" in ts_clean:
                            ts_clean = ts_clean.split("+")[0]
                        suite_start_time = datetime.fromisoformat(ts_clean)
                        # Ensure timezone-naive
                        if suite_start_time.tzinfo is not None:
                            suite_start_time = suite_start_time.replace(tzinfo=None)
                    except ValueError:
                        pass

                # Track cumulative time to estimate test start times
                cumulative_time = 0.0

                for testcase in testsuite.findall("testcase"):
                    test_name = testcase.get("name", "Unknown")
                    classname = testcase.get("classname", "")
                    test_time = float(testcase.get("time", 0))

                    # Calculate approximate start/end times for this test
                    test_start = None
                    test_end = None
                    if suite_start_time:
                        test_start = suite_start_time + timedelta(
                            seconds=cumulative_time
                        )
                        test_end = test_start + timedelta(seconds=test_time)

                    cumulative_time += test_time

                    failure = testcase.find("failure")
                    error = testcase.find("error")
                    skipped = testcase.find("skipped")

                    # Capture system-out and system-err (test stdout/stderr)
                    system_out_elem = testcase.find("system-out")
                    system_err_elem = testcase.find("system-err")
                    system_out = (
                        system_out_elem.text if system_out_elem is not None else ""
                    )
                    system_err = (
                        system_err_elem.text if system_err_elem is not None else ""
                    )

                    # Determine test status
                    if failure is not None:
                        status = "failed"
                        message = failure.get("message", "")
                        details = failure.text or ""
                    elif error is not None:
                        status = "error"
                        message = error.get("message", "")
                        details = error.text or ""
                    elif skipped is not None:
                        status = "skipped"
                        message = skipped.get("message", "")
                        details = ""
                    else:
                        status = "passed"
                        message = ""
                        details = ""

                    test_data = {
                        "name": test_name,
                        "classname": classname,
                        "time": test_time,
                        "status": status,
                        "message": message,
                        "details": details,
                        "system_out": system_out,
                        "system_err": system_err,
                        "start_time": test_start,
                        "end_time": test_end,
                    }

                    all_tests.append(test_data)

                    if status in ("failed", "error"):
                        failed_tests.append(test_data)

        except ET.ParseError as e:
            print(f"Warning: Could not parse {xml_file}: {e}", file=sys.stderr)
            continue

    return {
        "total_tests": total_tests,
        "total_failures": total_failures,
        "total_errors": total_errors,
        "total_skipped": total_skipped,
        "total_time": total_time,
        "failed_tests": failed_tests,
        "all_tests": all_tests,
        "suite_start_time": suite_start_time,
    }


def generate_summary(
    results,
    provider,
    output_file,
    controller_logs=None,
    show_logs_for="failed",
    debug=False,
    log_levels=None,
    test_outputs=None,
):
    """Generate markdown summary from test results with optional controller logs.

    Args:
        results: Parsed test results
        provider: Test provider name
        output_file: Output file path
        controller_logs: Parsed controller logs (optional)
        show_logs_for: Which tests to show logs for: "failed", "all", or "none"
        debug: Enable debug output
        log_levels: Set of log levels to include, or None for all levels
        test_outputs: Dict mapping test names to their stdout/stderr output
    """
    if test_outputs is None:
        test_outputs = {}
    total_tests = results["total_tests"]
    total_failures = results["total_failures"]
    total_errors = results["total_errors"]
    total_skipped = results["total_skipped"]
    total_time = results["total_time"]
    failed_tests = results["failed_tests"]
    all_tests = results["all_tests"]

    passed = total_tests - total_failures - total_errors - total_skipped

    with open(output_file, "a") as f:
        f.write(f"## E2E Test Results - {provider}\n\n")

        # Status emoji
        if total_failures > 0 or total_errors > 0:
            status_emoji = "❌"
        elif total_tests == 0:
            status_emoji = "⚠️"
        else:
            status_emoji = "✅"

        f.write(f"{status_emoji} **{passed}/{total_tests}** tests passed")
        if total_skipped > 0:
            f.write(f" ({total_skipped} skipped)")
        f.write(f" in **{total_time:.1f}s**\n\n")

        # Summary table
        f.write("| Metric | Count |\n")
        f.write("|--------|-------|\n")
        f.write(f"| ✅ Passed | {passed} |\n")
        f.write(f"| ❌ Failed | {total_failures} |\n")
        f.write(f"| 💥 Errors | {total_errors} |\n")
        f.write(f"| ⏭️ Skipped | {total_skipped} |\n")
        f.write(f"| **Total** | **{total_tests}** |\n\n")

        # All tests timing table (sorted by duration, longest first)
        if all_tests:
            sorted_tests = sorted(all_tests, key=lambda x: x["time"], reverse=True)
            f.write("### All Tests\n\n")
            f.write("| Status | Test | Duration |\n")
            f.write("|--------|------|----------|\n")

            for test in sorted_tests:
                name = html.escape(test["name"]).replace("|", "\\|").replace("\n", " ")
                duration = format_duration(test["time"])
                status = test["status"]

                if status == "passed":
                    status_icon = "✅"
                elif status == "failed":
                    status_icon = "❌"
                elif status == "error":
                    status_icon = "💥"
                else:
                    status_icon = "⏭️"

                f.write(f"| {status_icon} | `{name}` | {duration} |\n")

            f.write("\n")
        elif total_tests > 0:
            f.write("🎉 All tests passed!\n\n")

        # Determine which tests to show details/logs for
        if show_logs_for == "all":
            tests_to_detail = all_tests
            section_title = "### Test Details (with Controller Logs)\n\n"
        elif show_logs_for == "failed":
            tests_to_detail = failed_tests
            section_title = "### Failure Details\n\n"
        else:
            tests_to_detail = []
            section_title = ""

        # Test details with controller logs
        if tests_to_detail:
            f.write(section_title)
            for i, test in enumerate(tests_to_detail, 1):
                # Add status icon for "all" mode
                status = test["status"]
                if status == "passed":
                    status_icon = "✅"
                elif status == "failed":
                    status_icon = "❌"
                elif status == "error":
                    status_icon = "💥"
                else:
                    status_icon = "⏭️"

                if show_logs_for == "all":
                    summary_prefix = f"{status_icon} {i}. "
                else:
                    summary_prefix = f"{i}. "

                f.write(
                    f"<details>\n<summary><strong>{summary_prefix}"
                    f"{html.escape(test['name'])}</strong> "
                    f"({format_duration(test['time'])})</summary>\n\n"
                )
                if test["classname"]:
                    f.write(f"**Package:** `{html.escape(test['classname'])}`\n\n")

                # Show test time window
                if test.get("start_time") and test.get("end_time"):
                    start_str = test["start_time"].strftime("%H:%M:%S")
                    end_str = test["end_time"].strftime("%H:%M:%S")
                    f.write(f"**Time window:** {start_str} - {end_str}\n\n")

                if test["message"]:
                    f.write(f"**Message:** {html.escape(test['message'])}\n\n")

                if test["details"]:
                    f.write("**Stack trace / Details:**\n```\n")
                    f.write(test["details"])
                    f.write("\n```\n\n")

                # Include test output from JSON (preferred) or XML system-out/system-err
                test_name = test["name"]
                test_output = test_outputs.get(test_name, "")

                # If no exact match, try to find by suffix (for subtests)
                if not test_output:
                    for name, output in test_outputs.items():
                        if name.endswith(test_name) or test_name.endswith(name):
                            test_output = output
                            break

                if test_output:
                    f.write("**Test Output:**\n```\n")
                    f.write(test_output)
                    f.write("```\n\n")
                elif test.get("system_out") or test.get("system_err"):
                    # Fall back to XML system-out/system-err
                    if test.get("system_out"):
                        f.write("**Test Output (stdout):**\n```\n")
                        f.write(test["system_out"])
                        f.write("\n```\n\n")
                    if test.get("system_err"):
                        f.write("**Test Output (stderr):**\n```\n")
                        f.write(test["system_err"])
                        f.write("\n```\n\n")

                # Include controller logs for this test's time window
                if controller_logs:
                    # Use configured log levels, or default based on test status
                    if log_levels is not None:
                        filter_levels = log_levels
                    elif status in ("failed", "error"):
                        filter_levels = {"error", "warn"}
                    else:
                        filter_levels = {"error", "warn", "info"}

                    test_logs = []

                    if test.get("start_time") and test.get("end_time"):
                        # Add buffer to capture logs slightly before/after the test
                        buffer = timedelta(seconds=5)
                        start_with_buffer = test["start_time"] - buffer
                        end_with_buffer = test["end_time"] + buffer

                        if debug:
                            print(
                                f"Filtering logs for {test['name']}: "
                                f"{start_with_buffer} to {end_with_buffer}",
                                file=sys.stderr,
                            )

                        test_logs = filter_logs_by_time(
                            controller_logs,
                            start_with_buffer,
                            end_with_buffer,
                            levels=filter_levels,
                        )

                        if debug:
                            print(f"  Found {len(test_logs)} logs", file=sys.stderr)
                    else:
                        # No time window - show note about missing timestamp
                        if debug:
                            print(
                                f"No time window for {test['name']}",
                                file=sys.stderr,
                            )

                    if test_logs:
                        f.write(
                            f"**Controller Logs** ({len(test_logs)} entries):\n```\n"
                        )
                        for entry in test_logs:
                            f.write(format_log_entry_for_summary(entry) + "\n")
                        f.write("```\n\n")
                    elif show_logs_for == "all":
                        time_info = ""
                        if test.get("start_time") and test.get("end_time"):
                            time_info = (
                                f" (searched {test['start_time'].strftime('%H:%M:%S')}"
                                f" to {test['end_time'].strftime('%H:%M:%S')})"
                            )
                        f.write(
                            f"**Controller Logs:** No matching logs found{time_info}\n\n"
                        )

                f.write("</details>\n\n")


def generate_combined_summary(
    artifacts_dir,
    output_file,
    show_logs_for="failed",
    debug=False,
    log_levels=None,
):
    """Generate a combined summary for all providers from artifacts directory.

    Args:
        artifacts_dir: Directory containing provider artifacts (logs-e2e-tests-*)
        output_file: Output file path
        show_logs_for: Which tests to show logs for: "failed", "all", or "none"
        debug: Enable debug output
        log_levels: Set of log levels to include, or None for all levels
    """
    import pathlib

    artifacts_path = pathlib.Path(artifacts_dir)
    if not artifacts_path.exists():
        with open(output_file, "a") as f:
            f.write("## E2E Test Results\n\n")
            f.write(f"⚠️ Artifacts directory not found: {artifacts_dir}\n")
        return

    # Find all provider directories
    provider_dirs = sorted(artifacts_path.glob("logs-e2e-tests-*"))
    if not provider_dirs:
        with open(output_file, "a") as f:
            f.write("## E2E Test Results\n\n")
            f.write("⚠️ No provider artifact directories found.\n")
        return

    # Collect results from all providers
    all_provider_results = []
    total_passed = 0
    total_failed = 0
    total_errors = 0
    total_skipped = 0
    total_tests = 0
    total_time = 0.0

    for provider_dir in provider_dirs:
        provider = provider_dir.name.replace("logs-e2e-tests-", "")
        results_dir = provider_dir / "test-results"

        if debug:
            print(f"Processing provider: {provider}", file=sys.stderr)
            print(f"  Results dir: {results_dir}", file=sys.stderr)

        # Find XML and JSON files
        xml_files = list(results_dir.glob("*.xml")) if results_dir.exists() else []
        json_files = list(results_dir.glob("*.json")) if results_dir.exists() else []

        if not xml_files:
            if debug:
                print(f"  No XML files found for {provider}", file=sys.stderr)
            all_provider_results.append(
                {
                    "provider": provider,
                    "results": None,
                    "controller_logs": None,
                    "test_outputs": {},
                }
            )
            continue

        # Parse results
        results = parse_junit_xml([str(f) for f in xml_files])

        # Parse controller logs
        logs_file = provider_dir / "pac-pods.log"
        controller_logs = None
        if logs_file.exists():
            controller_logs = parse_controller_logs(str(logs_file))
            if debug and controller_logs:
                print(
                    f"  Loaded {len(controller_logs)} controller log entries",
                    file=sys.stderr,
                )

        # Parse test JSON output
        test_outputs = {}
        if json_files:
            test_outputs = parse_test_json_output([str(f) for f in json_files])
            if debug and test_outputs:
                print(
                    f"  Loaded test output for {len(test_outputs)} tests",
                    file=sys.stderr,
                )

        all_provider_results.append(
            {
                "provider": provider,
                "results": results,
                "controller_logs": controller_logs,
                "test_outputs": test_outputs,
            }
        )

        # Accumulate totals
        total_tests += results["total_tests"]
        total_failed += results["total_failures"]
        total_errors += results["total_errors"]
        total_skipped += results["total_skipped"]
        total_time += results["total_time"]
        passed = (
            results["total_tests"]
            - results["total_failures"]
            - results["total_errors"]
            - results["total_skipped"]
        )
        total_passed += passed

    # Write combined summary
    with open(output_file, "a") as f:
        f.write("# E2E Test Results Summary\n\n")

        # Overall status
        if total_failed > 0 or total_errors > 0:
            status_emoji = "❌"
        elif total_tests == 0:
            status_emoji = "⚠️"
        else:
            status_emoji = "✅"

        f.write(f"{status_emoji} **{total_passed}/{total_tests}** tests passed")
        if total_skipped > 0:
            f.write(f" ({total_skipped} skipped)")
        f.write(f" in **{format_duration(total_time)}**\n\n")

        # Overall summary table
        f.write("## Overall Summary\n\n")
        f.write("| Metric | Count |\n")
        f.write("|--------|-------|\n")
        f.write(f"| ✅ Passed | {total_passed} |\n")
        f.write(f"| ❌ Failed | {total_failed} |\n")
        f.write(f"| 💥 Errors | {total_errors} |\n")
        f.write(f"| ⏭️ Skipped | {total_skipped} |\n")
        f.write(f"| **Total** | **{total_tests}** |\n\n")

        # Per-provider summary table
        f.write("## Results by Provider\n\n")
        f.write(
            "| Provider | Passed | Failed | Errors | Skipped | Total | Duration |\n"
        )
        f.write(
            "|----------|--------|--------|--------|---------|-------|----------|\n"
        )

        for pr in all_provider_results:
            provider = pr["provider"]
            results = pr["results"]

            if results is None:
                f.write(f"| {provider} | - | - | - | - | - | No results |\n")
                continue

            passed = (
                results["total_tests"]
                - results["total_failures"]
                - results["total_errors"]
                - results["total_skipped"]
            )
            failed = results["total_failures"]
            errors = results["total_errors"]
            skipped = results["total_skipped"]
            tests = results["total_tests"]
            duration = format_duration(results["total_time"])

            # Status indicator
            if failed > 0 or errors > 0:
                status = "❌"
            elif tests == 0:
                status = "⚠️"
            else:
                status = "✅"

            f.write(
                f"| {status} {provider} | {passed} | {failed} | {errors} | "
                f"{skipped} | {tests} | {duration} |\n"
            )

        f.write("\n")

        # Detailed failure information per provider
        has_failures = any(
            pr["results"]
            and (
                pr["results"]["total_failures"] > 0 or pr["results"]["total_errors"] > 0
            )
            for pr in all_provider_results
        )

        if has_failures:
            f.write("## Failure Details\n\n")

            for pr in all_provider_results:
                provider = pr["provider"]
                results = pr["results"]
                controller_logs = pr["controller_logs"]
                test_outputs = pr["test_outputs"]

                if results is None:
                    continue

                failed_tests = results.get("failed_tests", [])
                if not failed_tests:
                    continue

                f.write(f"### {provider}\n\n")

                for i, test in enumerate(failed_tests, 1):
                    status = test["status"]
                    if status == "failed":
                        status_icon = "❌"
                    elif status == "error":
                        status_icon = "💥"
                    else:
                        status_icon = "⏭️"

                    f.write(
                        f"<details>\n<summary><strong>{status_icon} {i}. "
                        f"{html.escape(test['name'])}</strong> "
                        f"({format_duration(test['time'])})</summary>\n\n"
                    )

                    if test["classname"]:
                        f.write(f"**Package:** `{html.escape(test['classname'])}`\n\n")

                    if test.get("start_time") and test.get("end_time"):
                        start_str = test["start_time"].strftime("%H:%M:%S")
                        end_str = test["end_time"].strftime("%H:%M:%S")
                        f.write(f"**Time window:** {start_str} - {end_str}\n\n")

                    if test["message"]:
                        f.write(f"**Message:** {html.escape(test['message'])}\n\n")

                    if test["details"]:
                        f.write("**Stack trace / Details:**\n```\n")
                        f.write(test["details"])
                        f.write("\n```\n\n")

                    # Include test output
                    test_name = test["name"]
                    test_output = test_outputs.get(test_name, "")

                    if not test_output:
                        for name, output in test_outputs.items():
                            if name.endswith(test_name) or test_name.endswith(name):
                                test_output = output
                                break

                    if test_output:
                        f.write("**Test Output:**\n```\n")
                        f.write(test_output)
                        f.write("```\n\n")
                    elif test.get("system_out") or test.get("system_err"):
                        if test.get("system_out"):
                            f.write("**Test Output (stdout):**\n```\n")
                            f.write(test["system_out"])
                            f.write("\n```\n\n")
                        if test.get("system_err"):
                            f.write("**Test Output (stderr):**\n```\n")
                            f.write(test["system_err"])
                            f.write("\n```\n\n")

                    # Include controller logs
                    if (
                        controller_logs
                        and test.get("start_time")
                        and test.get("end_time")
                    ):
                        if log_levels is not None:
                            filter_levels = log_levels
                        else:
                            filter_levels = {"error", "warn"}

                        buffer = timedelta(seconds=5)
                        start_with_buffer = test["start_time"] - buffer
                        end_with_buffer = test["end_time"] + buffer

                        test_logs = filter_logs_by_time(
                            controller_logs,
                            start_with_buffer,
                            end_with_buffer,
                            levels=filter_levels,
                        )

                        if test_logs:
                            f.write(
                                f"**Controller Logs** ({len(test_logs)} entries):\n```\n"
                            )
                            for entry in test_logs:
                                f.write(format_log_entry_for_summary(entry) + "\n")
                            f.write("```\n\n")

                    f.write("</details>\n\n")


def main():
    """Main entry point."""
    # Get configuration from environment
    summary_file = os.environ.get("GITHUB_STEP_SUMMARY", "/dev/stdout")
    provider = os.environ.get("TEST_PROVIDER", "")
    results_dir = os.environ.get("TEST_RESULTS_DIR", "/tmp/test-results")
    logs_file = os.environ.get("CONTROLLER_LOGS_FILE", "")
    # ARTIFACTS_DIR: for combined mode, directory containing all provider artifacts
    artifacts_dir = os.environ.get("ARTIFACTS_DIR", "")
    # SHOW_LOGS_FOR: "failed" (default), "all", or "none"
    show_logs_for = os.environ.get("SHOW_LOGS_FOR", "failed").lower()
    # DEBUG: set to "true" to see timing debug info
    debug = os.environ.get("DEBUG_SUMMARY", "").lower() == "true"
    # LOG_LEVELS: comma-separated list of levels to show, or "all" for all levels
    # Default: "error,warn,info"
    log_levels_env = os.environ.get("LOG_LEVELS", "error,warn,info").lower()
    if log_levels_env == "all":
        log_levels_config = None  # None means no filtering
    else:
        log_levels_config = set(
            lvl.strip() for lvl in log_levels_env.split(",") if lvl.strip()
        )

    if show_logs_for not in ("failed", "all", "none"):
        print(
            f"Warning: Invalid SHOW_LOGS_FOR value '{show_logs_for}', using 'failed'",
            file=sys.stderr,
        )
        show_logs_for = "failed"

    # Combined mode: generate summary for all providers from artifacts directory
    if artifacts_dir:
        print(
            f"Running in combined mode with artifacts from: {artifacts_dir}",
            file=sys.stderr,
        )
        generate_combined_summary(
            artifacts_dir,
            summary_file,
            show_logs_for,
            debug,
            log_levels_config,
        )
        return

    # Single provider mode (original behavior)
    if not provider:
        provider = "unknown"

    xml_files = glob.glob(f"{results_dir}/*.xml")
    json_files = glob.glob(f"{results_dir}/*.json")

    if not xml_files:
        with open(summary_file, "a") as f:
            f.write("## Test Results\n\n")
            f.write("⚠️ No test result files found.\n")
        return

    # Parse controller logs if available
    controller_logs = None
    if logs_file:
        controller_logs = parse_controller_logs(logs_file)
        if controller_logs:
            print(
                f"Loaded {len(controller_logs)} controller log entries",
                file=sys.stderr,
            )

    # Parse test JSON output for test logs
    test_outputs = {}
    if json_files:
        test_outputs = parse_test_json_output(json_files)
        if test_outputs:
            print(
                f"Loaded test output for {len(test_outputs)} tests",
                file=sys.stderr,
            )

    results = parse_junit_xml(xml_files)

    if debug:
        print(f"Suite start time: {results.get('suite_start_time')}", file=sys.stderr)
        if results["all_tests"]:
            first_test = results["all_tests"][0]
            print(
                f"First test: {first_test['name']}, "
                f"start: {first_test.get('start_time')}, "
                f"end: {first_test.get('end_time')}",
                file=sys.stderr,
            )
        if controller_logs:
            first_log = controller_logs[0]
            print(
                f"First log timestamp: {first_log.get('_parsed_ts')}",
                file=sys.stderr,
            )
            last_log = controller_logs[-1]
            print(
                f"Last log timestamp: {last_log.get('_parsed_ts')}",
                file=sys.stderr,
            )

    generate_summary(
        results,
        provider,
        summary_file,
        controller_logs,
        show_logs_for,
        debug,
        log_levels_config,
        test_outputs,
    )


if __name__ == "__main__":
    main()
