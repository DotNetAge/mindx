import { useTranslation } from '../../i18n';

interface OllamaSectionProps {
  ollamaStatus: any;
  onInstall: () => void;
  onSync?: () => void;
}

export default function OllamaSection({ ollamaStatus, onInstall, onSync }: OllamaSectionProps) {
  const { t } = useTranslation();

  return (
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
          <button className="install-button" onClick={onInstall}>
            {t('advanced.installOllama')}
          </button>
        )}

        {ollamaStatus?.installed && ollamaStatus?.running && onSync && (
          <button className="install-button" onClick={onSync}>
            {t('advanced.syncOllamaModels')}
          </button>
        )}
      </div>
    </div>
  );
}
