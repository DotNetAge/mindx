import { useState } from 'react';
import { useTranslation } from '../../i18n';
import type { ServerConfig, ModelConfig } from './types';

interface BasicConfigSectionProps {
  config: ServerConfig;
  models: ModelConfig[];
  onConfigChange: (updates: Partial<ServerConfig>) => void;
}

export default function BasicConfigSection({
  config, models, onConfigChange,
}: BasicConfigSectionProps) {
  const { t } = useTranslation();
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<string>('');

  const handleTestOllama = async () => {
    setTesting(true);
    setTestResult('');
    try {
      const ollamaURL = config.ollama_url || 'http://localhost:11434';
      const response = await fetch(`${ollamaURL}/api/tags`);
      if (response.ok) {
        setTestResult(t('advanced.ollamaConnectionSuccess'));
      } else {
        setTestResult(t('advanced.ollamaConnectionFailed'));
      }
    } catch (error) {
      setTestResult(t('advanced.ollamaConnectionFailed'));
    }
    setTesting(false);
  };

  return (
    <div className="config-section">
      <h3>{t('advanced.basicConfig')}</h3>
      <div className="config-item">
        <label>{t('advanced.defaultModel')}</label>
        <select
          value={config.default_model}
          onChange={(e) => onConfigChange({ default_model: e.target.value })}
          title={t('advanced.defaultModel')}
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
          onChange={(e) => onConfigChange({ embedding_model: e.target.value })}
          title={t('advanced.embeddingModel')}
        >
          {models.map(model => (
            <option key={model.name} value={model.name}>
              {model.name} {model.description ? `(${model.description})` : ''}
            </option>
          ))}
        </select>
      </div>
      <div className="config-item">
        <label>{t('advanced.ollamaUrl')}</label>
        <div className="model-input-group">
          <input
            type="text"
            value={config.ollama_url || ''}
            onChange={(e) => onConfigChange({ ollama_url: e.target.value })}
            placeholder="http://localhost:11434"
            title={t('advanced.ollamaUrl')}
          />
          <button
            className="test-button"
            onClick={handleTestOllama}
            disabled={testing}
            title={t('advanced.testOllamaConnection')}
          >
            {testing ? t('advanced.testing') : t('advanced.test')}
          </button>
        </div>
        {testResult && <div className="test-result">{testResult}</div>}
      </div>
    </div>
  );
}
