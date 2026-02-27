import { useState, useEffect } from 'react';
import './AdvancedSettings.css';
import { useTranslation } from '../i18n';
import type { ServerConfig, ModelConfig, BrainHalfConfig, OllamaStatus } from './settings/types';
import OllamaSection from './settings/OllamaSection';
import BasicConfigSection from './settings/BasicConfigSection';
import BrainModelSection from './settings/BrainModelSection';

function deepMergeConfig(defaults: ServerConfig, loaded: Partial<ServerConfig>): ServerConfig {
  return {
    ...defaults,
    ...loaded,
    token_budget: { ...defaults.token_budget, ...loaded.token_budget },
    subconscious: { ...defaults.subconscious, ...loaded.subconscious },
    consciousness: { ...defaults.consciousness, ...loaded.consciousness },
    memory: { ...defaults.memory, ...loaded.memory },
    vector_store: { ...defaults.vector_store, ...loaded.vector_store },
    websocket: { ...defaults.websocket, ...loaded.websocket },
  };
}




const defaultConfig: ServerConfig = {
  version: '0.0.1',
  host: 'localhost',
  port: 911,
  ws_port: 1314,
  ollama_url: 'http://localhost:11434',
  token_budget: {
    reserved_output_tokens: 8192,
    min_history_rounds: 5,
    avg_tokens_per_round: 200,
  },
  subconscious: {
    left: '',
    right: '',
  },
  consciousness: {
    left: '',
    right: '',
  },
  embedding_model: '',
  default_model: '',
  memory: {
    enabled: false,
    summary_model: '',
    keyword_model: '',
    schedule: '',
  },
  vector_store: {
    type: 'badger',
    data_path: '',
  },
  websocket: {
    max_connections: 100,
    ping_interval: 30,
    allowed_origins: [],
    token: '',
  },
};

export default function AdvancedSettings() {
  const [config, setConfig] = useState<ServerConfig>(defaultConfig);
  const [loading, setLoading] = useState(false);
  const [ollamaStatus, setOllamaStatus] = useState<OllamaStatus | null>(null);
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
        setConfig(deepMergeConfig(defaultConfig, data.server || {}));
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

  const handleSyncOllamaModels = async () => {
    setLoading(true);
    setMessage('');
    try {
      const response = await fetch('/api/config/ollama-sync', {
        method: 'POST',
      });
      if (response.ok) {
        setMessage(t('advanced.syncOllamaModelsSuccess'));
        // Refresh models list after sync
        fetchModels();
      } else {
        setMessage(t('advanced.syncOllamaModelsFailed'));
      }
    } catch (error) {
      console.error('Failed to sync Ollama models:', error);
      setMessage(t('advanced.syncOllamaModelsFailed'));
    }
    setLoading(false);
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



  const updateSubconscious = (field: keyof BrainHalfConfig, value: string) => {
    setConfig({ ...config, subconscious: { ...config.subconscious, [field]: value } });
  };

  const updateConsciousness = (field: keyof BrainHalfConfig, value: string) => {
    setConfig({ ...config, consciousness: { ...config.consciousness, [field]: value } });
  };





  return (
    <div className="advanced-settings">
      <h2>{t('advanced.title')}</h2>
      {message && (
        <div className={`message ${message.includes(t('advanced.saveSuccess')) || message.includes(t('advanced.syncOllamaModelsSuccess')) || message.includes(t('advanced.ollamaInstalling')) ? 'success' : 'error'}`}>
          {message}
        </div>
      )}
      {loadError && (
        <div className="warning-banner">
          {t('advanced.loadConfigFailed')}
        </div>
      )}

      <OllamaSection 
        ollamaStatus={ollamaStatus} 
        config={config}
        onConfigChange={updateConfig}
        onInstall={handleInstallOllama} 
        onSync={handleSyncOllamaModels} 
      />

      <BasicConfigSection
        config={config}
        models={models}
        onConfigChange={updateConfig}
      />

      <div className="brain-models-container">
        <BrainModelSection
          title={t('advanced.subconsciousModel')}
          description={t('advanced.subconsciousDescription')}
          config={config.subconscious}
          models={models}
          onUpdate={updateSubconscious}
        />

        <BrainModelSection
          title={t('advanced.consciousnessModel')}
          description={t('advanced.consciousnessDescription')}
          config={config.consciousness}
          models={models}
          showFCNote
          onUpdate={updateConsciousness}
        />
      </div>



      <div className="config-actions">
        <button className="save-button" onClick={handleSave} disabled={loading}>
          {loading ? t('advanced.saving') : t('advanced.save')}
        </button>
      </div>
    </div>
  );
}
