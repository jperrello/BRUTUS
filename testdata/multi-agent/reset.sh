#!/bin/bash
# Reset script for multi-agent demo
# Run this before starting a fresh demo

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Reset mock1.txt
cat > "$SCRIPT_DIR/mock1.txt" << 'EOF'
# Mock File 1
This file will be edited by Agent 1.

TODO: Add a greeting function
EOF

# Reset mock2.txt
cat > "$SCRIPT_DIR/mock2.txt" << 'EOF'
# Mock File 2
This file will be edited by Agent 2.

TODO: Add a farewell function
EOF

# Reset agent-1 status
cat > "$SCRIPT_DIR/status/agent-1.md" << 'EOF'
# Agent 1 Status
Status: idle
Current task: none
Last action: none
EOF

# Reset agent-2 status
cat > "$SCRIPT_DIR/status/agent-2.md" << 'EOF'
# Agent 2 Status
Status: idle
Current task: none
Last action: none
EOF

echo -e "\033[32mMulti-agent demo files reset to initial state.\033[0m"
echo ""
echo "Files reset:"
echo "  - mock1.txt"
echo "  - mock2.txt"
echo "  - status/agent-1.md"
echo "  - status/agent-2.md"
