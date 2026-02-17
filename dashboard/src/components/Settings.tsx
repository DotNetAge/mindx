import { useState, useEffect } from 'react';
import { SaveIcon, RefreshIcon } from 'tdesign-icons-react';
import './styles/Settings.css';

interface ModelConfig {
  provider: string;
  model: string;
  apiKey: string;
  baseUrl: string;
  temperature: number;
  maxTokens: number;
}

interface AppConfig {
  theme: 'dark' | 'light';
  language: string;
  enableNotifications: boolean;
  autoSaveHistory: boolean;
}

export default function Settings() {
  const [activeTab, setActiveTab] = useState<'models' | 'skills' | 'general'>('models');
  const [loading, setLoading] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  
  const [modelConfig, setModelConfig] = useState<ModelConfig>({
    provider: 'ollama',
    model: 'llama3.2',
    apiKey: '',
    baseUrl: 'http://localhost:11434',
    temperature: 0.7,
    maxTokens: 2048,
  });

  const [appConfig, setAppConfig] = useState<AppConfig>({
    theme: 'dark',
    language: 'zh-CN',
    enableNotifications: true,
    autoSaveHistory: true,
  });

  useEffect(() => {
    loadSettings();
  }, []);

  const loadSettings = async () => {
    try {
      const response = await fetch('http://localhost:1314/api/settings');
      if (response.ok) {
        const data = await response.json();
        if (data.modelConfig) {
          setModelConfig(data.modelConfig);
        }
        if (data.appConfig) {
          setAppConfig(data.appConfig);
        }
      }
    } catch (error) {
      console.error('Failed to load settings:', error);
    }
  };

  const handleSave = async () => {
    setLoading(true);
    setSaveSuccess(false);
    try {
      const response = await fetch('http://localhost:1314/api/settings', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          modelConfig,
          appConfig,
        }),
      });

      if (response.ok) {
        setSaveSuccess(true);
        setTimeout(() => setSaveSuccess(false), 3000);
      } else {
        console.error('Failed to save settings');
      }
    } catch (error) {
      console.error('Error saving settings:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleReset = () => {
    setModelConfig({
      provider: 'ollama',
      model: 'llama3.2',
      apiKey: '',
      baseUrl: 'http://localhost:11434',
      temperature: 0.7,
      maxTokens: 2048,
    });
  };

  return (
    <div className="settings-container">
      <div className="settings-header">
        <h1>设置</h1>
        <div className="header-actions">
          <button className="action-btn secondary" onClick={handleReset}>
            <RefreshIcon size={16} />
            重置
          </button>
          <button 
            className={`action-btn primary ${loading ? 'loading' : ''} ${saveSuccess ? 'success' : ''}`} 
            onClick={handleSave}
            disabled={loading}
          >
            {loading ? (
              <span className="spinner"></span>
            ) : saveSuccess ? (
              <span className="checkmark">✓</span>
            ) : (
              <SaveIcon size={16} />
            )}
            {saveSuccess ? '已保存' : '保存'}
          </button>
        </div>
      </div>

      <div className="settings-tabs">
        <button
          className={`tab ${activeTab === 'models' ? 'active' : ''}`}
          onClick={() => setActiveTab('models')}
        >
          模型配置
        </button>
        <button
          className={`tab ${activeTab === 'skills' ? 'active' : ''}`}
          onClick={() => setActiveTab('skills')}
        >
          技能管理
        </button>
        <button
          className={`tab ${activeTab === 'general' ? 'active' : ''}`}
          onClick={() => setActiveTab('general')}
        >
          通用设置
        </button>
      </div>

      <div className="settings-content">
        {activeTab === 'models' && (
          <div className="settings-section">
            <h2>模型配置</h2>
            <div className="form-group">
              <label>提供商</label>
              <select
                value={modelConfig.provider}
                onChange={(e) => setModelConfig({ ...modelConfig, provider: e.target.value })}
              >
                <option value="ollama">Ollama</option>
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic</option>
              </select>
            </div>
            <div className="form-group">
              <label>模型</label>
              <input
                type="text"
                value={modelConfig.model}
                onChange={(e) => setModelConfig({ ...modelConfig, model: e.target.value })}
                placeholder="llama3.2"
              />
            </div>
            <div className="form-group">
              <label>API 密钥</label>
              <input
                type="password"
                value={modelConfig.apiKey}
                onChange={(e) => setModelConfig({ ...modelConfig, apiKey: e.target.value })}
                placeholder="••••••••"
              />
            </div>
            <div className="form-group">
              <label>基础 URL</label>
              <input
                type="text"
                value={modelConfig.baseUrl}
                onChange={(e) => setModelConfig({ ...modelConfig, baseUrl: e.target.value })}
                placeholder="http://localhost:11434"
              />
            </div>
            <div className="form-group">
              <label>温度: {modelConfig.temperature}</label>
              <input
                type="range"
                min="0"
                max="2"
                step="0.1"
                value={modelConfig.temperature}
                onChange={(e) => setModelConfig({ ...modelConfig, temperature: parseFloat(e.target.value) })}
              />
            </div>
            <div className="form-group">
              <label>最大 Tokens: {modelConfig.maxTokens}</label>
              <input
                type="number"
                min="1"
                max="8192"
                value={modelConfig.maxTokens}
                onChange={(e) => setModelConfig({ ...modelConfig, maxTokens: parseInt(e.target.value) })}
              />
            </div>
          </div>
        )}

        {activeTab === 'skills' && (
          <div className="settings-section">
            <h2>技能管理</h2>
            <div className="skills-list">
              <div className="skill-item">
                <div className="skill-info">
                  <h3>系统技能</h3>
                  <p>sysinfo, screenshot, voice, notify, volume</p>
                </div>
                <span className="skill-status enabled">启用</span>
              </div>
              <div className="skill-item">
                <div className="skill-info">
                  <h3>文件技能</h3>
                  <p>finder, search, clipboard</p>
                </div>
                <span className="skill-status enabled">启用</span>
              </div>
              <div className="skill-item">
                <div className="skill-info">
                  <h3>网络技能</h3>
                  <p>wifi, openurl</p>
                </div>
                <span className="skill-status enabled">启用</span>
              </div>
              <div className="skill-item">
                <div className="skill-info">
                  <h3>通信技能</h3>
                  <p>mail, imessage, contacts</p>
                </div>
                <span className="skill-status enabled">启用</span>
              </div>
            </div>
          </div>
        )}

        {activeTab === 'general' && (
          <div className="settings-section">
            <h2>通用设置</h2>
            <div className="form-group">
              <label>主题</label>
              <select
                value={appConfig.theme}
                onChange={(e) => setAppConfig({ ...appConfig, theme: e.target.value as 'dark' | 'light' })}
              >
                <option value="dark">深色</option>
                <option value="light">浅色</option>
              </select>
            </div>
            <div className="form-group">
              <label>语言</label>
              <select
                value={appConfig.language}
                onChange={(e) => setAppConfig({ ...appConfig, language: e.target.value })}
              >
                <option value="zh-CN">简体中文</option>
                <option value="en-US">English</option>
              </select>
            </div>
            <div className="form-group switch-group">
              <label>启用通知</label>
              <label className="switch">
                <input
                  type="checkbox"
                  checked={appConfig.enableNotifications}
                  onChange={(e) => setAppConfig({ ...appConfig, enableNotifications: e.target.checked })}
                />
                <span className="slider"></span>
              </label>
            </div>
            <div className="form-group switch-group">
              <label>自动保存历史</label>
              <label className="switch">
                <input
                  type="checkbox"
                  checked={appConfig.autoSaveHistory}
                  onChange={(e) => setAppConfig({ ...appConfig, autoSaveHistory: e.target.checked })}
                />
                <span className="slider"></span>
              </label>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
