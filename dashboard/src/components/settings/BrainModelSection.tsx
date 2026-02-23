import { useTranslation } from '../../i18n';
import type { BrainHalfConfig, ModelConfig } from './types';

interface BrainModelSectionProps {
  title: string;
  description: string;
  config: BrainHalfConfig;
  models: ModelConfig[];
  testingModel: string;
  showFCNote?: boolean;
  onUpdate: (field: keyof BrainHalfConfig, value: string) => void;
  onTestModel: (modelName: string) => void;
}

export default function BrainModelSection({
  title, description, config, models, testingModel, showFCNote,
  onUpdate, onTestModel,
}: BrainModelSectionProps) {
  const { t } = useTranslation();

  return (
    <div className="config-section">
      <h3>{title}</h3>
      <p className="section-desc">{description}</p>
      <div className="config-item">
        <label>默认模型</label>
        <div className="model-input-group">
          <select
            value={config.default}
            onChange={(e) => onUpdate('default', e.target.value)}
            title={`${title}默认模型`}
          >
            {models.map(model => (
              <option key={model.name} value={model.name}>
                {model.name} {model.description ? `(${model.description})` : ''}
              </option>
            ))}
          </select>
          <button
            className="test-button"
            onClick={() => onTestModel(config.default)}
            disabled={testingModel === config.default}
          >
            {testingModel === config.default ? t('advanced.testing') : t('advanced.test')}
          </button>
        </div>
        {showFCNote && <small>{t('advanced.mustSupportFC')}</small>}
      </div>
      <div className="config-item">
        <label>左脑模型</label>
        <select
          value={config.left}
          onChange={(e) => onUpdate('left', e.target.value)}
          title={`${title}左脑模型`}
        >
          {models.map(model => (
            <option key={model.name} value={model.name}>
              {model.name} {model.description ? `(${model.description})` : ''}
            </option>
          ))}
        </select>
      </div>
      <div className="config-item">
        <label>右脑模型</label>
        <select
          value={config.right}
          onChange={(e) => onUpdate('right', e.target.value)}
          title={`${title}右脑模型`}
        >
          {models.map(model => (
            <option key={model.name} value={model.name}>
              {model.name} {model.description ? `(${model.description})` : ''}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}
