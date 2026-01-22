#!/usr/bin/env python3
"""Filter controller logs by event-sha, event-id, level, and/or time range.

Usage:
    # Filter all error logs (default level)
    ./hack/filter-logs-by-event.py -f /tmp/logs/pac-pods.log

    # Filter all logs (all levels)
    ./hack/filter-logs-by-event.py --all-levels -f /tmp/logs/pac-pods.log

    # Filter by event-sha
    ./hack/filter-logs-by-event.py --event-sha abc123,def456 -f logfile.log

    # Filter by event-id
    ./hack/filter-logs-by-event.py --event-id b3bb62b7-9f0b-45d2-b45f-3f78211cf258 -f logfile.log

    # Filter by specific log level(s)
    ./hack/filter-logs-by-event.py --level error,warn -f logfile.log

    # Filter by time range (supports milliseconds)
    ./hack/filter-logs-by-event.py --start-time "06:11:50.278" --end-time "06:11:52.500" -f logfile.log
    ./hack/filter-logs-by-event.py --start-time "2026-01-22 06:11:50.278" -f logfile.log

    # Combine filters
    ./hack/filter-logs-by-event.py --event-sha abc123 --level error,warn -S "06:00" -E "07:00" -f logfile.log

    # Output as JSON (default is pretty-printed)
    ./hack/filter-logs-by-event.py --json -f logfile.log
"""

import argparse
import json
import sys
from datetime import datetime


def parse_timestamp(ts):
    """Parse timestamp from various formats for display."""
    if isinstance(ts, (int, float)):
        return datetime.fromtimestamp(ts).strftime("%Y-%m-%d %H:%M:%S")
    elif isinstance(ts, str):
        # ISO format
        try:
            return ts.replace("T", " ").replace("Z", "")[:19]
        except (ValueError, IndexError):
            return ts
    return str(ts)


def parse_timestamp_to_datetime(ts):
    """Parse timestamp from log entry to datetime object for comparison."""
    if isinstance(ts, (int, float)):
        return datetime.fromtimestamp(ts)
    elif isinstance(ts, str):
        # Try ISO format: 2026-01-22T06:11:50.278Z
        try:
            # Remove Z and handle milliseconds
            ts_clean = ts.rstrip("Z")
            if "." in ts_clean:
                return datetime.fromisoformat(ts_clean)
            return datetime.fromisoformat(ts_clean)
        except ValueError:
            pass
    return None


def parse_time_arg(time_str):
    """Parse time argument in various formats, including milliseconds."""
    if not time_str:
        return None

    # Try various formats (with and without milliseconds)
    # %f handles microseconds (6 digits), but works with 3 digits (milliseconds) too
    formats = [
        # With milliseconds
        "%Y-%m-%d %H:%M:%S.%f",
        "%Y-%m-%dT%H:%M:%S.%f",
        "%H:%M:%S.%f",
        # Without milliseconds
        "%Y-%m-%d %H:%M:%S",
        "%Y-%m-%dT%H:%M:%S",
        "%Y-%m-%d %H:%M",
        "%H:%M:%S",
        "%H:%M",
    ]

    time_only_formats = ("%H:%M:%S.%f", "%H:%M:%S", "%H:%M")

    for fmt in formats:
        try:
            parsed = datetime.strptime(time_str, fmt)
            # If only time was provided, use today's date
            if fmt in time_only_formats:
                today = datetime.now().date()
                parsed = datetime.combine(today, parsed.time())
            return parsed
        except ValueError:
            continue

    return None


def format_time_for_display(dt):
    """Format datetime for display, including milliseconds if present."""
    if dt is None:
        return "*"
    # Include milliseconds in display
    if dt.microsecond:
        return dt.strftime("%Y-%m-%d %H:%M:%S.") + f"{dt.microsecond // 1000:03d}"
    return dt.strftime("%Y-%m-%d %H:%M:%S")


def format_log_entry(entry, use_json=False):
    """Format a log entry for display."""
    if use_json:
        return json.dumps(entry)

    ts = parse_timestamp(entry.get("ts", ""))
    level = entry.get("level", "info").upper()
    msg = entry.get("msg", "")
    caller = entry.get("caller", "")

    # Color codes for different levels
    colors = {
        "INFO": "\033[32m",  # Green
        "WARN": "\033[33m",  # Yellow
        "ERROR": "\033[31m",  # Red
        "DEBUG": "\033[36m",  # Cyan
    }
    reset = "\033[0m"
    color = colors.get(level, "")

    # Build context string from relevant fields
    context_fields = [
        "event-id",
        "event-sha",
        "event-type",
        "namespace",
        "provider",
        "source-repo-url",
    ]
    context_parts = []
    for field in context_fields:
        if field in entry and entry[field]:
            context_parts.append(f"{field}={entry[field]}")

    context = " ".join(context_parts)

    output = f"{ts} {color}[{level}]{reset} {caller}: {msg}"
    if context:
        output += f"\n    └─ {context}"

    return output


def filter_logs(
    input_stream, event_shas, event_ids, levels, start_time, end_time, use_json=False
):
    """Filter logs by event-sha and/or event-id, optionally filtering by log level and time."""
    event_shas = set(event_shas) if event_shas else set()
    event_ids = set(event_ids) if event_ids else set()
    levels = set(lvl.lower() for lvl in levels) if levels else None

    for line in input_stream:
        line = line.strip()
        if not line:
            continue

        # Try to parse as JSON
        if not line.startswith("{"):
            continue

        try:
            entry = json.loads(line)
        except json.JSONDecodeError:
            continue

        # Check log level filter first
        entry_level = entry.get("level", "info").lower()
        if levels and entry_level not in levels:
            continue

        # Check timestamp filter
        if start_time or end_time:
            entry_ts = parse_timestamp_to_datetime(entry.get("ts"))
            if entry_ts:
                if start_time and entry_ts < start_time:
                    continue
                if end_time and entry_ts > end_time:
                    continue

        # Check if this entry matches any of our filters
        entry_sha = entry.get("event-sha", "")
        entry_id = entry.get("event-id", "")

        matched = False

        # Match by event-sha (if provided)
        if event_shas:
            if entry_sha and entry_sha in event_shas:
                matched = True
            # Also check if any provided sha is a prefix of the entry sha
            for sha in event_shas:
                if (
                    sha
                    and entry_sha
                    and (entry_sha.startswith(sha) or sha.startswith(entry_sha))
                ):
                    matched = True
                    break

        # Match by event-id (if provided)
        if event_ids:
            if entry_id and entry_id in event_ids:
                matched = True

        # If no event filters provided, match all (filtered by level/time above)
        if not event_shas and not event_ids:
            matched = True

        if matched:
            print(format_log_entry(entry, use_json))


def main():
    parser = argparse.ArgumentParser(
        description="Filter controller logs by event-sha or event-id",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    parser.add_argument(
        "--event-sha",
        "-s",
        help="Comma-separated list of event-sha values to filter by",
    )
    parser.add_argument(
        "--event-id",
        "-i",
        help="Comma-separated list of event-id values to filter by",
    )
    parser.add_argument(
        "--file",
        "-f",
        help="Log file to read (default: stdin)",
    )
    parser.add_argument(
        "--json",
        "-j",
        action="store_true",
        help="Output as JSON instead of pretty-printed",
    )
    parser.add_argument(
        "--level",
        "-l",
        default="error",
        help="Comma-separated list of log levels to include (default: error). "
        "Valid levels: debug, info, warn, error",
    )
    parser.add_argument(
        "--all-levels",
        "-a",
        action="store_true",
        help="Show all log levels (overrides --level)",
    )
    parser.add_argument(
        "--start-time",
        "-S",
        help="Start time filter (formats: 'HH:MM:SS.mmm', 'YYYY-MM-DD HH:MM:SS.mmm', etc.)",
    )
    parser.add_argument(
        "--end-time",
        "-E",
        help="End time filter (formats: 'HH:MM:SS.mmm', 'YYYY-MM-DD HH:MM:SS.mmm', etc.)",
    )

    args = parser.parse_args()

    # Parse comma-separated values
    event_shas = []
    if args.event_sha:
        event_shas = [s.strip() for s in args.event_sha.split(",") if s.strip()]

    event_ids = []
    if args.event_id:
        event_ids = [s.strip() for s in args.event_id.split(",") if s.strip()]

    # Parse log levels
    if args.all_levels:
        levels = None  # None means no level filtering
    else:
        levels = [lvl.strip().lower() for lvl in args.level.split(",") if lvl.strip()]

    # Parse time filters
    start_time = parse_time_arg(args.start_time)
    end_time = parse_time_arg(args.end_time)

    if args.start_time and not start_time:
        print(f"Error: Invalid start time format: {args.start_time}", file=sys.stderr)
        sys.exit(1)
    if args.end_time and not end_time:
        print(f"Error: Invalid end time format: {args.end_time}", file=sys.stderr)
        sys.exit(1)

    # Format filter strings for summary
    levels_str = ", ".join(levels) if levels else "all"
    time_str = ""
    if start_time or end_time:
        start_fmt = format_time_for_display(start_time)
        end_fmt = format_time_for_display(end_time)
        time_str = f", time: {start_fmt} to {end_fmt}"

    # Read from file or stdin
    if args.file:
        try:
            with open(args.file) as f:
                filter_logs(
                    f, event_shas, event_ids, levels, start_time, end_time, args.json
                )
        except FileNotFoundError:
            print(f"Error: File not found: {args.file}", file=sys.stderr)
            sys.exit(1)
    else:
        filter_logs(
            sys.stdin, event_shas, event_ids, levels, start_time, end_time, args.json
        )

    # Always display summary
    print("\n--- Summary ---", file=sys.stderr)
    if event_shas:
        print(
            f"Event SHAs: {', '.join(event_shas)} (levels: {levels_str}{time_str})",
            file=sys.stderr,
        )
    if event_ids:
        print(
            f"Event IDs: {', '.join(event_ids)} (levels: {levels_str}{time_str})",
            file=sys.stderr,
        )
    if not event_shas and not event_ids:
        print(
            f"All events (levels: {levels_str}{time_str})",
            file=sys.stderr,
        )


if __name__ == "__main__":
    main()
