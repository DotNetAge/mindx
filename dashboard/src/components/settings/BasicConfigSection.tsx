import { useTranslation } from '../../i18n';
import type { ServerConfig, ModelConfig } from './types';

interface BasicConfigSectionProps {
  config: ServerConfig;
  models: ModelConfig[];
  onConfigChange: (updates: Partial<ServerConfig>) => void;
}

function formatModelOption(model: ModelConfig): string {
  if (model.description) {
    return `${model.description} (${model.name})`;
  }
  return model.name;
}

export default function BasicConfigSection({
  config, models, onConfigChange,
}: BasicConfigSectionProps) {
  const { t } = useTranslation();

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
          <option value="">{t('advanced.selectModel')}</option>
          {models.map(model => (
            <option key={model.name} value={model.name}>
              {formatModelOption(model)}
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
          <option value="">{t('advanced.selectModel')}</option>
          {models.map(model => (
            <option key={model.name} value={model.name}>
              {formatModelOption(model)}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}
