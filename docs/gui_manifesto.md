# BRUTUS GUI Manifesto

> The definitive anchor document for building BRUTUS's graphical interface.
> All GUI development should trace back to principles established here.

---

## Vision Statement

**BRUTUS GUI is a command center for agentic coding.**

It is not an IDE. There is no code editor, no file browser, no plugin system. The human doesn't touch code—that's what the agent is for.

The GUI exists for one purpose: **multi-agent visibility**. Launch agents, watch them work, see costs accumulate, intervene when needed. Think camera control unit, not text editor.

Core metaphor: **Autumn command center**—warm, mature, dense with information, refined through focus.

---

## Research Foundation

This manifesto is built on extensive research into:
- Existing coding agent GUIs (Cursor, Windsurf, Claude Code, Goose, Aider-Desk)
- Common user complaints and pain points
- Favored design patterns
- Technical implementation approaches

### Key Insights from Research

#### What Users Hate

| Category | Pain Point |
|----------|------------|
| **Performance** | Memory leaks, high CPU, sluggishness even on powerful machines |
| **Context Loss** | "Compacting conversation" with no visibility into what was lost |
| **Diff UX** | Can't copy deleted code, forced commit workflows, no granular accept/reject |
| **Approvals** | Too many confirmations OR inconsistent behavior (same command asks sometimes) |
| **Wrong Files** | Agent edits wrong file (same name, different dir), ignores explicit references |
| **Hijacked UX** | Shortcuts overridden, layouts reset on updates, no way to restore |

#### What Users Love

| Feature | Why It Works |
|---------|--------------|
| **Granular diff control** | Accept one hunk, reject another, copy from deleted |
| **Trust tiers** | Auto-approve reads, confirm writes, block destructive |
| **Visible context** | Show tokens used/remaining, what files are loaded |
| **Multiple modes** | Inline edit (Cmd+K), chat sidebar, background async |
| **Streaming with stop** | Token-by-token display, user can interrupt |
| **Clean defaults** | Minimal clutter, progressive disclosure |

---

## Technical Architecture

### Framework Decision

**Decision: Wails v2** (Confirmed 2026-01-15)

| Metric | Value |
|--------|-------|
| Framework | Wails v2.11.0 |
| Binary Size | 11 MB (GUI) / 12 MB (CLI) |
| Backend | Go (native) |
| Frontend | React + TypeScript + Vite |

**Why Wails:**
- Go backend matches BRUTUS architecture
- Single binary distribution
- Native WebView2 rendering (no embedded Chromium)
- Auto-generated TypeScript bindings for Go methods
- Active development with good documentation

**Alternatives Rejected:**
- Electron: 85MB+ binaries, memory hungry
- Tauri: Would require Rust, different language from core BRUTUS

### Core Components: Command Center Layout

```
┌────────────────────────────────────────────────────────────────────────────┐
│  STATUS BAR                                      Total: $2.47 | 3 agents   │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│  ┌──────────────────────────┐  ┌──────────────────────────┐               │
│  │ AGENT 1: Feature work    │  │ AGENT 2: Test coverage   │  [+ New Agent]│
│  │ $0.82 | opus | running   │  │ $1.23 | sonnet | running │               │
│  ├──────────────────────────┤  ├──────────────────────────┤               │
│  │ [Tool] edit: auth.go     │  │ [Tool] bash: go test     │               │
│  │ [Tool] read: config.go   │  │ > Running 47 tests...    │               │
│  │ [AI] Adding JWT parsing  │  │ [AI] Tests passing, now  │               │
│  │      logic to handler    │  │      adding edge cases   │               │
│  │                          │  │                          │               │
│  │ ░░░░░░░░░░░░░░░░░░░░░░░░ │  │ ░░░░░░░░░░░░░░░░░░░░░░░░ │               │
│  │ > _                      │  │ > _                      │               │
│  └──────────────────────────┘  └──────────────────────────┘               │
│                                                                            │
│  ┌──────────────────────────┐                                              │
│  │ AGENT 3: Docs            │                                              │
│  │ $0.42 | haiku | idle     │                                              │
│  ├──────────────────────────┤                                              │
│  │ [Done] Updated README    │                                              │
│  │ [Done] Added API docs    │                                              │
│  │                          │                                              │
│  │ ✓ Completed              │                                              │
│  │ > _                      │                                              │
│  └──────────────────────────┘                                              │
│                                                                            │
└────────────────────────────────────────────────────────────────────────────┘
```

**Key elements:**
- No sidebar, no file browser, no editor
- Each agent is a "camera feed" showing: cost, model, status, recent tool calls, conversation
- Agents can be arranged, resized, expanded
- Dense information display (Blender-inspired)
- Total cost always visible

### Technology Stack

| Layer | Technology | Rationale |
|-------|------------|-----------|
| Framework | Wails v2 | Go backend, native rendering, small binary |
| Frontend | React + TypeScript | Component ecosystem, type safety |
| Terminal | xterm.js | Industry standard, GPU-accelerated |
| Diff View | Monaco Editor | VS Code's engine, built-in diff support |
| Styling | Tailwind CSS | Utility-first, consistent design |
| State | Zustand | Minimal, performant React state |
| IPC | Wails bindings | Auto-generated TypeScript types |

### Communication Pattern

```
┌─────────────────────────────────────────────────────────────┐
│                      GO BACKEND                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ BRUTUS      │  │ PTY Manager │  │ Session Store       │  │
│  │ Agent Loop  │  │ (creack/pty)│  │ (SQLite/BoltDB)     │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
│         │                │                    │             │
│         └────────────────┼────────────────────┘             │
│                          │                                  │
│              Wails Method Bindings + Events                 │
└─────────────────────────────────────────────────────────────┘
                           │
                 Auto-generated TypeScript
                           │
┌─────────────────────────────────────────────────────────────┐
│                    REACT FRONTEND                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │ Terminal    │  │ Chat Panel  │  │ Diff Viewer         │  │
│  │ Component   │  │ + Streaming │  │ + Approval          │  │
│  └─────────────┘  └─────────────┘  └─────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## Design Principles

*Derived from interview—these override any conflicting earlier assumptions.*

### 1. Command Center, Not IDE
This is not an editor with AI bolted on. There is no code editor, no file browser, no syntax highlighting. The agent writes code. The human coordinates agents.

### 2. Multi-Agent First
The core value proposition is seeing multiple agents work simultaneously. Single-agent is just a degenerate case. Design for N agents from day one.

### 3. Complete Autonomy
Agents have full file control. No approval workflows, no confirmation dialogs, no "are you sure?" popups. This is for vibe coders who trust the agent. Intervention is the exception, not the rule.

### 4. Dense Information Display
Blender-inspired: pack information in. Cost, tokens, tool calls, conversation—all visible. Screen real estate is precious. Reward users who learn the density.

### 5. Speed is Non-Negotiable
If the user can feel the delay, we've failed. No animations that slow things down. No loading spinners that could be avoided. Instant feedback or nothing.

### 6. Focus Over Features
Do fewer things exceptionally well. No plugin system. No themes. No customization bike-shedding. One look, done. Ship it.

### 7. No Forced Workflows
Never require clicking through wizards. Never block with modals. Never hide the escape hatch. Let users skip, cancel, or ignore anything.

---

## Implementation Phases

*Revised based on interview—multi-agent is now core, not advanced.*

### Phase 0: Foundation
- [x] Interview & Vision (BRUTUS-zvs) ✓
- [x] Architecture Decision (BRUTUS-030) ✓ - Wails v2, 11MB binary
- [x] Project Scaffolding ✓ - React + TypeScript + Vite

### Phase 1: Single Agent Panel
- [ ] Wails project setup with React
- [ ] Single agent panel: input, output, tool calls
- [ ] Cost/token display per agent
- [ ] Streaming conversation output
- [ ] Connect to BRUTUS backend

### Phase 2: Multi-Agent Core
- [ ] Multiple agent panels (the killer feature)
- [ ] Launch new agent with project/task
- [ ] Arrange/resize agent panels
- [ ] Total cost aggregation
- [ ] Agent status indicators (running/idle/done)

### Phase 3: Agent Management
- [ ] Session persistence (resume agents)
- [ ] Agent templates/presets
- [ ] Keyboard shortcuts for agent control
- [ ] Focus mode (expand single agent)

### Phase 4: Polish
- [ ] Performance optimization
- [ ] Autumn color palette
- [ ] Dense information layout refinement
- [ ] Error handling without popups

---

## Interview Responses

*Completed 2026-01-15*

### Part 1: Identity & Audience

**Q1: Who is BRUTUS for?**
Tinkerers—curious devs who love customization and understanding, frustrated by black boxes.

**Q2: User-BRUTUS relationship?**
Fully agentic. BRUTUS is a tool the user wields, but also their sole writer of code. The human won't even have an IDE open. Complete commitment to vibe coding.

**Q3: Personality?**
The Operator—tactical, efficient, mission-focused. Military precision.

### Part 2: Aesthetic & Feel

**Q4: Season?**
Autumn—warm, mature, refined. Comfortable depth. Settled confidence.

**Q5: Interface inspiration?**
Blender—dense, powerful, rewards mastery. Everything accessible.

**Q6: Steal from?**
Warp terminal—modern blocks and AI integration, but too flashy. Take the concepts, tone down the chrome.

### Part 3: Workflow & Priorities

**Q7: Ideal 30-minute session?**
"I open three chat sessions and send off agents to work on different parts of the code. Like a camera control unit, but for agentic coding."

**Q8: Autonomy level?**
Complete autonomy. BRUTUS has full control over creating, writing, deleting files. This is for vibe coders. Approval/confirmation should not be a UI focus.

**Q9: Always-visible information?**
1. Cost/tokens
2. Conversation/log
3. Tool calls

### Part 4: Philosophy & Values

**Q10: A good GUI should feel like...?**
A command center—multiple screens, agents reporting in. You're coordinating operations.

**Q11: Trade-off?**
Focus over features. Do fewer things exceptionally well.

**Q12: Why choose GUI over CLI?**
Multi-agent visibility. Seeing multiple sessions at once—something CLI can't do well.

### Part 5: Anti-patterns & Fears

**Q13: Deal-breakers?**
- Slow/laggy—if I can feel the delay, I'm out
- Popup spam—constant notifications, confirmations, tooltips
- Forced workflow—can't skip steps, must click through wizards

**Q14: Features to explicitly NOT build?**
- Code editor (that's what the agent is for)
- Plugin system (closed system, no extension ecosystem)
- Themes/customization (one look, done, no bike-shedding)

**Q15: Desired feeling?**
Powerful—multiplied capability, doing 10x with same effort.

---

## Open Questions

1. Should GUI be a separate binary or embedded in BRUTUS?
2. How to handle GUI ↔ headless mode switching?
3. Session persistence format (SQLite? JSON? Git-tracked?)
4. Remote access / web mode feasibility?

---

## References

### Codebases Studied
- [block/goose](https://github.com/block/goose) - Rust + Electron
- [hotovo/aider-desk](https://github.com/hotovo/aider-desk) - Electron + React
- [wailsapp/wails](https://github.com/wailsapp/wails) - Go framework
- [CopilotKit](https://github.com/CopilotKit/CopilotKit) - HITL patterns
- [stream-monaco](https://github.com/Simon-He95/stream-monaco) - Streaming editor

### Key Libraries
- [xterm.js](https://xtermjs.org/) - Terminal emulator
- [Monaco Editor](https://microsoft.github.io/monaco-editor/) - Code editor + diff
- [creack/pty](https://github.com/creack/pty) - Go PTY
- [Tailwind CSS](https://tailwindcss.com/) - Styling

---

*Last Updated: 2026-01-15*
*Document Owner: BRUTUS GUI Team*
