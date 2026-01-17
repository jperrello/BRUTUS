# Reset script for multi-agent demo
# Run this before starting a fresh demo

$baseDir = Split-Path -Parent $MyInvocation.MyCommand.Path

# Reset mock1.txt
@"
# Mock File 1
This file will be edited by Agent 1.

TODO: Add a greeting function
"@ | Set-Content "$baseDir\mock1.txt"

# Reset mock2.txt
@"
# Mock File 2
This file will be edited by Agent 2.

TODO: Add a farewell function
"@ | Set-Content "$baseDir\mock2.txt"

# Reset agent-1 status
@"
# Agent 1 Status
Status: idle
Current task: none
Last action: none
"@ | Set-Content "$baseDir\status\agent-1.md"

# Reset agent-2 status
@"
# Agent 2 Status
Status: idle
Current task: none
Last action: none
"@ | Set-Content "$baseDir\status\agent-2.md"

Write-Host "Multi-agent demo files reset to initial state." -ForegroundColor Green
Write-Host ""
Write-Host "Files reset:"
Write-Host "  - mock1.txt"
Write-Host "  - mock2.txt"
Write-Host "  - status/agent-1.md"
Write-Host "  - status/agent-2.md"
