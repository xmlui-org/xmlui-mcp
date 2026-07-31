#!/bin/bash

# XMLUI-MCP Analytics Helper Script
# This script helps you view and analyze agent usage data from JSONL format

# Determine cache directory based on OS
get_cache_dir() {
    local os
    os=$(uname -s)
    local home_dir="$HOME"

    case "$os" in
        Darwin)
            # macOS: ~/Library/Caches/xmlui/xmlui-mcp
            echo "$home_dir/Library/Caches/xmlui/xmlui-mcp"
            ;;
        Linux)
            # Linux: $XDG_CACHE_HOME/xmlui/xmlui-mcp or ~/.cache/xmlui/xmlui-mcp
            local xdg_cache="${XDG_CACHE_HOME:-}"
            if [ -n "$xdg_cache" ]; then
                echo "$xdg_cache/xmlui/xmlui-mcp"
            else
                echo "$home_dir/.cache/xmlui/xmlui-mcp"
            fi
            ;;
        *)
            echo "Error: Unsupported OS: $os" >&2
            echo "This script currently supports macOS and Linux only." >&2
            exit 1
            ;;
    esac
}

# Get the analytics file path. The override supports fixture-based verification
# without writing test observations into the user's live analytics log.
if [ -n "${XMLUI_MCP_ANALYTICS_FILE:-}" ]; then
    ANALYTICS_FILE="$XMLUI_MCP_ANALYTICS_FILE"
else
    CACHE_DIR=$(get_cache_dir)
    ANALYTICS_FILE="$CACHE_DIR/xmlui-mcp-analytics.json"
fi

# Check if analytics file exists
if [ ! -f "$ANALYTICS_FILE" ]; then
    echo "No analytics file found at $ANALYTICS_FILE"
    echo "Analytics data will be created after agents start using the MCP server."
    exit 1
fi

echo "Using analytics file: $ANALYTICS_FILE"

# Function to display usage
show_help() {
    cat << EOF
XMLUI-MCP Analytics Helper

Usage: $0 [COMMAND]

Commands:
    summary     Show a summary of agent usage patterns
    tools       Show tool usage statistics
    searches    Show search query analysis
    xmlui_search Show xmlui_search queries and results analysis
    xmlui_search_fail Show completed v2 xmlui_search queries with no results
    xmlui_search_success Show completed v2 xmlui_search queries with results

    server      Show server analytics endpoints (when running in HTTP mode)
    help        Show this help message

Examples:
    $0 summary
    $0 tools
    $0 searches
    $0 xmlui_search
    $0 xmlui_search_fail
    $0 xmlui_search_success
EOF
}

# Function to check if jq is available
check_jq() {
    if ! command -v jq &> /dev/null; then
        echo "Warning: 'jq' is not installed. Output will be raw JSON."
        echo "Install jq for better formatted output: brew install jq"
        return 1
    fi
    return 0
}

# Function to show summary
show_summary() {
    echo "=== XMLUI-MCP Analytics Summary ==="
    echo

    if check_jq; then
        echo "Overall Statistics:"
        jq -c 'select(.type != null)' "$ANALYTICS_FILE" | jq -s '
            (map(select(.type == "search_query"))) as $searches |
            ($searches | map(select(.schema_version == 2))) as $v2 |
            ($searches | map(select(.schema_version != 2))) as $legacy |
            ($v2 | map(select(.execution_success == true))) as $executed |
            ($executed | map(select(.yielded_results == true))) as $hits |
            ($executed | map(select(.yielded_results == false))) as $misses |
            "• Total tool invocations: " + (map(select(.type == "tool_invocation")) | length | tostring),
            "• Total search queries: " + ($searches | length | tostring),
            "• Schema-v2 search queries: " + ($v2 | length | tostring),
            "• Legacy search queries (excluded from v2 rates): " + ($legacy | length | tostring),
            "• V2 execution success rate: " +
                (if ($v2 | length) > 0 then (($executed | length) * 100 / ($v2 | length) | floor | tostring) + "%" else "n/a" end),
            "• V2 retrieval hit rate (completed searches): " +
                (if ($executed | length) > 0 then (($hits | length) * 100 / ($executed | length) | floor | tostring) + "%" else "n/a" end),
            "• V2 zero-result queries: " + ($misses | length | tostring),
            "• V2 operational failures: " + (($v2 | map(select(.execution_success != true)) | length) | tostring)
        '

        echo
        echo "Most Used Tools:"
        jq -c 'select(.type == "tool_invocation")' "$ANALYTICS_FILE" | jq -s '
            group_by(.tool_name) |
            map({tool: .[0].tool_name, count: length}) |
            sort_by(.count) | reverse |
            .[:5] |
            .[] | "• " + .tool + ": " + (.count | tostring) + " uses"
        '

        echo
        echo "Popular Schema-v2 Search Terms:"
        jq -c 'select(.type == "search_query" and .schema_version == 2)' "$ANALYTICS_FILE" | jq -s '
            group_by(.query) |
            map(
                . as $group |
                {
                    query: $group[0].query,
                    count: ($group | length),
                    avg_matched_files: (
                        ($group | map(.matched_file_count // 0) | add)
                        / ($group | length)
                    )
                }
            ) |
            sort_by(.count) | reverse |
            .[:10] |
            .[] | "• " + .query + " (" + (.count | tostring) + " times, avg " + (.avg_matched_files | floor | tostring) + " matched files)"
        '
    else
        cat "$ANALYTICS_FILE"
    fi
}

# Function to show tool statistics
show_tools() {
    echo "=== Tool Usage Analysis ==="
    echo

    if check_jq; then
        echo "Tool Performance:"
        jq -c 'select(.type == "tool_invocation")' "$ANALYTICS_FILE" | jq -s '
            group_by(.tool_name) |
            map(
                . as $group |
                {
                    tool: $group[0].tool_name,
                    count: length,
                    success_rate: (
                        if length > 0 then
                            (map(select(.success == true)) | length) * 100 / length
                        else 0 end
                    ),
                    avg_result_size: (
                        map(select(.result_size_chars != null) | .result_size_chars) as $sizes |
                        if $sizes | length > 0 then ($sizes | add) / ($sizes | length) else 0 end
                    )
                }
            ) |
            sort_by(.count) | reverse |
            .[] |
            "• " + .tool + ":",
            "  - Uses: " + (.count | tostring),
            "  - Success Rate: " + (.success_rate | floor | tostring) + "%",
            "  - Avg Result Size: " + (.avg_result_size | floor | tostring) + " chars",
            ""
        '
    else
        echo "Raw tool invocation data:"
        cat "$ANALYTICS_FILE"
    fi
}

# Function to show search analysis
show_searches() {
    echo "=== Search Query Analysis ==="
    echo

    if check_jq; then
        echo "Schema-v2 Search Outcomes:"
        jq -c 'select(.type == "search_query")' "$ANALYTICS_FILE" | jq -s '
            (map(select(.schema_version == 2))) as $v2 |
            (map(select(.schema_version != 2))) as $legacy |
            ($v2 | map(select(.execution_success == true))) as $executed |
            ($executed | map(select(.yielded_results == true))) as $hits |
            "V2 searches: " + ($v2 | length | tostring),
            "Legacy searches (excluded): " + ($legacy | length | tostring),
            "Unique v2 queries: " + ($v2 | group_by(.query) | length | tostring),
            "Average matched files per completed v2 search: " +
                (if ($executed | length) > 0 then (($executed | map(.matched_file_count // 0) | add) / ($executed | length) | floor | tostring) else "n/a" end),
            "Average matched lines per completed v2 search: " +
                (if ($executed | length) > 0 then (($executed | map(.matched_line_count // 0) | add) / ($executed | length) | floor | tostring) else "n/a" end),
            "Execution success rate: " +
                (if ($v2 | length) > 0 then (($executed | length) * 100 / ($v2 | length) | floor | tostring) + "%" else "n/a" end),
            "Retrieval hit rate (completed searches): " +
                (if ($executed | length) > 0 then (($hits | length) * 100 / ($executed | length) | floor | tostring) + "%" else "n/a" end),
            "Zero-result completed searches: " + (($executed | map(select(.yielded_results == false)) | length) | tostring),
            "",
            "Most frequent v2 searches:"
        '

        jq -c 'select(.type == "search_query" and .schema_version == 2)' "$ANALYTICS_FILE" | jq -s '
            group_by(.query) |
            map(
                . as $group |
                {
                    query: $group[0].query,
                    count: ($group | length),
                    avg_matched_files: (
                        ($group | map(.matched_file_count // 0) | add)
                        / ($group | length)
                    )
                }
            ) |
            sort_by(.count) | reverse |
            .[:10] |
            .[] | "• " + .query + " (" + (.count | tostring) + " times, avg " + (.avg_matched_files | floor | tostring) + " matched files)"
        '

        echo
        echo "Most Matched Paths in Schema-v2 Records:"
        jq -c 'select(.type == "search_query" and .schema_version == 2)' "$ANALYTICS_FILE" | jq -s '
            map(.matched_paths // []) | flatten | group_by(.) |
            map({path: .[0], count: length}) |
            sort_by(.count) | reverse |
            .[:15] |
            .[] | "• " + .path + " (" + (.count | tostring) + " times)"
        '
    else
        echo "Raw search query data:"
        cat "$ANALYTICS_FILE"
    fi
}

# Function to show xmlui_search analysis
show_xmlui() {
    echo "=== XMLUI Search Analysis ==="
    echo

    if check_jq; then
        echo "Schema-v2 XMLUI Search Overview:"
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search")' "$ANALYTICS_FILE" | jq -s '
            (map(select(.schema_version == 2))) as $v2 |
            (map(select(.schema_version != 2))) as $legacy |
            ($v2 | map(select(.execution_success == true))) as $executed |
            ($executed | map(select(.yielded_results == true))) as $hits |
            "V2 xmlui_search queries: " + ($v2 | length | tostring),
            "Legacy xmlui_search queries (excluded): " + ($legacy | length | tostring),
            "Unique v2 queries: " + ($v2 | group_by(.query) | length | tostring),
            "Execution successes: " + ($executed | length | tostring),
            "Operational failures: " + (($v2 | map(select(.execution_success != true)) | length) | tostring),
            "Retrieval hits: " + ($hits | length | tostring),
            "Zero-result searches: " + (($executed | map(select(.yielded_results == false)) | length) | tostring),
            "Retrieval hit rate (completed searches): " +
                (if ($executed | length) > 0 then (($hits | length) * 100 / ($executed | length) | floor | tostring) + "%" else "n/a" end),
            ""
        '

        echo "Schema-v2 Search Query Details:"
        echo "==============================="
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .schema_version == 2)' "$ANALYTICS_FILE" | jq -s '
            sort_by(.timestamp) |
            .[] |
            "Query: " + .query,
            "  Time: " + .timestamp,
            "  Invocation ID: " + (.invocation_id // ""),
            "  Execution success: " + (.execution_success | tostring),
            "  Yielded results: " + (.yielded_results | tostring),
            "  Matched files: " + ((.matched_file_count // 0) | tostring),
            "  Matched lines: " + ((.matched_line_count // 0) | tostring),
            "  Confidence: " + (.confidence // ""),
            "  Corpus version: " + (.corpus_version // ""),
            (if .matched_paths and (.matched_paths | length) > 0 then
                "  Matched paths (ranked):",
                (.matched_paths | .[] | "    • " + .)
            else
                "  No matched paths"
            end),
            ""
        '

        echo
        echo "Most Frequent Schema-v2 XMLUI Search Queries:"
        echo "=============================================="
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .schema_version == 2)' "$ANALYTICS_FILE" | jq -s '
            group_by(.query) |
            map(
                . as $group |
                {
                    query: $group[0].query,
                    count: ($group | length),
                    execution_success_count: ($group | map(select(.execution_success == true)) | length),
                    hit_count: ($group | map(select(.execution_success == true and .yielded_results == true)) | length),
                    avg_matched_files: (
                        ($group | map(.matched_file_count // 0) | add)
                        / ($group | length)
                    )
                }
            ) |
            sort_by(.count) | reverse |
            .[] |
            "• " + .query,
            "  Searched " + (.count | tostring) + " times",
            "  Execution successes: " + (.execution_success_count | tostring) + "/" + (.count | tostring),
            "  Retrieval hits: " + (.hit_count | tostring),
            "  Avg matched files: " + (.avg_matched_files | floor | tostring),
            ""
        '

        echo
        echo "Most Matched Paths in Schema-v2 XMLUI Searches:"
        echo "================================================"
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .schema_version == 2)' "$ANALYTICS_FILE" | jq -s '
            map(.matched_paths // []) | flatten | group_by(.) |
            map({path: .[0], count: length}) |
            sort_by(.count) | reverse |
            .[:20] |
            .[] | "• " + .path + " (" + (.count | tostring) + " times)"
        '

        echo
        echo "Legacy XMLUI Search Context:"
        echo "============================"
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .schema_version != 2)' "$ANALYTICS_FILE" | jq -s '
            "Legacy records: " + (length | tostring),
            "Legacy success values are text-derived and are not comparable to v2 retrieval outcomes."
        '

    else
        echo "Raw xmlui_search data:"
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search")' "$ANALYTICS_FILE"
    fi
}

# Function to show failed xmlui_search queries
show_xmlui_search_fail() {
    echo "=== Schema-v2 Zero-result XMLUI Search Queries ==="
    echo

    if check_jq; then
        echo "Completed searches that returned no results (case-insensitive alphabetical order):"
        echo "=================================================================================="

        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .schema_version == 2 and .execution_success == true and .yielded_results == false)' "$ANALYTICS_FILE" | jq -s '
            map(.query) | unique | sort_by(ascii_downcase) |
            .[] | "• " + .
        '

        echo
        echo "Summary:"
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .schema_version == 2)' "$ANALYTICS_FILE" | jq -s '
            (map(select(.execution_success == true and .yielded_results == false))) as $misses |
            "Total zero-result searches: " + ($misses | length | tostring),
            "Unique zero-result terms: " + ($misses | map(.query) | unique | length | tostring),
            "Operational failures excluded: " + (map(select(.execution_success != true)) | length | tostring)
        '

        echo
        echo "Legacy schema-v1 records with success=false (historical, text-derived):"
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .schema_version != 2 and .success == false)' "$ANALYTICS_FILE" | jq -s '
            map(.query) | unique | sort_by(ascii_downcase) |
            .[] | "• " + .
        '

    else
        echo "Raw schema-v2 zero-result xmlui_search data:"
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .schema_version == 2 and .execution_success == true and .yielded_results == false)' "$ANALYTICS_FILE"
    fi
}

# Function to show successful xmlui_search terms
show_xmlui_search_success() {
    echo "=== Schema-v2 XMLUI Search Hits ==="
    echo

    if check_jq; then
        echo "Completed search queries that yielded results (alphabetical):"
        echo "============================================================"
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .schema_version == 2 and .execution_success == true and .yielded_results == true)' "$ANALYTICS_FILE" | jq -s '
            map(.query) |
            unique |
            sort |
            .[] |
            "• " + .
        '

        echo
        echo "Retrieval-hit queries by frequency:"
        echo "==================================="
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .schema_version == 2 and .execution_success == true and .yielded_results == true)' "$ANALYTICS_FILE" | jq -s '
            group_by(.query) |
            map({query: .[0].query, count: length}) |
            sort_by(-.count) |
            .[] |
            .query + " (" + (.count | tostring) + "x)"
        '

        echo
        echo "Summary:"
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .schema_version == 2 and .execution_success == true and .yielded_results == true)' "$ANALYTICS_FILE" | jq -s '
            "Total retrieval hits: " + (length | tostring),
            "Unique retrieval-hit terms: " + (map(.query) | unique | length | tostring)
        '

        echo
        echo "Legacy schema-v1 records with success=true (historical, text-derived):"
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .schema_version != 2 and .success == true)' "$ANALYTICS_FILE" | jq -s '
            map(.query) | unique | sort_by(ascii_downcase) |
            .[] | "• " + .
        '

    else
        echo "Raw schema-v2 retrieval-hit xmlui_search data:"
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .schema_version == 2 and .execution_success == true and .yielded_results == true)' "$ANALYTICS_FILE"
    fi
}

# Function to show server endpoints
show_server() {
    cat << EOF
=== Server Analytics Endpoints ===

When running xmlui-mcp in HTTP mode (--http), these endpoints provide real-time analytics:

Analytics Summary:
   http://localhost:8080/analytics/summary

Tools List:
   http://localhost:8080/tools

Prompts List:
   http://localhost:8080/prompts

Session Data:
   http://localhost:8080/session/{session_id}

Examples:
   curl http://localhost:8080/analytics/summary | jq
   curl http://localhost:8080/tools | jq '.[] | .name'
EOF
}

# Main script logic
case "${1:-help}" in
    summary)
        show_summary
        ;;
    tools)
        show_tools
        ;;
    searches)
        show_searches
        ;;
    xmlui_search)
        show_xmlui
        ;;
    xmlui_search_fail)
        show_xmlui_search_fail
        ;;
    xmlui_search_success)
        show_xmlui_search_success
        ;;
    server)
        show_server
        ;;
    help|*)
        show_help
        ;;
esac
