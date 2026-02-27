import { useState } from 'react';
import { useTranslation } from '../../i18n';
import type { OllamaStatus, ServerConfig } from './types';

interface OllamaSectionProps {
  ollamaStatus: OllamaStatus | null;
  config: ServerConfig;
  onConfigChange: (updates: Partial<ServerConfig>) => void;
  onInstall: () => void;
  onSync?: () => void;
}

export default function OllamaSection({ 
  ollamaStatus, 
  config, 
  onConfigChange, 
  onInstall, 
  onSync 
}: OllamaSectionProps) {
  const { t } = useTranslation();
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<string>('');

  const handleTestConnection = async () => {
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
    } catch {
      setTestResult(t('advanced.ollamaConnectionFailed'));
    }
    setTesting(false);
  };

  const getStatusInfo = () => {
    if (!ollamaStatus) {
      return { status: 'checking', text: t('advanced.checking'), className: '' };
    }
    if (!ollamaStatus.installed) {
      return { status: 'not_installed', text: t('advanced.ollamaNotInstalled'), className: 'error' };
    }
    if (!ollamaStatus.running) {
      return { status: 'not_running', text: t('advanced.ollamaInstalledNotRunning'), className: 'warning' };
    }
    return { status: 'running', text: t('advanced.ollamaRunning'), className: 'ok' };
  };

  const statusInfo = getStatusInfo();

  return (
    <div className="config-section">
      <h3>{t('advanced.ollamaStatus')}</h3>
      <div className="ollama-status">
        <div className={`status-item ${statusInfo.className}`}>
          {statusInfo.text}
        </div>

        {ollamaStatus?.running && ollamaStatus.models && (
          <div className="models-list">
            <h4>{t('advanced.installedModels')}</h4>
            <pre>{ollamaStatus.models}</pre>
          </div>
        )}

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
              type="button"
              className="test-button"
              onClick={handleTestConnection}
              disabled={testing}
              title={t('advanced.testOllamaConnection')}
            >
              {testing ? t('advanced.testing') : t('advanced.test')}
            </button>
          </div>
          {testResult && <div className="test-result">{testResult}</div>}
        </div>

        <div className="ollama-actions">
          {statusInfo.status === 'not_installed' && (
            <button type="button" className="install-button" onClick={onInstall}>
              {t('advanced.installOllama')}
            </button>
          )}

          {statusInfo.status === 'running' && onSync && (
            <button type="button" className="install-button" onClick={onSync}>
              {t('advanced.syncOllamaModels')}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
