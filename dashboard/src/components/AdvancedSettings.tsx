import { useState, useEffect } from 'react';
import './AdvancedSettings.css';
import { useTranslation } from '../i18n';

interface ModelConfig {
  name: string;
  domain: string;
  api_key: string;
  base_url: string;
  temperature: number;
  max_tokens: number;
}

interface TokenBudget {
  reserved_output_tokens: number;
  min_history_rounds: number;
  avg_tokens_per_round: number;
}

interface BrainConfig {
  leftbrain: ModelConfig;
  rightbrain: ModelConfig;
  token_budget: TokenBudget;
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

interface AdvancedConfig {
  ollama_url: string;
  brain: BrainConfig;
  index_model: string;
  embedding: string;
  memory: MemoryConfig;
  vector_store: VectorStoreConfig;
}

export default function AdvancedSettings() {
  const [config, setConfig] = useState<AdvancedConfig | null>(null);
  const [loading, setLoading] = useState(false);
  const [ollamaStatus, setOllamaStatus] = useState<any>(null);
  const [testingModel, setTestingModel] = useState('');
  const [message, setMessage] = useState('');
  const { t } = useTranslation();

  useEffect(() => {
    fetchConfig();
    checkOllama();
  }, []);

  const fetchConfig = async () => {
    try {
      const response = await fetch('/api/config/advanced');
      const data = await response.json();
      setConfig(data);
    } catch (error) {
      console.error('Failed to fetch config:', error);
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
    if (!config) return;

    setLoading(true);
    setMessage('');
    try {
      const response = await fetch('/api/config/advanced', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(config),
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

  const updateLeftbrain = (field: keyof ModelConfig, value: string | number) => {
    if (!config) return;
    setConfig({
      ...config,
      brain: {
        ...config.brain,
        leftbrain: { ...config.brain.leftbrain, [field]: value }
      }
    });
  };

  const updateRightbrain = (field: keyof ModelConfig, value: string | number) => {
    if (!config) return;
    setConfig({
      ...config,
      brain: {
        ...config.brain,
        rightbrain: { ...config.brain.rightbrain, [field]: value }
      }
    });
  };

  const updateTokenBudget = (field: keyof TokenBudget, value: number) => {
    if (!config) return;
    setConfig({
      ...config,
      brain: {
        ...config.brain,
        token_budget: { ...config.brain.token_budget, [field]: value }
      }
    });
  };

  const updateMemory = (field: keyof MemoryConfig, value: string | boolean) => {
    if (!config) return;
    setConfig({
      ...config,
      memory: { ...config.memory, [field]: value }
    });
  };

  const updateVectorStore = (field: keyof VectorStoreConfig, value: string) => {
    if (!config) return;
    setConfig({
      ...config,
      vector_store: { ...config.vector_store, [field]: value }
    });
  };

  if (!config) {
    return <div className="loading">{t('common.loading')}</div>;
  }

  return (
    <div className="advanced-settings">
      <h2>{t('advanced.title')}</h2>

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
        <h3>{t('advanced.basicConfig')}</h3>
        <div className="config-item">
          <label>{t('advanced.ollamaUrl')}</label>
          <input
            type="text"
            value={config.ollama_url}
            onChange={(e) => setConfig({ ...config, ollama_url: e.target.value })}
          />
        </div>
        <div className="config-item">
          <label>{t('advanced.indexModel')}</label>
          <input
            type="text"
            value={config.index_model}
            onChange={(e) => setConfig({ ...config, index_model: e.target.value })}
          />
        </div>
        <div className="config-item">
          <label>{t('advanced.embeddingModel')}</label>
          <input
            type="text"
            value={config.embedding}
            onChange={(e) => setConfig({ ...config, embedding: e.target.value })}
          />
        </div>
      </div>

      <div className="config-section">
        <h3>{t('advanced.leftbrainConfig')}</h3>
        <p className="section-desc">{t('advanced.leftbrainDesc')}</p>
        <div className="brain-config">
          <div className="config-item">
            <label>{t('advanced.modelName')}</label>
            <div className="model-input-group">
              <input
                type="text"
                value={config.brain.leftbrain.name}
                onChange={(e) => updateLeftbrain('name', e.target.value)}
              />
              <button
                className="test-button"
                onClick={() => handleTestModel(config.brain.leftbrain.name)}
                disabled={testingModel === config.brain.leftbrain.name}
              >
                {testingModel === config.brain.leftbrain.name ? t('advanced.testing') : t('advanced.test')}
              </button>
            </div>
            <small>{t('advanced.mustSupportFC')}</small>
          </div>

          <div className="config-item">
            <label>{t('advanced.baseUrl')}</label>
            <input
              type="text"
              value={config.brain.leftbrain.base_url}
              onChange={(e) => updateLeftbrain('base_url', e.target.value)}
            />
          </div>

          <div className="config-item">
            <label>{t('advanced.apiKey')}</label>
            <input
              type="password"
              value={config.brain.leftbrain.api_key}
              onChange={(e) => updateLeftbrain('api_key', e.target.value)}
            />
          </div>

          <div className="config-item">
            <label>{t('advanced.temperature')}</label>
            <input
              type="number"
              step="0.1"
              min="0"
              max="2"
              value={config.brain.leftbrain.temperature}
              onChange={(e) => updateLeftbrain('temperature', parseFloat(e.target.value) || 0)}
            />
          </div>

          <div className="config-item">
            <label>{t('advanced.maxTokens')}</label>
            <input
              type="number"
              value={config.brain.leftbrain.max_tokens}
              onChange={(e) => updateLeftbrain('max_tokens', parseInt(e.target.value) || 0)}
            />
          </div>
        </div>
      </div>

      <div className="config-section">
        <h3>{t('advanced.rightbrainConfig')}</h3>
        <p className="section-desc">{t('advanced.rightbrainDesc')}</p>
        <div className="brain-config">
          <div className="config-item">
            <label>{t('advanced.modelName')}</label>
            <div className="model-input-group">
              <input
                type="text"
                value={config.brain.rightbrain.name}
                onChange={(e) => updateRightbrain('name', e.target.value)}
              />
              <button
                className="test-button"
                onClick={() => handleTestModel(config.brain.rightbrain.name)}
                disabled={testingModel === config.brain.rightbrain.name}
              >
                {testingModel === config.brain.rightbrain.name ? t('advanced.testing') : t('advanced.test')}
              </button>
            </div>
            <small>{t('advanced.mustSupportFC')}</small>
          </div>

          <div className="config-item">
            <label>{t('advanced.baseUrl')}</label>
            <input
              type="text"
              value={config.brain.rightbrain.base_url}
              onChange={(e) => updateRightbrain('base_url', e.target.value)}
            />
          </div>

          <div className="config-item">
            <label>{t('advanced.apiKey')}</label>
            <input
              type="password"
              value={config.brain.rightbrain.api_key}
              onChange={(e) => updateRightbrain('api_key', e.target.value)}
            />
          </div>

          <div className="config-item">
            <label>{t('advanced.temperature')}</label>
            <input
              type="number"
              step="0.1"
              min="0"
              max="2"
              value={config.brain.rightbrain.temperature}
              onChange={(e) => updateRightbrain('temperature', parseFloat(e.target.value) || 0)}
            />
          </div>

          <div className="config-item">
            <label>{t('advanced.maxTokens')}</label>
            <input
              type="number"
              value={config.brain.rightbrain.max_tokens}
              onChange={(e) => updateRightbrain('max_tokens', parseInt(e.target.value) || 0)}
            />
          </div>
        </div>
      </div>

      <div className="config-section">
        <h3>{t('advanced.tokenBudget')}</h3>
        <div className="config-item">
          <label>{t('advanced.reservedOutputTokens')}</label>
          <input
            type="number"
            value={config.brain.token_budget.reserved_output_tokens}
            onChange={(e) => updateTokenBudget('reserved_output_tokens', parseInt(e.target.value) || 0)}
          />
          <small>{t('advanced.reservedOutputTokensDesc')}</small>
        </div>
        <div className="config-item">
          <label>{t('advanced.minHistoryRounds')}</label>
          <input
            type="number"
            value={config.brain.token_budget.min_history_rounds}
            onChange={(e) => updateTokenBudget('min_history_rounds', parseInt(e.target.value) || 0)}
          />
        </div>
        <div className="config-item">
          <label>{t('advanced.avgTokensPerRound')}</label>
          <input
            type="number"
            value={config.brain.token_budget.avg_tokens_per_round}
            onChange={(e) => updateTokenBudget('avg_tokens_per_round', parseInt(e.target.value) || 0)}
          />
        </div>
      </div>

      <div className="config-section">
        <h3>{t('advanced.memoryConfig')}</h3>
        <div className="form-group">
          <label>
            <input
              type="checkbox"
              checked={config.memory.enabled}
              onChange={(e) => updateMemory('enabled', e.target.checked)}
            />
            {t('advanced.enableMemory')}
          </label>
        </div>

        <div className="config-item">
          <label>{t('advanced.summaryModel')}</label>
          <input
            type="text"
            value={config.memory.summary_model}
            onChange={(e) => updateMemory('summary_model', e.target.value)}
          />
        </div>

        <div className="config-item">
          <label>{t('advanced.keywordModel')}</label>
          <input
            type="text"
            value={config.memory.keyword_model}
            onChange={(e) => updateMemory('keyword_model', e.target.value)}
          />
        </div>

        <div className="config-item">
          <label>{t('advanced.schedule')}</label>
          <input
            type="text"
            value={config.memory.schedule}
            onChange={(e) => updateMemory('schedule', e.target.value)}
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
