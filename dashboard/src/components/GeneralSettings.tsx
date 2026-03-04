import { useState, useEffect } from 'react';
import { useTranslation } from '../i18n';
import './GeneralSettings.css';

interface GeneralConfig {
  workplace: string;
  server: {
    address: string;
    port: number;
  };
  gateway_protection: {
    enabled: boolean;
    mode: string;
  };
}

export default function GeneralSettings() {
  const { t } = useTranslation();
  const [config, setConfig] = useState<GeneralConfig>({
    workplace: 'data',
    server: {
      address: '0.0.0.0',
      port: 1314,
    },
    gateway_protection: {
      enabled: false,
      mode: '',
    },
  });
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [messageType, setMessageType] = useState<'success' | 'error'>('success');

  useEffect(() => {
    fetchConfig();
  }, []);

  const fetchConfig = async () => {
    try {
      const response = await fetch('/api/config/general');
      const data = await response.json();
      setConfig({
        workplace: data.workplace || 'data',
        server: data.server || { address: '0.0.0.0', port: 1314 },
        gateway_protection: data.gateway_protection || { enabled: false, mode: '' },
      });
    } catch (error) {
      console.error('Failed to fetch config:', error);
    }
  };

  const handleSave = async () => {
    setLoading(true);
    setMessage('');
    try {
      const response = await fetch('/api/config/general', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(config),
      });

      if (response.ok) {
        setMessageType('success');
        setMessage(t('settings.saveSuccess'));
      } else {
        setMessageType('error');
        setMessage(t('settings.saveFailed'));
      }
    } catch (error) {
      console.error('Failed to save config:', error);
      setMessageType('error');
      setMessage(t('settings.saveFailed'));
    }
    setLoading(false);
  };

  const toggleGatewayProtection = () => {
    setConfig({
      ...config,
      gateway_protection: {
        ...config.gateway_protection,
        enabled: !config.gateway_protection.enabled,
      },
    });
  };

  return (
    <div className="general-settings">
      <h2>{t('settings.general')}</h2>

      <div className="config-section">
        <h3>{t('settings.gatewayProtection')}</h3>
        <p className="config-description">{t('settings.gatewayProtectionDesc')}</p>
        <div className="config-item">
          <label>{t('settings.gatewayProtection')}</label>
          <div
            className={`toggle-switch ${config.gateway_protection.enabled ? 'active' : ''}`}
            onClick={toggleGatewayProtection}
            role="switch"
            tabIndex={0}
            aria-checked={config.gateway_protection.enabled}
            onKeyDown={(e) => {
              if (e.key === ' ') {
                e.preventDefault();
              }
              if (e.key === 'Enter' || e.key === ' ') {
                toggleGatewayProtection();
              }
            }}
          >
            <div className="toggle-knob" />
            <span className="toggle-label">
              {config.gateway_protection.enabled ? t('settings.gatewayEnabled') : t('settings.gatewayDisabled')}
            </span>
          </div>
        </div>
      </div>

      <div className="config-section">
        <h3>{t('settings.workspace')}</h3>
        <div className="config-item">
          <label>{t('settings.workspacePath')}</label>
          <input
            type="text"
            value={config.workplace}
            onChange={(e) => setConfig({ ...config, workplace: e.target.value })}
            placeholder={t('settings.workspacePathPlaceholder')}
          />
        </div>
      </div>

      <div className="config-section">
        <h3>{t('settings.serverSettings')}</h3>
        <div className="config-item">
          <label>{t('settings.serverAddress')}</label>
          <input
            type="text"
            value={config.server.address}
            onChange={(e) => setConfig({ ...config, server: { ...config.server, address: e.target.value } })}
            placeholder="0.0.0.0"
          />
        </div>
        <div className="config-item">
          <label>{t('settings.serverPort')}</label>
          <input
            type="number"
            value={config.server.port}
            onChange={(e) => setConfig({ ...config, server: { ...config.server, port: parseInt(e.target.value) } })}
            placeholder="1314"
          />
        </div>
      </div>

      <div className="config-actions">
        <button className="save-button" onClick={handleSave} disabled={loading}>
          {loading ? t('settings.saving') : t('settings.save')}
        </button>
      </div>

      {message && <div className={`message ${messageType}`}>{message}</div>}
    </div>
  );
}
