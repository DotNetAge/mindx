import { useState, useEffect } from 'react';
import './AdvancedSettings.css';
import { useTranslation } from '../i18n';
import type { ServerConfig, ModelConfig, TokenBudgetConfig, BrainHalfConfig, MemoryConfig, VectorStoreConfig } from './settings/types';
import OllamaSection from './settings/OllamaSection';
import BasicConfigSection from './settings/BasicConfigSection';
import BrainModelSection from './settings/BrainModelSection';
import TokenBudgetSection from './settings/TokenBudgetSection';
import MemorySection from './settings/MemorySection';
import VectorStoreSection from './settings/VectorStoreSection';

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
        headers: { 'Content-Type': 'application/json' },
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
        headers: { 'Content-Type': 'application/json' },
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

  const updateConfig = (updates: Partial<ServerConfig>) => {
    setConfig({ ...config, ...updates });
  };

  const updateTokenBudget = (field: keyof TokenBudgetConfig, value: number) => {
    setConfig({ ...config, token_budget: { ...config.token_budget, [field]: value } });
  };

  const updateSubconscious = (field: keyof BrainHalfConfig, value: string) => {
    setConfig({ ...config, subconscious: { ...config.subconscious, [field]: value } });
  };

  const updateConsciousness = (field: keyof BrainHalfConfig, value: string) => {
    setConfig({ ...config, consciousness: { ...config.consciousness, [field]: value } });
  };

  const updateMemory = (field: keyof MemoryConfig, value: string | boolean) => {
    const current = config.memory ?? { enabled: false, summary_model: '', keyword_model: '', schedule: '' };
    setConfig({ ...config, memory: { ...current, [field]: value } });
  };

  const updateVectorStore = (field: keyof VectorStoreConfig, value: string) => {
    setConfig({ ...config, vector_store: { ...config.vector_store, [field]: value } });
  };

  return (
    <div className="advanced-settings">
      <h2>{t('advanced.title')}</h2>
      {loadError && (
        <div className="warning-banner">
          加载配置失败，当前显示的是默认配置
        </div>
      )}

      <OllamaSection ollamaStatus={ollamaStatus} onInstall={handleInstallOllama} />

      <BasicConfigSection
        config={config}
        models={models}
        testingModel={testingModel}
        onConfigChange={updateConfig}
        onTestModel={handleTestModel}
      />

      <BrainModelSection
        title="潜意识模型 (Subconscious)"
        description="用于快速响应和直觉处理的模型"
        config={config.subconscious}
        models={models}
        testingModel={testingModel}
        onUpdate={updateSubconscious}
        onTestModel={handleTestModel}
      />

      <BrainModelSection
        title="意识模型 (Consciousness)"
        description="用于深度思考和复杂推理的模型，需要支持 Function Calling"
        config={config.consciousness}
        models={models}
        testingModel={testingModel}
        showFCNote
        onUpdate={updateConsciousness}
        onTestModel={handleTestModel}
      />

      <TokenBudgetSection config={config.token_budget} onUpdate={updateTokenBudget} />

      <MemorySection config={config.memory} onUpdate={updateMemory} />

      <VectorStoreSection config={config.vector_store} onUpdate={updateVectorStore} />

      <div className="config-actions">
        <button className="save-button" onClick={handleSave} disabled={loading}>
          {loading ? t('advanced.saving') : t('advanced.save')}
        </button>
      </div>

      {message && <div className={`message ${message.includes(t('advanced.saveSuccess')) ? 'success' : 'error'}`}>{message}</div>}
    </div>
  );
}
