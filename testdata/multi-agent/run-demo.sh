#!/bin/bash
# Multi-Agent Coordination Demo Runner
# This script runs the full multi-agent demo

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Default values
LIVE=false
VERBOSE=false
SEQUENTIAL=false
MAX_TURNS=10
TIMEOUT=5
MODEL=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -l|--live)
            LIVE=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -s|--sequential)
            SEQUENTIAL=true
            shift
            ;;
        --max-turns)
            MAX_TURNS="$2"
            shift 2
            ;;
        --timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        --model)
            MODEL="$2"
            shift 2
            ;;
        -h|--help)
            echo "Multi-Agent Coordination Demo Runner"
            echo ""
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  -l, --live         Use real Saturn LLM (requires beacon)"
            echo "  -v, --verbose      Enable verbose output"
            echo "  -s, --sequential   Run agents sequentially (default: concurrent)"
            echo "  --max-turns N      Maximum turns per agent (default: 10)"
            echo "  --timeout N        Saturn discovery timeout in seconds (default: 5)"
            echo "  --model NAME       Model to use (optional)"
            echo "  -h, --help         Show this help"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

cat << 'EOF'

 __  __       _ _   _          _                    _
|  \/  |_   _| | |_(_)        / \   __ _  ___ _ __ | |_
| |\/| | | | | | __| |_____  / _ \ / _` |/ _ \ '_ \| __|
| |  | | |_| | | |_| |_____/ ___ \ (_| |  __/ | | | |_
|_|  |_|\__,_|_|\__|_|    /_/   \_\__, |\___|_| |_|\__|
                                  |___/
         BRUTUS Multi-Agent Coordination Demo

EOF

# Step 1: Reset demo files
echo -e "\033[33m[1/3] Resetting demo files...\033[0m"
"$SCRIPT_DIR/reset.sh"
echo ""

# Step 2: Build brutus-test if needed
echo -e "\033[33m[2/3] Checking for brutus-test...\033[0m"
BRUTUS_TEST="$PROJECT_ROOT/brutus-test"
if [[ ! -f "$BRUTUS_TEST" ]]; then
    echo "Building brutus-test..."
    cd "$PROJECT_ROOT"
    go build -o brutus-test ./cmd/brutus-test
    cd "$SCRIPT_DIR"
fi
echo -e "\033[32mbrutus-test is ready\033[0m"
echo ""

# Step 3: Run the demo
echo -e "\033[33m[3/3] Running multi-agent demo...\033[0m"
echo ""

ARGS=""
if $VERBOSE; then ARGS="$ARGS -v"; fi
if $SEQUENTIAL; then ARGS="$ARGS -concurrent=false"; fi

if $LIVE; then
    echo -e "\033[35mMode: LIVE (using real Saturn LLM)\033[0m"
    echo -e "\033[90mNOTE: Requires a Saturn beacon on the network!\033[0m"
    echo ""

    ARGS="$ARGS -max-turns=$MAX_TURNS -timeout=$TIMEOUT"
    if [[ -n "$MODEL" ]]; then ARGS="$ARGS -model=$MODEL"; fi

    "$BRUTUS_TEST" live-multi-agent $ARGS "$SCRIPT_DIR/live-scenario.json"
else
    echo -e "\033[34mMode: MOCKED (no network required)\033[0m"
    echo ""

    "$BRUTUS_TEST" multi-agent $ARGS "$SCRIPT_DIR/multi-scenario.json"
fi

echo ""
echo -e "\033[32mDemo completed successfully!\033[0m"
