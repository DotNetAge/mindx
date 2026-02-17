import { useState, useEffect } from 'react';
import { SaveIcon, RefreshIcon } from 'tdesign-icons-react';
import './styles/Settings.css';

interface TokenBudgetConfig {
  reserved_output_tokens: number;
  min_history_rounds: number;
  avg_tokens_per_round: number;
}

interface BrainHalfConfig {
  default: string;
  left: string;
  right: string;
}

interface ServerConfig {
  version: string;
  host: string;
  port: number;
  ws_port: number;
  token_budget: TokenBudgetConfig;
  subconscious: BrainHalfConfig;
  consciousness: BrainHalfConfig;
  memory_model: string;
  index_model: string;
  embedding_model: string;
  default_model: string;
}

interface AppConfig {
  theme: 'dark' | 'light';
  language: string;
  enableNotifications: boolean;
  autoSaveHistory: boolean;
}

const defaultServerConfig: ServerConfig = {
  version: '0.0.1',
  host: 'localhost',
  port: 911,
  ws_port: 1314,
  token_budget: {
    reserved_output_tokens: 8192,
    min_history_rounds: 5,
    avg_tokens_per_round: 200,
  },
  subconscious: {
    default: '',
    left: '',
    right: '',
  },
  consciousness: {
    default: '',
    left: '',
    right: '',
  },
  memory_model: '',
  index_model: '',
  embedding_model: '',
  default_model: '',
};

export default function Settings() {
  const [activeTab, setActiveTab] = useState<'server' | 'general'>('server');
  const [loading, setLoading] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  
  const [serverConfig, setServerConfig] = useState<ServerConfig>(defaultServerConfig);
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
      const serverRes = await fetch('/api/config/server');
      
      if (serverRes.ok) {
        const data = await serverRes.json();
        setServerConfig({ ...defaultServerConfig, ...data.server });
      }
    } catch (error) {
      console.error('Failed to load settings:', error);
    }
  };

  const handleSave = async () => {
    setLoading(true);
    setSaveSuccess(false);
    try {
      const serverRes = await fetch('/api/config/server', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server: serverConfig }),
      });

      if (serverRes.ok) {
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

  const updateTokenBudget = (field: keyof TokenBudgetConfig, value: number) => {
    setServerConfig({
      ...serverConfig,
      token_budget: { ...serverConfig.token_budget, [field]: value }
    });
  };

  const updateSubconscious = (field: keyof BrainHalfConfig, value: string) => {
    setServerConfig({
      ...serverConfig,
      subconscious: { ...serverConfig.subconscious, [field]: value }
    });
  };

  const updateConsciousness = (field: keyof BrainHalfConfig, value: string) => {
    setServerConfig({
      ...serverConfig,
      consciousness: { ...serverConfig.consciousness, [field]: value }
    });
  };

  return (
    <div className="settings-container">
      <div className="settings-header">
        <h1>设置</h1>
        <div className="header-actions">
          <button className="action-btn secondary" onClick={loadSettings}>
            <RefreshIcon size={16} />
            刷新
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
          className={`tab ${activeTab === 'server' ? 'active' : ''}`}
          onClick={() => setActiveTab('server')}
        >
          服务器配置
        </button>
        <button
          className={`tab ${activeTab === 'general' ? 'active' : ''}`}
          onClick={() => setActiveTab('general')}
        >
          通用设置
        </button>
      </div>

      <div className="settings-content">
        {activeTab === 'server' && (
          <div className="settings-section">
            <h2>基础配置</h2>
            <div className="form-group">
              <label>版本</label>
              <input
                type="text"
                value={serverConfig.version}
                onChange={(e) => setServerConfig({ ...serverConfig, version: e.target.value })}
                title="版本号"
                placeholder="版本号"
              />
            </div>
            <div className="form-group">
              <label>主机地址</label>
              <input
                type="text"
                value={serverConfig.host}
                onChange={(e) => setServerConfig({ ...serverConfig, host: e.target.value })}
                title="主机地址"
                placeholder="localhost"
              />
            </div>
            <div className="form-group">
              <label>HTTP 端口</label>
              <input
                type="number"
                value={serverConfig.port}
                onChange={(e) => setServerConfig({ ...serverConfig, port: parseInt(e.target.value) || 911 })}
                title="HTTP 端口"
                placeholder="911"
              />
            </div>
            <div className="form-group">
              <label>WebSocket 端口</label>
              <input
                type="number"
                value={serverConfig.ws_port}
                onChange={(e) => setServerConfig({ ...serverConfig, ws_port: parseInt(e.target.value) || 1314 })}
                title="WebSocket 端口"
                placeholder="1314"
              />
            </div>
            <div className="form-group">
              <label>默认模型</label>
              <input
                type="text"
                value={serverConfig.default_model}
                onChange={(e) => setServerConfig({ ...serverConfig, default_model: e.target.value })}
                title="默认模型"
                placeholder="qwen3:0.6b"
              />
            </div>
            <div className="form-group">
              <label>记忆模型</label>
              <input
                type="text"
                value={serverConfig.memory_model}
                onChange={(e) => setServerConfig({ ...serverConfig, memory_model: e.target.value })}
                title="记忆模型"
                placeholder="qwen3:0.6b"
              />
            </div>
            <div className="form-group">
              <label>索引模型</label>
              <input
                type="text"
                value={serverConfig.index_model}
                onChange={(e) => setServerConfig({ ...serverConfig, index_model: e.target.value })}
                title="索引模型"
                placeholder="qwen3:0.6b"
              />
            </div>
            <div className="form-group">
              <label>嵌入模型</label>
              <input
                type="text"
                value={serverConfig.embedding_model}
                onChange={(e) => setServerConfig({ ...serverConfig, embedding_model: e.target.value })}
                title="嵌入模型"
                placeholder="qllama/bge-small-zh-v1.5:latest"
              />
            </div>

            <h2>潜意识模型 (Subconscious)</h2>
            <p className="section-desc">用于快速响应和直觉处理的模型</p>
            <div className="form-group">
              <label>默认模型</label>
              <input
                type="text"
                value={serverConfig.subconscious.default}
                onChange={(e) => updateSubconscious('default', e.target.value)}
                title="潜意识默认模型"
                placeholder="qwen3:0.6b"
              />
            </div>
            <div className="form-group">
              <label>左脑模型</label>
              <input
                type="text"
                value={serverConfig.subconscious.left}
                onChange={(e) => updateSubconscious('left', e.target.value)}
                title="潜意识左脑模型"
                placeholder="qwen3:0.6b"
              />
            </div>
            <div className="form-group">
              <label>右脑模型</label>
              <input
                type="text"
                value={serverConfig.subconscious.right}
                onChange={(e) => updateSubconscious('right', e.target.value)}
                title="潜意识右脑模型"
                placeholder="qwen3:0.6b"
              />
            </div>

            <h2>意识模型 (Consciousness)</h2>
            <p className="section-desc">用于深度思考和复杂推理的模型</p>
            <div className="form-group">
              <label>默认模型</label>
              <input
                type="text"
                value={serverConfig.consciousness.default}
                onChange={(e) => updateConsciousness('default', e.target.value)}
                title="意识默认模型"
                placeholder="qwen3:1.7b"
              />
            </div>
            <div className="form-group">
              <label>左脑模型</label>
              <input
                type="text"
                value={serverConfig.consciousness.left}
                onChange={(e) => updateConsciousness('left', e.target.value)}
                title="意识左脑模型"
                placeholder="qwen3:0.6b"
              />
            </div>
            <div className="form-group">
              <label>右脑模型</label>
              <input
                type="text"
                value={serverConfig.consciousness.right}
                onChange={(e) => updateConsciousness('right', e.target.value)}
                title="意识右脑模型"
                placeholder="qwen3:1.7b"
              />
            </div>

            <h2>Token 预算</h2>
            <div className="form-group">
              <label>预留输出 Tokens</label>
              <input
                type="number"
                value={serverConfig.token_budget.reserved_output_tokens}
                onChange={(e) => updateTokenBudget('reserved_output_tokens', parseInt(e.target.value) || 0)}
                title="预留输出 Token 数"
                placeholder="8192"
              />
            </div>
            <div className="form-group">
              <label>最小历史轮数</label>
              <input
                type="number"
                value={serverConfig.token_budget.min_history_rounds}
                onChange={(e) => updateTokenBudget('min_history_rounds', parseInt(e.target.value) || 0)}
                title="最小历史对话轮数"
                placeholder="5"
              />
            </div>
            <div className="form-group">
              <label>平均每轮 Tokens</label>
              <input
                type="number"
                value={serverConfig.token_budget.avg_tokens_per_round}
                onChange={(e) => updateTokenBudget('avg_tokens_per_round', parseInt(e.target.value) || 0)}
                title="单轮对话平均 Token 数"
                placeholder="200"
              />
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
                title="主题"
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
                title="语言"
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
                  title="启用通知"
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
                  title="自动保存历史"
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
