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

    export      Export all raw analytics data
    server      Show server analytics endpoints (when running in HTTP mode)
    help        Show this help message

Examples:
    $0 summary
    $0 tools
    $0 searches
    $0 export > analytics-backup.json
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
        echo "📊 Overall Statistics:"
        jq -c 'select(.type != null)' "$ANALYTICS_FILE" | jq -s '
            "• Total tool invocations: \(map(select(.type == "tool_invocation")) | length)",
            "• Total search queries: \(map(select(.type == "search_query")) | length)",
            "• Active sessions: \(map(select(.type == "session_activity")) | length)"
        '

        echo
        echo "🔧 Most Used Tools:"
        jq -c 'select(.type == "tool_invocation")' "$ANALYTICS_FILE" | jq -s '
            group_by(.tool_name) |
            map({tool: .[0].tool_name, count: length}) |
            sort_by(.count) | reverse |
            .[:5] |
            .[] | "• \(.tool): \(.count) uses"
        '

        echo
        echo "🔍 Popular Search Terms:"
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
            .[] | "• \(.query) (\(.count) times, avg \(.avg_results | floor) results)"
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
        echo "📈 Tool Performance:"
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
            "• \(.tool):",
            "  - Uses: \(.count)",
            "  - Success Rate: \(.success_rate | floor)%",

            "  - Avg Result Size: \(.avg_result_size | floor) chars",
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
        echo "🔍 Search Patterns:"
        jq -c 'select(.type == "search_query")' "$ANALYTICS_FILE" | jq -s '
            "Total searches: \(length)",
            "Unique queries: \(group_by(.query) | length)",
            "Average results per search: \((map(.result_count // 0) | add) / length | floor)",
            "Success rate: \((map(select(.success == true)) | length) * 100 / length | floor)%",
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
            .[] | "• \(.query) (\(.count) times, avg \(.avg_results | floor) results)"
        '
    else
        echo "Raw search query data:"
        cat "$ANALYTICS_FILE"
    fi
}



# Function to show server endpoints
show_server() {
    cat << EOF
=== Server Analytics Endpoints ===

When running xmlui-mcp in HTTP mode (--http), these endpoints provide real-time analytics:

📊 Analytics Summary:
   http://localhost:8080/analytics/summary

📁 Export All Data:
   http://localhost:8080/analytics/export

🔧 Tools List:
   http://localhost:8080/tools

📝 Prompts List:
   http://localhost:8080/prompts

👥 Session Data:
   http://localhost:8080/session/{session_id}

Examples:
   curl http://localhost:8080/analytics/summary | jq
   curl http://localhost:8080/analytics/export > backup.json
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

    export)
        cat "$ANALYTICS_FILE"
        ;;
    server)
        show_server
        ;;
    help|*)
        show_help
        ;;
esac
