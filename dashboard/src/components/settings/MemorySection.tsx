import { useTranslation } from '../../i18n';
import type { MemoryConfig } from './types';

interface MemorySectionProps {
  config?: MemoryConfig;
  onUpdate: (field: keyof MemoryConfig, value: string | boolean) => void;
}

export default function MemorySection({ config, onUpdate }: MemorySectionProps) {
  const { t } = useTranslation();

  return (
    <div className="config-section">
      <h3>{t('advanced.memoryConfig')}</h3>
      <div className="form-group">
        <label>
          <input
            type="checkbox"
            checked={config?.enabled || false}
            onChange={(e) => onUpdate('enabled', e.target.checked)}
            title="启用记忆"
          />
          {t('advanced.enableMemory')}
        </label>
      </div>
      <div className="config-item">
        <label>{t('advanced.summaryModel')}</label>
        <input
          type="text"
          value={config?.summary_model || ''}
          onChange={(e) => onUpdate('summary_model', e.target.value)}
          title="摘要模型"
          placeholder="qwen3:0.6b"
        />
      </div>
      <div className="config-item">
        <label>{t('advanced.keywordModel')}</label>
        <input
          type="text"
          value={config?.keyword_model || ''}
          onChange={(e) => onUpdate('keyword_model', e.target.value)}
          title="关键词模型"
          placeholder="qwen3:0.6b"
        />
      </div>
      <div className="config-item">
        <label>{t('advanced.schedule')}</label>
        <input
          type="text"
          value={config?.schedule || ''}
          onChange={(e) => onUpdate('schedule', e.target.value)}
          title="调度时间"
          placeholder="0 2 * * *"
        />
        <small>{t('advanced.scheduleDesc')}</small>
      </div>
    </div>
  );
}
