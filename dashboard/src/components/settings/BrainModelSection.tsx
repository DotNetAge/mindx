import { useTranslation } from '../../i18n';
import type { BrainHalfConfig, ModelConfig } from './types';

interface BrainModelSectionProps {
  title: string;
  description: string;
  config: BrainHalfConfig;
  models: ModelConfig[];
  showFCNote?: boolean;
  onUpdate: (field: keyof BrainHalfConfig, value: string) => void;
}

function formatModelOption(model: ModelConfig): string {
  if (model.description) {
    return `${model.description} (${model.name})`;
  }
  return model.name;
}

export default function BrainModelSection({
  title, description, config, models,
  onUpdate,
}: BrainModelSectionProps) {
  const { t } = useTranslation();

  return (
    <div className="config-section">
      <h3>{title}</h3>
      <p className="section-desc">{description}</p>

      <div className="brain-hemispheres-container">
        <div className="config-item">
          <label>{t('advanced.leftBrainModel')}</label>
          <select
            value={config.left}
            onChange={(e) => onUpdate('left', e.target.value)}
            title={`${title} ${t('advanced.leftBrainModel')}`}
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
          <label>{t('advanced.rightBrainModel')}</label>
          <select
            value={config.right}
            onChange={(e) => onUpdate('right', e.target.value)}
            title={`${title} ${t('advanced.rightBrainModel')}`}
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
    </div>
  );
}
