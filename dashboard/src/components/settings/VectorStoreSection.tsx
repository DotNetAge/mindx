import { useTranslation } from '../../i18n';
import type { VectorStoreConfig } from './types';

interface VectorStoreSectionProps {
  config: VectorStoreConfig;
  onUpdate: (field: keyof VectorStoreConfig, value: string) => void;
}

export default function VectorStoreSection({ config, onUpdate }: VectorStoreSectionProps) {
  const { t } = useTranslation();

  return (
    <div className="config-section">
      <h3>{t('advanced.vectorStore')}</h3>
      <div className="config-item">
        <label>{t('advanced.vectorStoreType')}</label>
        <select
          value={config.type}
          onChange={(e) => onUpdate('type', e.target.value)}
          title="向量存储类型"
        >
          <option value="memory">{t('advanced.vectorStoreMemory')}</option>
          <option value="badger">{t('advanced.vectorStoreBadger')}</option>
        </select>
      </div>
      <div className="config-item">
        <label>{t('advanced.vectorStoreDataPath')}</label>
        <input
          type="text"
          value={config.data_path}
          onChange={(e) => onUpdate('data_path', e.target.value)}
          title="数据路径"
          placeholder="数据存储路径"
        />
      </div>
    </div>
  );
}
