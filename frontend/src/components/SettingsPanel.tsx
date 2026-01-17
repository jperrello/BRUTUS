import { useState, useEffect } from 'react';
import './SettingsPanel.css';

export interface Settings {
  appearance: {
    theme: 'dark' | 'light' | 'system';
    fontSize: number;
    uiDensity: 'compact' | 'comfortable';
  };
  behavior: {
    autoScrollMessages: boolean;
    confirmBeforeStop: boolean;
    showToolResults: boolean;
  };
  agent: {
    defaultModel: string;
    autoApproveReadTools: boolean;
  };
  shortcuts: {
    commandPalette: string;
    newAgent: string;
    sendMessage: string;
    stopGeneration: string;
  };
}

const DEFAULT_SETTINGS: Settings = {
  appearance: {
    theme: 'dark',
    fontSize: 13,
    uiDensity: 'comfortable',
  },
  behavior: {
    autoScrollMessages: true,
    confirmBeforeStop: false,
    showToolResults: true,
  },
  agent: {
    defaultModel: '',
    autoApproveReadTools: false,
  },
  shortcuts: {
    commandPalette: 'Ctrl+K',
    newAgent: 'Ctrl+N',
    sendMessage: 'Ctrl+Enter',
    stopGeneration: 'Ctrl+.',
  },
};

interface SettingsPanelProps {
  isOpen: boolean;
  onClose: () => void;
  settings: Settings;
  onSettingsChange: (settings: Settings) => void;
}

type SettingsTab = 'appearance' | 'behavior' | 'agent' | 'shortcuts' | 'about';

export function SettingsPanel({ isOpen, onClose, settings, onSettingsChange }: SettingsPanelProps) {
  const [activeTab, setActiveTab] = useState<SettingsTab>('appearance');
  const [localSettings, setLocalSettings] = useState<Settings>(settings);

  useEffect(() => {
    setLocalSettings(settings);
  }, [settings]);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  const updateSetting = <T extends keyof Settings, K extends keyof Settings[T]>(
    category: T,
    key: K,
    value: Settings[T][K]
  ) => {
    const updated = {
      ...localSettings,
      [category]: {
        ...localSettings[category],
        [key]: value,
      },
    };
    setLocalSettings(updated);
    onSettingsChange(updated);
  };

  const handleReset = () => {
    setLocalSettings(DEFAULT_SETTINGS);
    onSettingsChange(DEFAULT_SETTINGS);
  };

  if (!isOpen) return null;

  return (
    <div className="settings-overlay" onClick={onClose}>
      <div className="settings-panel" onClick={e => e.stopPropagation()}>
        <div className="settings-header">
          <h2>Settings</h2>
          <button className="settings-close" onClick={onClose}>×</button>
        </div>

        <div className="settings-body">
          <div className="settings-tabs">
            <button
              className={`settings-tab ${activeTab === 'appearance' ? 'active' : ''}`}
              onClick={() => setActiveTab('appearance')}
            >
              Appearance
            </button>
            <button
              className={`settings-tab ${activeTab === 'behavior' ? 'active' : ''}`}
              onClick={() => setActiveTab('behavior')}
            >
              Behavior
            </button>
            <button
              className={`settings-tab ${activeTab === 'agent' ? 'active' : ''}`}
              onClick={() => setActiveTab('agent')}
            >
              Agent
            </button>
            <button
              className={`settings-tab ${activeTab === 'shortcuts' ? 'active' : ''}`}
              onClick={() => setActiveTab('shortcuts')}
            >
              Shortcuts
            </button>
            <button
              className={`settings-tab ${activeTab === 'about' ? 'active' : ''}`}
              onClick={() => setActiveTab('about')}
            >
              About
            </button>
          </div>

          <div className="settings-content">
            {activeTab === 'appearance' && (
              <div className="settings-section">
                <div className="setting-item">
                  <label className="setting-label">Theme</label>
                  <select
                    className="setting-select"
                    value={localSettings.appearance.theme}
                    onChange={e => updateSetting('appearance', 'theme', e.target.value as 'dark' | 'light' | 'system')}
                  >
                    <option value="dark">Dark</option>
                    <option value="light">Light (coming soon)</option>
                    <option value="system">System</option>
                  </select>
                </div>

                <div className="setting-item">
                  <label className="setting-label">Font Size</label>
                  <div className="setting-slider-group">
                    <input
                      type="range"
                      min="10"
                      max="18"
                      value={localSettings.appearance.fontSize}
                      onChange={e => updateSetting('appearance', 'fontSize', Number(e.target.value))}
                      className="setting-slider"
                    />
                    <span className="setting-value">{localSettings.appearance.fontSize}px</span>
                  </div>
                </div>

                <div className="setting-item">
                  <label className="setting-label">UI Density</label>
                  <select
                    className="setting-select"
                    value={localSettings.appearance.uiDensity}
                    onChange={e => updateSetting('appearance', 'uiDensity', e.target.value as 'compact' | 'comfortable')}
                  >
                    <option value="compact">Compact</option>
                    <option value="comfortable">Comfortable</option>
                  </select>
                </div>
              </div>
            )}

            {activeTab === 'behavior' && (
              <div className="settings-section">
                <div className="setting-item">
                  <label className="setting-label">Auto-scroll Messages</label>
                  <input
                    type="checkbox"
                    className="setting-checkbox"
                    checked={localSettings.behavior.autoScrollMessages}
                    onChange={e => updateSetting('behavior', 'autoScrollMessages', e.target.checked)}
                  />
                </div>

                <div className="setting-item">
                  <label className="setting-label">Confirm Before Stop</label>
                  <input
                    type="checkbox"
                    className="setting-checkbox"
                    checked={localSettings.behavior.confirmBeforeStop}
                    onChange={e => updateSetting('behavior', 'confirmBeforeStop', e.target.checked)}
                  />
                </div>

                <div className="setting-item">
                  <label className="setting-label">Show Tool Results</label>
                  <input
                    type="checkbox"
                    className="setting-checkbox"
                    checked={localSettings.behavior.showToolResults}
                    onChange={e => updateSetting('behavior', 'showToolResults', e.target.checked)}
                  />
                </div>
              </div>
            )}

            {activeTab === 'agent' && (
              <div className="settings-section">
                <div className="setting-item">
                  <label className="setting-label">Default Model</label>
                  <input
                    type="text"
                    className="setting-input"
                    placeholder="e.g., gpt-4, claude-3"
                    value={localSettings.agent.defaultModel}
                    onChange={e => updateSetting('agent', 'defaultModel', e.target.value)}
                  />
                </div>

                <div className="setting-item">
                  <label className="setting-label">Auto-approve Read Tools</label>
                  <input
                    type="checkbox"
                    className="setting-checkbox"
                    checked={localSettings.agent.autoApproveReadTools}
                    onChange={e => updateSetting('agent', 'autoApproveReadTools', e.target.checked)}
                  />
                  <span className="setting-hint">Automatically approve read_file, list_files, etc.</span>
                </div>
              </div>
            )}

            {activeTab === 'shortcuts' && (
              <div className="settings-section">
                <p className="shortcuts-note">Keyboard shortcuts reference. Customization coming soon.</p>

                <div className="shortcuts-list">
                  <div className="shortcut-row">
                    <span className="shortcut-action">Command Palette</span>
                    <span className="shortcut-key">Ctrl+K</span>
                  </div>
                  <div className="shortcut-row">
                    <span className="shortcut-action">New Agent</span>
                    <span className="shortcut-key">Ctrl+N</span>
                  </div>
                  <div className="shortcut-row">
                    <span className="shortcut-action">Send Message</span>
                    <span className="shortcut-key">Ctrl+Enter</span>
                  </div>
                  <div className="shortcut-row">
                    <span className="shortcut-action">Stop Generation</span>
                    <span className="shortcut-key">Ctrl+.</span>
                  </div>
                  <div className="shortcut-row">
                    <span className="shortcut-action">Settings</span>
                    <span className="shortcut-key">Ctrl+,</span>
                  </div>
                  <div className="shortcut-row">
                    <span className="shortcut-action">Close Modal</span>
                    <span className="shortcut-key">Escape</span>
                  </div>
                  <div className="shortcut-row">
                    <span className="shortcut-action">Approve Tool</span>
                    <span className="shortcut-key">Y</span>
                  </div>
                  <div className="shortcut-row">
                    <span className="shortcut-action">Deny Tool</span>
                    <span className="shortcut-key">N</span>
                  </div>
                  <div className="shortcut-row">
                    <span className="shortcut-action">Focus Agent 1/2/3</span>
                    <span className="shortcut-key">Ctrl+1/2/3</span>
                  </div>
                  <div className="shortcut-row">
                    <span className="shortcut-action">Show Shortcuts</span>
                    <span className="shortcut-key">Ctrl+Shift+/</span>
                  </div>
                </div>
              </div>
            )}

            {activeTab === 'about' && (
              <div className="settings-section about-section">
                <div className="about-logo">BRUTUS</div>
                <p className="about-description">
                  A Saturn-Powered Coding Agent with multi-agent orchestration capabilities.
                </p>
                <div className="about-links">
                  <a href="#" className="about-link">Documentation</a>
                  <a href="#" className="about-link">GitHub</a>
                </div>
                <p className="about-version">Version 0.1.0</p>
              </div>
            )}
          </div>
        </div>

        <div className="settings-footer">
          <button className="btn-reset" onClick={handleReset}>
            Reset to Defaults
          </button>
        </div>
      </div>
    </div>
  );
}

export { DEFAULT_SETTINGS };
