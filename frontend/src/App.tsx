import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import './App.css';
import { NewAgent, GetAgents, SendMessage, GetVersion, StopAgent, RespondToApproval, LaunchMultiAgentDemo } from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { DiffEditor } from '@monaco-editor/react';
import { CommandPalette } from './components/CommandPalette';
import { SettingsPanel, Settings, DEFAULT_SETTINGS } from './components/SettingsPanel';
import { useKeyboardShortcuts } from './hooks/useKeyboardShortcuts';

interface Agent {
  id: string;
  model: string;
  status: string;
  cost: number;
  serviceName?: string;
  serviceHost?: string;
  connected?: boolean;
}

interface Message {
  role: string;
  content: string;
  isTool?: boolean;
  isStreaming?: boolean;
}

interface ApprovalRequest {
  id: string;
  agentId: string;
  tool: string;
  arguments: string;
}

interface PendingDiff {
  id: string;
  file: string;
  original: string;
  modified: string;
}

interface CoordinationStatus {
  agent_id: string;
  status: string;
  current_task: string;
  last_action: string;
}

function ToolApprovalModal({ request, onApprove, onDeny }: {
  request: ApprovalRequest;
  onApprove: () => void;
  onDeny: () => void;
}) {
  let parsedArgs: any = {};
  try {
    parsedArgs = JSON.parse(request.arguments);
  } catch {}

  return (
    <div className="approval-modal-overlay">
      <div className="approval-modal">
        <div className="approval-header">
          <span className="approval-icon">⚠️</span>
          <span className="approval-title">Tool Approval Required</span>
        </div>
        <div className="approval-content">
          <div className="approval-tool">
            <span className="label">Tool:</span>
            <span className="value">{request.tool}</span>
          </div>
          <div className="approval-args">
            <span className="label">Arguments:</span>
            <pre className="value">{JSON.stringify(parsedArgs, null, 2)}</pre>
          </div>
        </div>
        <div className="approval-actions">
          <button className="btn-deny" onClick={onDeny}>
            Deny (n)
          </button>
          <button className="btn-approve" onClick={onApprove}>
            Approve (y)
          </button>
        </div>
      </div>
    </div>
  );
}

function DiffModal({ diff, onAccept, onReject }: {
  diff: PendingDiff;
  onAccept: () => void;
  onReject: () => void;
}) {
  return (
    <div className="diff-modal-overlay">
      <div className="diff-modal">
        <div className="diff-header">
          <span className="diff-title">Review Changes: {diff.file}</span>
        </div>
        <div className="diff-content">
          <DiffEditor
            height="400px"
            original={diff.original}
            modified={diff.modified}
            language="typescript"
            theme="vs-dark"
            options={{
              readOnly: true,
              renderSideBySide: true,
              minimap: { enabled: false },
            }}
          />
        </div>
        <div className="diff-actions">
          <button className="btn-reject" onClick={onReject}>
            Reject Changes
          </button>
          <button className="btn-accept" onClick={onAccept}>
            Accept Changes
          </button>
        </div>
      </div>
    </div>
  );
}

function AgentPanel({ agent, onSend, onStop }: {
  agent: Agent;
  onSend: (msg: string) => Promise<void>;
  onStop: () => void;
}) {
  const [input, setInput] = useState('');
  const [messages, setMessages] = useState<Message[]>([]);
  const [streamingContent, setStreamingContent] = useState('');
  const [approvalRequest, setApprovalRequest] = useState<ApprovalRequest | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages, streamingContent, scrollToBottom]);

  useEffect(() => {
    const unsubStream = EventsOn('agent:stream', (data: { id: string; content: string }) => {
      if (data.id === agent.id) {
        setStreamingContent(prev => prev + data.content);
      }
    });

    const unsubMessage = EventsOn('agent:message', (data: { id: string; role: string; content: string }) => {
      if (data.id === agent.id) {
        setStreamingContent('');
        setMessages(prev => [...prev, { role: data.role, content: data.content }]);
      }
    });

    const unsubTool = EventsOn('agent:tool', (data: { id: string; tool: string }) => {
      if (data.id === agent.id) {
        if (streamingContent) {
          setMessages(prev => [...prev, { role: 'assistant', content: streamingContent }]);
          setStreamingContent('');
        }
        setMessages(prev => [...prev, { role: 'tool', content: `[${data.tool}]`, isTool: true }]);
      }
    });

    const unsubToolResult = EventsOn('agent:tool_result', (data: { id: string; tool: string; result: string; isError: boolean }) => {
      if (data.id === agent.id) {
        setMessages(prev => [...prev, {
          role: data.isError ? 'error' : 'result',
          content: `${data.tool}: ${data.result}`,
          isTool: true
        }]);
      }
    });

    const unsubApproval = EventsOn('agent:approval_request', (data: ApprovalRequest) => {
      if (data.agentId === agent.id) {
        setApprovalRequest(data);
      }
    });

    const unsubError = EventsOn('agent:error', (data: { id: string; error: string }) => {
      if (data.id === agent.id) {
        setStreamingContent('');
        setMessages(prev => [...prev, { role: 'error', content: data.error }]);
      }
    });

    return () => {
      unsubStream();
      unsubMessage();
      unsubTool();
      unsubToolResult();
      unsubApproval();
      unsubError();
    };
  }, [agent.id, streamingContent]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!approvalRequest) return;

      if (e.key === 'y' || e.key === 'Y') {
        handleApprove();
      } else if (e.key === 'n' || e.key === 'N') {
        handleDeny();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [approvalRequest]);

  const handleApprove = () => {
    if (approvalRequest) {
      RespondToApproval(agent.id, approvalRequest.id, true, '');
      setApprovalRequest(null);
    }
  };

  const handleDeny = () => {
    if (approvalRequest) {
      RespondToApproval(agent.id, approvalRequest.id, false, 'User denied');
      setApprovalRequest(null);
    }
  };

  const handleSend = () => {
    if (!input.trim()) return;
    const message = input;
    setMessages(prev => [...prev, { role: 'user', content: message }]);
    setInput('');
    onSend(message).catch((err: Error) => {
      setMessages(prev => [...prev, { role: 'error', content: `Failed to send: ${err.message || err}` }]);
    });
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="agent-panel">
      {approvalRequest && (
        <ToolApprovalModal
          request={approvalRequest}
          onApprove={handleApprove}
          onDeny={handleDeny}
        />
      )}

      <div className="agent-header">
        <span className="agent-title">{agent.id}</span>
        <span className="agent-model">{agent.model || 'default'}</span>
        {agent.serviceName && (
          <span className="agent-service" title={`Host: ${agent.serviceHost || 'unknown'}`}>
            <span className={`service-indicator ${agent.connected ? 'connected' : 'disconnected'}`} />
            {agent.serviceName}
          </span>
        )}
        <span className={`agent-status status-${agent.status}`}>{agent.status}</span>
        <span className="agent-cost">${agent.cost.toFixed(2)}</span>
      </div>

      <div className="agent-messages">
        {messages.map((msg, i) => (
          <div key={i} className={`message message-${msg.role}`}>
            <span className="message-role">{msg.role}</span>
            <span className="message-content">{msg.content}</span>
          </div>
        ))}
        {streamingContent && (
          <div className="message message-assistant streaming">
            <span className="message-role">assistant</span>
            <span className="message-content">{streamingContent}<span className="cursor">▌</span></span>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      <div className="agent-input">
        <textarea
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Enter message..."
          rows={2}
          disabled={agent.status === 'running'}
        />
        {agent.status === 'running' ? (
          <button className="btn-stop" onClick={onStop}>
            Stop
          </button>
        ) : (
          <button onClick={handleSend}>
            Send
          </button>
        )}
      </div>
    </div>
  );
}

function CoordinationBar({ statuses }: { statuses: CoordinationStatus[] }) {
  if (statuses.length === 0) return null;

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'working': return '⚡';
      case 'idle': return '○';
      case 'done': return '✓';
      case 'stopped': return '■';
      default: return '○';
    }
  };

  return (
    <div className="coordination-bar">
      <span className="coordination-label">Agent Coordination:</span>
      {statuses.map(s => (
        <div key={s.agent_id} className={`coord-status coord-${s.status}`}>
          <span className="coord-icon">{getStatusIcon(s.status)}</span>
          <span className="coord-agent">{s.agent_id}</span>
          {s.status === 'working' && s.current_task && (
            <span className="coord-task">{s.current_task}</span>
          )}
        </div>
      ))}
    </div>
  );
}

function App() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [version, setVersion] = useState('');
  const [totalCost, setTotalCost] = useState(0);
  const [coordinationStatuses, setCoordinationStatuses] = useState<CoordinationStatus[]>([]);
  const [showCommandPalette, setShowCommandPalette] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [showShortcutsHelp, setShowShortcutsHelp] = useState(false);
  const [settings, setSettings] = useState<Settings>(DEFAULT_SETTINGS);
  const agentRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  const { registerShortcut, formatShortcut } = useKeyboardShortcuts();

  useEffect(() => {
    const savedSettings = localStorage.getItem('brutus-settings');
    if (savedSettings) {
      try {
        setSettings(JSON.parse(savedSettings));
      } catch {}
    }
  }, []);

  const handleSettingsChange = useCallback((newSettings: Settings) => {
    setSettings(newSettings);
    localStorage.setItem('brutus-settings', JSON.stringify(newSettings));
  }, []);

  useEffect(() => {
    const unsubPalette = registerShortcut({
      key: 'k',
      modifiers: ['ctrl'],
      action: () => setShowCommandPalette(true),
      description: 'Open command palette',
      category: 'General',
    });

    const unsubSettings = registerShortcut({
      key: ',',
      modifiers: ['ctrl'],
      action: () => setShowSettings(true),
      description: 'Open settings',
      category: 'General',
    });

    const unsubNewAgent = registerShortcut({
      key: 'n',
      modifiers: ['ctrl'],
      action: () => handleNewAgent(),
      description: 'New agent',
      category: 'Agents',
    });

    const unsubShortcuts = registerShortcut({
      key: '/',
      modifiers: ['ctrl', 'shift'],
      action: () => setShowSettings(s => { if (!s) return true; return s; }),
      description: 'Show shortcuts',
      category: 'Help',
    });

    const unsubEscape = registerShortcut({
      key: 'Escape',
      action: () => {
        setShowCommandPalette(false);
        setShowSettings(false);
        setShowShortcutsHelp(false);
      },
      description: 'Close modal',
      category: 'Navigation',
    });

    const unsubFocus1 = registerShortcut({
      key: '1',
      modifiers: ['ctrl'],
      action: () => focusAgent(0),
      description: 'Focus first agent',
      category: 'Navigation',
    });

    const unsubFocus2 = registerShortcut({
      key: '2',
      modifiers: ['ctrl'],
      action: () => focusAgent(1),
      description: 'Focus second agent',
      category: 'Navigation',
    });

    const unsubFocus3 = registerShortcut({
      key: '3',
      modifiers: ['ctrl'],
      action: () => focusAgent(2),
      description: 'Focus third agent',
      category: 'Navigation',
    });

    return () => {
      unsubPalette();
      unsubSettings();
      unsubNewAgent();
      unsubShortcuts();
      unsubEscape();
      unsubFocus1();
      unsubFocus2();
      unsubFocus3();
    };
  }, [registerShortcut]);

  const focusAgent = useCallback((index: number) => {
    const agentIds = agents.map(a => a.id);
    if (index < agentIds.length) {
      const agentEl = agentRefs.current.get(agentIds[index]);
      if (agentEl) {
        agentEl.scrollIntoView({ behavior: 'smooth', block: 'center' });
        const textarea = agentEl.querySelector('textarea');
        textarea?.focus();
      }
    }
  }, [agents]);

  const commands = useMemo(() => [
    { id: 'new-agent', label: 'New Agent', shortcut: 'Ctrl+N', category: 'Agents', action: () => handleNewAgent() },
    { id: 'launch-demo', label: 'Launch Multi-Agent Demo', category: 'Agents', action: () => handleLaunchDemo() },
    { id: 'settings', label: 'Open Settings', shortcut: 'Ctrl+,', category: 'General', action: () => setShowSettings(true) },
    { id: 'focus-1', label: 'Focus Agent 1', shortcut: 'Ctrl+1', category: 'Navigation', action: () => focusAgent(0) },
    { id: 'focus-2', label: 'Focus Agent 2', shortcut: 'Ctrl+2', category: 'Navigation', action: () => focusAgent(1) },
    { id: 'focus-3', label: 'Focus Agent 3', shortcut: 'Ctrl+3', category: 'Navigation', action: () => focusAgent(2) },
  ], [focusAgent]);

  useEffect(() => {
    GetVersion().then(setVersion);
    GetAgents().then(setAgents);

    EventsOn('agent:created', () => {
      GetAgents().then(setAgents);
    });

    EventsOn('agent:status', () => {
      GetAgents().then(list => {
        setAgents(list);
        setTotalCost(list.reduce((sum, a) => sum + a.cost, 0));
      });
    });

    EventsOn('coordination:status', (statuses: CoordinationStatus[]) => {
      setCoordinationStatuses(statuses || []);
    });
  }, []);

  const handleNewAgent = () => {
    NewAgent('');
  };

  const handleLaunchDemo = () => {
    LaunchMultiAgentDemo().then(() => {
      GetAgents().then(setAgents);
    });
  };

  const handleSendMessage = (agentId: string, message: string): Promise<void> => {
    return SendMessage(agentId, message);
  };

  const handleStopAgent = (agentId: string) => {
    StopAgent(agentId);
  };

  return (
    <div className="app">
      <header className="status-bar">
        <div className="status-left">
          <span className="logo">BRUTUS</span>
          <span className="version">v{version}</span>
        </div>
        <div className="status-center">
          <button className="cmd-palette-btn" onClick={() => setShowCommandPalette(true)} title="Command Palette (Ctrl+K)">
            <span className="cmd-icon">⌘</span>
            <span className="cmd-text">Commands</span>
            <span className="cmd-shortcut">Ctrl+K</span>
          </button>
        </div>
        <div className="status-right">
          <span className="total-cost">Total: ${totalCost.toFixed(2)}</span>
          <span className="agent-count">{agents.length} agent{agents.length !== 1 ? 's' : ''}</span>
          <button className="settings-btn" onClick={() => setShowSettings(true)} title="Settings (Ctrl+,)">
            ⚙
          </button>
        </div>
      </header>

      <main className="command-center">
        {agents.length === 0 ? (
          <div className="empty-state">
            <h2>No agents running</h2>
            <p>Launch an agent to begin</p>
            <div className="empty-state-buttons">
              <button className="btn-primary" onClick={handleNewAgent}>
                + New Agent
              </button>
              <button className="btn-demo" onClick={handleLaunchDemo}>
                ⚡ Launch Multi-Agent Demo
              </button>
            </div>
            <p className="shortcut-hint">Press <kbd>Ctrl+K</kbd> for command palette</p>
          </div>
        ) : (
          <div className="agent-grid">
            {agents.map(agent => (
              <div
                key={agent.id}
                ref={el => { if (el) agentRefs.current.set(agent.id, el); }}
              >
                <AgentPanel
                  agent={agent}
                  onSend={(msg) => handleSendMessage(agent.id, msg)}
                  onStop={() => handleStopAgent(agent.id)}
                />
              </div>
            ))}
            <button className="add-agent-btn" onClick={handleNewAgent}>
              + New Agent
            </button>
          </div>
        )}
      </main>

      <CoordinationBar statuses={coordinationStatuses} />

      <CommandPalette
        isOpen={showCommandPalette}
        onClose={() => setShowCommandPalette(false)}
        commands={commands}
      />

      <SettingsPanel
        isOpen={showSettings}
        onClose={() => setShowSettings(false)}
        settings={settings}
        onSettingsChange={handleSettingsChange}
      />
    </div>
  );
}

export default App;
