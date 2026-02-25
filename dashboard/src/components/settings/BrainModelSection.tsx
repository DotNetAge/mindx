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

export default function BrainModelSection({
  title, description, config, models, showFCNote,
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
            {models.map(model => (
              <option key={model.name} value={model.name}>
                {model.name} {model.description ? `(${model.description})` : ''}
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
            {models.map(model => (
              <option key={model.name} value={model.name}>
                {model.name} {model.description ? `(${model.description})` : ''}
              </option>
            ))}
          </select>
        </div>
      </div>
    </div>
  );
}
