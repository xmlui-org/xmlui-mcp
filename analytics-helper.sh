#!/bin/bash

# XMLUI-MCP Analytics Helper Script
# This script helps you view and analyze agent usage data from JSONL format

ANALYTICS_FILE="xmlui-mcp-analytics.json"

# Check if analytics file exists
if [ ! -f "$ANALYTICS_FILE" ]; then
    echo "No analytics file found at $ANALYTICS_FILE in current directory"
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
    xmlui_search_fail Show failed xmlui_search queries (no results found)

    server      Show server analytics endpoints (when running in HTTP mode)
    help        Show this help message

Examples:
    $0 summary
    $0 tools
    $0 searches
    $0 xmlui_search
    $0 xmlui_search_fail
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
            "• Total tool invocations: " + (map(select(.type == "tool_invocation")) | length | tostring),
            "• Total search queries: " + (map(select(.type == "search_query")) | length | tostring)
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
        echo "Popular Search Terms:"
        jq -c 'select(.type == "search_query")' "$ANALYTICS_FILE" | jq -s '
            group_by(.query) |
            map(
                . as $group |
                {
                    query: $group[0].query,
                    count: ($group | length),
                    avg_results: (
                        ($group | map(.result_count // 0) | add)
                        / ($group | length)
                    )
                }
            ) |
            sort_by(.count) | reverse |
            .[:10] |
            .[] | "• " + .query + " (" + (.count | tostring) + " times, avg " + (.avg_results | floor | tostring) + " results)"
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
        echo "Search Patterns:"
        jq -c 'select(.type == "search_query")' "$ANALYTICS_FILE" | jq -s '
            "Total searches: " + (length | tostring),
            "Unique queries: " + (group_by(.query) | length | tostring),
            "Average results per search: " + ((map(.result_count // 0) | add) / length | floor | tostring),
            "Success rate: " + ((map(select(.success == true)) | length) * 100 / length | floor | tostring) + "%",
            "",
            "Most frequent searches:"
        '

        jq -c 'select(.type == "search_query")' "$ANALYTICS_FILE" | jq -s '
            group_by(.query) |
            map(
                . as $group |
                {
                    query: $group[0].query,
                    count: ($group | length),
                    avg_results: (
                        ($group | map(.result_count // 0) | add)
                        / ($group | length)
                    )
                }
            ) |
            sort_by(.count) | reverse |
            .[:10] |
            .[] | "• " + .query + " (" + (.count | tostring) + " times, avg " + (.avg_results | floor | tostring) + " results)"
        '

        echo
        echo "Most Found URLs (by frequency):"
        jq -c 'select(.type == "search_query" and .found_urls != null)' "$ANALYTICS_FILE" | jq -s '
            map(.found_urls // []) | flatten | group_by(.) |
            map({url: .[0], count: length}) |
            sort_by(.count) | reverse |
            .[:15] |
            .[] | "• " + .url + " (" + (.count | tostring) + " times)"
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
        echo "XMLUI Search Overview:"
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search")' "$ANALYTICS_FILE" | jq -s '
            "Total xmlui_search queries: " + (length | tostring),
            "Unique queries: " + (group_by(.query) | length | tostring),
            "Successful searches: " + (map(select(.success == true)) | length | tostring),
            "Failed searches: " + (map(select(.success == false)) | length | tostring),
            "Average results per search: " + ((map(.result_count // 0) | add) / length | floor | tostring),
            ""
        '

        echo "Search Query Details:"
        echo "===================="
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search")' "$ANALYTICS_FILE" | jq -s '
            sort_by(.timestamp) |
            .[] |
            "Query: " + .query,
            "  Time: " + .timestamp,
            "  Success: " + (.success | tostring),
            "  Results: " + (.result_count | tostring),
            "  Search Paths: " + (.search_paths | join(", ")),
            (if .found_urls and (.found_urls | length) > 0 then
                "  Found Files (" + (.found_urls | length | tostring) + "):",
                (.found_urls | .[] | "    • " + .)
            else
                "  No files found"
            end),
            ""
        '

        echo
        echo "Most Frequent XMLUI Search Queries:"
        echo "==================================="
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search")' "$ANALYTICS_FILE" | jq -s '
            group_by(.query) |
            map(
                . as $group |
                {
                    query: $group[0].query,
                    count: ($group | length),
                    success_count: (map(select(.success == true)) | length),
                    avg_results: (
                        ($group | map(.result_count // 0) | add)
                        / ($group | length)
                    ),
                    total_results: ($group | map(.result_count // 0) | add)
                }
            ) |
            sort_by(.count) | reverse |
            .[] |
            "• " + .query,
            "  Searched " + (.count | tostring) + " times",
            "  Successful: " + (.success_count | tostring) + "/" + (.count | tostring),
            "  Avg results: " + (.avg_results | floor | tostring),
            "  Total results found: " + (.total_results | tostring),
            ""
        '

        echo
        echo "Most Found Files in XMLUI Searches:"
        echo "==================================="
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .found_urls != null)' "$ANALYTICS_FILE" | jq -s '
            map(.found_urls // []) | flatten | group_by(.) |
            map({url: .[0], count: length}) |
            sort_by(.count) | reverse |
            .[:20] |
            .[] | "• " + .url + " (" + (.count | tostring) + " times)"
        '

        echo
        echo "Search Path Analysis:"
        echo "===================="
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search")' "$ANALYTICS_FILE" | jq -s '
            map(.search_paths // []) | flatten | group_by(.) |
            map({path: .[0], count: length}) |
            sort_by(.count) | reverse |
            .[] | "• " + .path + " (" + (.count | tostring) + " searches)"
        '

    else
        echo "Raw xmlui_search data:"
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search")' "$ANALYTICS_FILE"
    fi
}

# Function to show failed xmlui_search queries
show_xmlui_search_fail() {
    echo "=== Failed XMLUI Search Queries ==="
    echo

    if check_jq; then
        echo "Search terms that returned no results (case-insensitive alphabetical order):"
        echo "============================================================================="

        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .success == false)' "$ANALYTICS_FILE" | jq -s '
            map(.query) | unique | sort_by(ascii_downcase) |
            .[] | "• " + .
        '

        echo
        echo "Summary:"
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .success == false)' "$ANALYTICS_FILE" | jq -s '
            "Total failed searches: " + (length | tostring),
            "Unique failed search terms: " + (map(.query) | unique | length | tostring)
        '

    else
        echo "Raw failed xmlui_search data:"
        jq -c 'select(.type == "search_query" and .tool_name == "xmlui_search" and .success == false)' "$ANALYTICS_FILE"
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
    server)
        show_server
        ;;
    help|*)
        show_help
        ;;
esac
