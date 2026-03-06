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
  file_access: {
    enabled: boolean;
    allowed_paths: string[];
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
    file_access: {
      enabled: true,
      allowed_paths: [],
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
      const [generalResponse, fileAccessResponse] = await Promise.all([
        fetch('/api/config/general'),
        fetch('/api/config/file-access'),
      ]);
      if (!generalResponse.ok) {
        throw new Error(
          `Failed to fetch general config: ${generalResponse.status} ${generalResponse.statusText}`,
        );
      }
      if (!fileAccessResponse.ok) {
        throw new Error(
          `Failed to fetch file access config: ${fileAccessResponse.status} ${fileAccessResponse.statusText}`,
        );
      }
      const data = await generalResponse.json();
      const fileAccessData = await fileAccessResponse.json();
      setConfig({
        workplace: data.workplace || 'data',
        server: data.server || { address: '0.0.0.0', port: 1314 },
        gateway_protection: data.gateway_protection || { enabled: false, mode: '' },
        file_access: fileAccessData.file_access || { enabled: true, allowed_paths: [] },
      });
    } catch (error) {
      console.error('Failed to fetch config:', error);
      setMessageType('error');
      const errorMessage = error instanceof Error ? error.message : String(error);
      setMessage(errorMessage || t('settings.loadFailed'));
    }
  };

  const handleSave = async () => {
    setLoading(true);
    setMessage('');
    try {
      const generalResponse = await fetch('/api/config/general', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          workplace: config.workplace,
          server: config.server,
          gateway_protection: config.gateway_protection,
        }),
      });
      const fileAccessResponse = await fetch('/api/config/file-access', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ file_access: config.file_access }),
      });

      if (generalResponse.ok && fileAccessResponse.ok) {
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

  const toggleFileAccess = () => {
    setConfig({
      ...config,
      file_access: {
        ...config.file_access,
        enabled: !config.file_access.enabled,
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
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
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
        <h3>{t('settings.fileAccessControl')}</h3>
        <p className="config-description">{t('settings.fileAccessControlDesc')}</p>
        <div className="config-item">
          <label>{t('settings.fileAccessControl')}</label>
          <div
            className={`toggle-switch ${config.file_access.enabled ? 'active' : ''}`}
            onClick={toggleFileAccess}
            role="switch"
            tabIndex={0}
            aria-checked={config.file_access.enabled}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                toggleFileAccess();
              }
            }}
          >
            <div className="toggle-knob" />
            <span className="toggle-label">
              {config.file_access.enabled ? t('settings.fileAccessEnabled') : t('settings.fileAccessDisabled')}
            </span>
          </div>
        </div>
        <div className="config-item">
          <label>{t('settings.fileAccessAllowedPaths')}</label>
          <textarea
            value={config.file_access.allowed_paths.join('\n')}
            onChange={(e) =>
              setConfig({
                ...config,
                file_access: {
                  ...config.file_access,
                  allowed_paths: e.target.value
                    .split('\n')
                    .map((line) => line.trim())
                    .filter((line) => line.length > 0),
                },
              })
            }
            placeholder={t('settings.fileAccessAllowedPathsPlaceholder')}
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
