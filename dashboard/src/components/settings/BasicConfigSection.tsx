import { useTranslation } from '../../i18n';
import type { ServerConfig, ModelConfig } from './types';

interface BasicConfigSectionProps {
  config: ServerConfig;
  models: ModelConfig[];
  testingModel: string;
  onConfigChange: (updates: Partial<ServerConfig>) => void;
  onTestModel: (modelName: string) => void;
}

export default function BasicConfigSection({
  config, models, testingModel, onConfigChange, onTestModel,
}: BasicConfigSectionProps) {
  const { t } = useTranslation();

  return (
    <div className="config-section">
      <h3>基础配置</h3>
      <div className="config-item">
        <label>版本</label>
        <input
          type="text"
          value={config.version}
          onChange={(e) => onConfigChange({ version: e.target.value })}
          title="版本号"
          placeholder="版本号"
        />
      </div>
      <div className="config-item">
        <label>主机地址</label>
        <input
          type="text"
          value={config.host}
          onChange={(e) => onConfigChange({ host: e.target.value })}
          title="主机地址"
          placeholder="localhost"
        />
      </div>
      <div className="config-item">
        <label>HTTP 端口</label>
        <input
          type="number"
          value={config.port}
          onChange={(e) => onConfigChange({ port: parseInt(e.target.value) || 911 })}
          title="HTTP 端口"
          placeholder="911"
        />
      </div>
      <div className="config-item">
        <label>WebSocket 端口</label>
        <input
          type="number"
          value={config.ws_port}
          onChange={(e) => onConfigChange({ ws_port: parseInt(e.target.value) || 1314 })}
          title="WebSocket 端口"
          placeholder="1314"
        />
      </div>
      <div className="config-item">
        <label>默认模型</label>
        <div className="model-input-group">
          <select
            value={config.default_model}
            onChange={(e) => onConfigChange({ default_model: e.target.value })}
            title="默认模型"
          >
            {models.map(model => (
              <option key={model.name} value={model.name}>
                {model.name} {model.description ? `(${model.description})` : ''}
              </option>
            ))}
          </select>
          <button
            className="test-button"
            onClick={() => onTestModel(config.default_model)}
            disabled={testingModel === config.default_model}
          >
            {testingModel === config.default_model ? t('advanced.testing') : t('advanced.test')}
          </button>
        </div>
      </div>
      <div className="config-item">
        <label>{t('advanced.indexModel')}</label>
        <select
          value={config.index_model}
          onChange={(e) => onConfigChange({ index_model: e.target.value })}
          title="索引模型"
        >
          {models.map(model => (
            <option key={model.name} value={model.name}>
              {model.name} {model.description ? `(${model.description})` : ''}
            </option>
          ))}
        </select>
      </div>
      <div className="config-item">
        <label>{t('advanced.embeddingModel')}</label>
        <select
          value={config.embedding_model}
          onChange={(e) => onConfigChange({ embedding_model: e.target.value })}
          title="嵌入模型"
        >
          {models.map(model => (
            <option key={model.name} value={model.name}>
              {model.name} {model.description ? `(${model.description})` : ''}
            </option>
          ))}
        </select>
      </div>
      <div className="config-item">
        <label>记忆模型</label>
        <select
          value={config.memory_model}
          onChange={(e) => onConfigChange({ memory_model: e.target.value })}
          title="记忆模型"
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
