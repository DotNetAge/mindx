import { useState, useEffect } from 'react';
import './AdvancedSettings.css';
import { useTranslation } from '../i18n';

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

interface MemoryConfig {
  enabled: boolean;
  summary_model: string;
  keyword_model: string;
  schedule: string;
}

interface VectorStoreConfig {
  type: string;
  data_path: string;
}

interface ModelConfig {
  name: string;
  description?: string;
  base_url: string;
  api_key: string;
  temperature: number;
  max_tokens: number;
}

interface ServerConfig {
  version: string;
  host: string;
  port: number;
  ws_port: number;
  ollama_url?: string;
  token_budget: TokenBudgetConfig;
  subconscious: BrainHalfConfig;
  consciousness: BrainHalfConfig;
  memory_model: string;
  index_model: string;
  embedding_model: string;
  default_model: string;
  memory?: MemoryConfig;
  vector_store: VectorStoreConfig;
}

const defaultConfig: ServerConfig = {
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
  vector_store: {
    type: 'badger',
    data_path: '',
  },
};

export default function AdvancedSettings() {
  const [config, setConfig] = useState<ServerConfig>(defaultConfig);
  const [loading, setLoading] = useState(false);
  const [ollamaStatus, setOllamaStatus] = useState<any>(null);
  const [testingModel, setTestingModel] = useState('');
  const [message, setMessage] = useState('');
  const [loadError, setLoadError] = useState(false);
  const [models, setModels] = useState<ModelConfig[]>([]);
  const { t } = useTranslation();

  useEffect(() => {
    fetchConfig();
    fetchModels();
    checkOllama();
  }, []);

  const fetchConfig = async () => {
    try {
      const response = await fetch('/api/config/server');
      if (response.ok) {
        const data = await response.json();
        setConfig({ ...defaultConfig, ...data.server });
        setLoadError(false);
      } else {
        console.error('API returned status:', response.status);
        setLoadError(true);
        setMessage('加载服务器配置失败');
      }
    } catch (error) {
      console.error('Failed to fetch config:', error);
      setLoadError(true);
      setMessage('加载配置失败，使用默认配置');
    }
  };

  const checkOllama = async () => {
    try {
      const response = await fetch('/api/service/ollama-check');
      const data = await response.json();
      setOllamaStatus(data);
    } catch (error) {
      console.error('Failed to check Ollama:', error);
    }
  };

  const fetchModels = async () => {
    try {
      const response = await fetch('/api/config/capabilities');
      if (response.ok) {
        const data = await response.json();
        setModels(data.models?.models || []);
      }
    } catch (error) {
      console.error('Failed to fetch models:', error);
    }
  };

  const handleInstallOllama = async () => {
    try {
      const response = await fetch('/api/service/ollama-install', {
        method: 'POST',
      });

      if (response.ok) {
        setMessage(t('advanced.ollamaInstalling'));
      }
    } catch (error) {
      console.error('Failed to install Ollama:', error);
      setMessage(t('advanced.ollamaInstallFailed'));
    }
  };

  const handleTestModel = async (modelName: string) => {
    setTestingModel(modelName);
    setMessage('');

    try {
      const response = await fetch('/api/service/model-test', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ model_name: modelName }),
      });

      const data = await response.json();

      if (data.supports_fc) {
        setMessage(t('advanced.modelSupportFC', { model: modelName }));
      } else {
        setMessage(t('advanced.modelNotSupportFC', { model: modelName }));
      }
    } catch (error) {
      console.error('Failed to test model:', error);
      setMessage(t('advanced.modelTestFailed', { model: modelName }));
    }

    setTestingModel('');
  };

  const handleSave = async () => {
    setLoading(true);
    setMessage('');
    try {
      const response = await fetch('/api/config/server', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ server: config }),
      });

      if (response.ok) {
        setMessage(t('advanced.saveSuccess'));
      } else {
        setMessage(t('advanced.saveFailed'));
      }
    } catch (error) {
      console.error('Failed to save config:', error);
      setMessage(t('advanced.saveFailed'));
    }
    setLoading(false);
  };

  const updateTokenBudget = (field: keyof TokenBudgetConfig, value: number) => {
    setConfig({
      ...config,
      token_budget: { ...config.token_budget, [field]: value }
    });
  };

  const updateSubconscious = (field: keyof BrainHalfConfig, value: string) => {
    setConfig({
      ...config,
      subconscious: { ...config.subconscious, [field]: value }
    });
  };

  const updateConsciousness = (field: keyof BrainHalfConfig, value: string) => {
    setConfig({
      ...config,
      consciousness: { ...config.consciousness, [field]: value }
    });
  };

  const updateMemory = (field: keyof MemoryConfig, value: string | boolean) => {
    setConfig({
      ...config,
      memory: { ...config.memory, [field]: value }
    });
  };

  const updateVectorStore = (field: keyof VectorStoreConfig, value: string) => {
    setConfig({
      ...config,
      vector_store: { ...config.vector_store, [field]: value }
    });
  };

  return (
    <div className="advanced-settings">
      <h2>{t('advanced.title')}</h2>
      {loadError && (
        <div className="warning-banner">
          加载配置失败，当前显示的是默认配置
        </div>
      )}

      <div className="config-section">
        <h3>{t('advanced.ollamaStatus')}</h3>
        <div className="ollama-status">
          {ollamaStatus ? (
            <>
              <div className={`status-item ${ollamaStatus.installed ? 'ok' : 'error'}`}>
                {ollamaStatus.installed ? t('advanced.ollamaInstalled') : t('advanced.ollamaNotInstalled')}
              </div>
              {ollamaStatus.installed && (
                <>
                  <div className={`status-item ${ollamaStatus.running ? 'ok' : 'warning'}`}>
                    {ollamaStatus.running ? t('advanced.ollamaRunning') : t('advanced.ollamaNotRunning')}
                  </div>
                  {ollamaStatus.models && (
                    <div className="models-list">
                      <h4>{t('advanced.installedModels')}</h4>
                      <pre>{ollamaStatus.models}</pre>
                    </div>
                  )}
                </>
              )}
            </>
          ) : (
            <div className="status-item">{t('advanced.checking')}</div>
          )}

          {!ollamaStatus?.installed && (
            <button className="install-button" onClick={handleInstallOllama}>
              {t('advanced.installOllama')}
            </button>
          )}
        </div>
      </div>

      <div className="config-section">
        <h3>基础配置</h3>
        <div className="config-item">
          <label>版本</label>
          <input
            type="text"
            value={config.version}
            onChange={(e) => setConfig({ ...config, version: e.target.value })}
            title="版本号"
            placeholder="版本号"
          />
        </div>
        <div className="config-item">
          <label>主机地址</label>
          <input
            type="text"
            value={config.host}
            onChange={(e) => setConfig({ ...config, host: e.target.value })}
            title="主机地址"
            placeholder="localhost"
          />
        </div>
        <div className="config-item">
          <label>HTTP 端口</label>
          <input
            type="number"
            value={config.port}
            onChange={(e) => setConfig({ ...config, port: parseInt(e.target.value) || 911 })}
            title="HTTP 端口"
            placeholder="911"
          />
        </div>
        <div className="config-item">
          <label>WebSocket 端口</label>
          <input
            type="number"
            value={config.ws_port}
            onChange={(e) => setConfig({ ...config, ws_port: parseInt(e.target.value) || 1314 })}
            title="WebSocket 端口"
            placeholder="1314"
          />
        </div>
        <div className="config-item">
          <label>默认模型</label>
          <div className="model-input-group">
            <select
              value={config.default_model}
              onChange={(e) => setConfig({ ...config, default_model: e.target.value })}
              title="默认模型"
            >
              {models.map(model => (
                <option key={model.name} value={model.name}>
                  {model.name} {model.description ? `(${model.description})` : ''}
                </option>
              ))}
            </select>
            <button
              className="test-button"
              onClick={() => handleTestModel(config.default_model)}
              disabled={testingModel === config.default_model}
            >
              {testingModel === config.default_model ? t('advanced.testing') : t('advanced.test')}
            </button>
          </div>
        </div>
        <div className="config-item">
          <label>{t('advanced.indexModel')}</label>
          <select
            value={config.index_model}
            onChange={(e) => setConfig({ ...config, index_model: e.target.value })}
            title="索引模型"
          >
            {models.map(model => (
              <option key={model.name} value={model.name}>
                {model.name} {model.description ? `(${model.description})` : ''}
              </option>
            ))}
          </select>
        </div>
        <div className="config-item">
          <label>{t('advanced.embeddingModel')}</label>
          <select
            value={config.embedding_model}
            onChange={(e) => setConfig({ ...config, embedding_model: e.target.value })}
            title="嵌入模型"
          >
            {models.map(model => (
              <option key={model.name} value={model.name}>
                {model.name} {model.description ? `(${model.description})` : ''}
              </option>
            ))}
          </select>
        </div>
        <div className="config-item">
          <label>记忆模型</label>
          <select
            value={config.memory_model}
            onChange={(e) => setConfig({ ...config, memory_model: e.target.value })}
            title="记忆模型"
          >
            {models.map(model => (
              <option key={model.name} value={model.name}>
                {model.name} {model.description ? `(${model.description})` : ''}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="config-section">
        <h3>潜意识模型 (Subconscious)</h3>
        <p className="section-desc">用于快速响应和直觉处理的模型</p>
        <div className="config-item">
          <label>默认模型</label>
          <div className="model-input-group">
            <select
              value={config.subconscious.default}
              onChange={(e) => updateSubconscious('default', e.target.value)}
              title="潜意识默认模型"
            >
              {models.map(model => (
                <option key={model.name} value={model.name}>
                  {model.name} {model.description ? `(${model.description})` : ''}
                </option>
              ))}
            </select>
            <button
              className="test-button"
              onClick={() => handleTestModel(config.subconscious.default)}
              disabled={testingModel === config.subconscious.default}
            >
              {testingModel === config.subconscious.default ? t('advanced.testing') : t('advanced.test')}
            </button>
          </div>
        </div>
        <div className="config-item">
          <label>左脑模型</label>
          <select
            value={config.subconscious.left}
            onChange={(e) => updateSubconscious('left', e.target.value)}
            title="潜意识左脑模型"
          >
            {models.map(model => (
              <option key={model.name} value={model.name}>
                {model.name} {model.description ? `(${model.description})` : ''}
              </option>
            ))}
          </select>
        </div>
        <div className="config-item">
          <label>右脑模型</label>
          <select
            value={config.subconscious.right}
            onChange={(e) => updateSubconscious('right', e.target.value)}
            title="潜意识右脑模型"
          >
            {models.map(model => (
              <option key={model.name} value={model.name}>
                {model.name} {model.description ? `(${model.description})` : ''}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="config-section">
        <h3>意识模型 (Consciousness)</h3>
        <p className="section-desc">用于深度思考和复杂推理的模型，需要支持 Function Calling</p>
        <div className="config-item">
          <label>默认模型</label>
          <div className="model-input-group">
            <select
              value={config.consciousness.default}
              onChange={(e) => updateConsciousness('default', e.target.value)}
              title="意识默认模型"
            >
              {models.map(model => (
                <option key={model.name} value={model.name}>
                  {model.name} {model.description ? `(${model.description})` : ''}
                </option>
              ))}
            </select>
            <button
              className="test-button"
              onClick={() => handleTestModel(config.consciousness.default)}
              disabled={testingModel === config.consciousness.default}
            >
              {testingModel === config.consciousness.default ? t('advanced.testing') : t('advanced.test')}
            </button>
          </div>
          <small>{t('advanced.mustSupportFC')}</small>
        </div>
        <div className="config-item">
          <label>左脑模型</label>
          <select
            value={config.consciousness.left}
            onChange={(e) => updateConsciousness('left', e.target.value)}
            title="意识左脑模型"
          >
            {models.map(model => (
              <option key={model.name} value={model.name}>
                {model.name} {model.description ? `(${model.description})` : ''}
              </option>
            ))}
          </select>
        </div>
        <div className="config-item">
          <label>右脑模型</label>
          <select
            value={config.consciousness.right}
            onChange={(e) => updateConsciousness('right', e.target.value)}
            title="意识右脑模型"
          >
            {models.map(model => (
              <option key={model.name} value={model.name}>
                {model.name} {model.description ? `(${model.description})` : ''}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="config-section">
        <h3>{t('advanced.tokenBudget')}</h3>
        <div className="config-item">
          <label>{t('advanced.reservedOutputTokens')}</label>
          <input
            type="number"
            value={config.token_budget.reserved_output_tokens}
            onChange={(e) => updateTokenBudget('reserved_output_tokens', parseInt(e.target.value) || 0)}
            title="预留输出 Token 数"
            placeholder="8192"
          />
          <small>{t('advanced.reservedOutputTokensDesc')}</small>
        </div>
        <div className="config-item">
          <label>{t('advanced.minHistoryRounds')}</label>
          <input
            type="number"
            value={config.token_budget.min_history_rounds}
            onChange={(e) => updateTokenBudget('min_history_rounds', parseInt(e.target.value) || 0)}
            title="最小历史对话轮数"
            placeholder="5"
          />
        </div>
        <div className="config-item">
          <label>{t('advanced.avgTokensPerRound')}</label>
          <input
            type="number"
            value={config.token_budget.avg_tokens_per_round}
            onChange={(e) => updateTokenBudget('avg_tokens_per_round', parseInt(e.target.value) || 0)}
            title="单轮对话平均 Token 数"
            placeholder="200"
          />
        </div>
      </div>

      <div className="config-section">
        <h3>{t('advanced.memoryConfig')}</h3>
        <div className="form-group">
          <label>
            <input
              type="checkbox"
              checked={config.memory?.enabled || false}
              onChange={(e) => updateMemory('enabled', e.target.checked)}
              title="启用记忆"
            />
            {t('advanced.enableMemory')}
          </label>
        </div>

        <div className="config-item">
          <label>{t('advanced.summaryModel')}</label>
          <input
            type="text"
            value={config.memory?.summary_model || ''}
            onChange={(e) => updateMemory('summary_model', e.target.value)}
            title="摘要模型"
            placeholder="qwen3:0.6b"
          />
        </div>

        <div className="config-item">
          <label>{t('advanced.keywordModel')}</label>
          <input
            type="text"
            value={config.memory?.keyword_model || ''}
            onChange={(e) => updateMemory('keyword_model', e.target.value)}
            title="关键词模型"
            placeholder="qwen3:0.6b"
          />
        </div>

        <div className="config-item">
          <label>{t('advanced.schedule')}</label>
          <input
            type="text"
            value={config.memory?.schedule || ''}
            onChange={(e) => updateMemory('schedule', e.target.value)}
            title="调度时间"
            placeholder="0 2 * * *"
          />
          <small>{t('advanced.scheduleDesc')}</small>
        </div>
      </div>

      <div className="config-section">
        <h3>{t('advanced.vectorStore')}</h3>
        <div className="config-item">
          <label>{t('advanced.vectorStoreType')}</label>
          <select
            value={config.vector_store.type}
            onChange={(e) => updateVectorStore('type', e.target.value)}
            title="向量存储类型"
          >
            <option value="memory">{t('advanced.vectorStoreMemory')}</option>
            <option value="badger">{t('advanced.vectorStoreBadger')}</option>
          </select>
        </div>
        <div className="config-item">
          <label>{t('advanced.vectorStoreDataPath')}</label>
          <input
            type="text"
            value={config.vector_store.data_path}
            onChange={(e) => updateVectorStore('data_path', e.target.value)}
            title="数据路径"
            placeholder="数据存储路径"
          />
        </div>
      </div>

      <div className="config-actions">
        <button className="save-button" onClick={handleSave} disabled={loading}>
          {loading ? t('advanced.saving') : t('advanced.save')}
        </button>
      </div>

      {message && <div className={`message ${message.includes(t('advanced.saveSuccess')) ? 'success' : 'error'}`}>{message}</div>}
    </div>
  );
}
