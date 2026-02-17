import { useState, useEffect } from 'react';
import { SaveIcon, RefreshIcon, AddIcon, DeleteIcon } from 'tdesign-icons-react';
import './styles/Models.css';

interface ModelConfig {
  name: string;
  description?: string;
  api_key: string;
  base_url: string;
  temperature: number;
  max_tokens: number;
}

export default function Models() {
  const [models, setModels] = useState<ModelConfig[]>([]);
  const [loading, setLoading] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [editingIndex, setEditingIndex] = useState<number | null>(null);

  useEffect(() => {
    loadModels();
  }, []);

  const loadModels = async () => {
    try {
      setLoading(true);
      const response = await fetch('/api/config/models');
      if (response.ok) {
        const data = await response.json();
        setModels(data.models?.models || []);
      }
    } catch (error) {
      console.error('Failed to load models:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    setLoading(true);
    setSaveSuccess(false);
    try {
      const response = await fetch('/api/config/models', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ models: { models } }),
      });

      if (response.ok) {
        setSaveSuccess(true);
        setTimeout(() => setSaveSuccess(false), 3000);
      } else {
        console.error('Failed to save models');
      }
    } catch (error) {
      console.error('Error saving models:', error);
    } finally {
      setLoading(false);
    }
  };

  const addModel = () => {
    setModels([
      ...models,
      {
        name: '',
        description: '',
        api_key: '',
        base_url: 'http://localhost:11434/v1',
        temperature: 0.7,
        max_tokens: 40960,
      },
    ]);
    setEditingIndex(models.length);
  };

  const removeModel = (index: number) => {
    setModels(models.filter((_, i) => i !== index));
    if (editingIndex === index) {
      setEditingIndex(null);
    }
  };

  const updateModel = (index: number, field: keyof ModelConfig, value: string | number) => {
    const updatedModels = [...models];
    updatedModels[index] = { ...updatedModels[index], [field]: value };
    setModels(updatedModels);
  };

  return (
    <div className="models-page">
      <div className="page-header">
        <h1>模型配置</h1>
        <div className="header-actions">
          <button className="action-btn secondary" onClick={loadModels} disabled={loading}>
            <RefreshIcon size={16} />
            刷新
          </button>
          <button className="action-btn add-btn" onClick={addModel} disabled={loading}>
            <AddIcon size={16} />
            添加模型
          </button>
          <button 
            className={`action-btn primary ${loading ? 'loading' : ''} ${saveSuccess ? 'success' : ''}`} 
            onClick={handleSave}
            disabled={loading}
          >
            {loading ? (
              <span className="spinner"></span>
            ) : saveSuccess ? (
              <span className="checkmark">✓</span>
            ) : (
              <SaveIcon size={16} />
            )}
            {saveSuccess ? '已保存' : '保存'}
          </button>
        </div>
      </div>

      <div className="models-content">
        {models.length === 0 ? (
          <div className="empty-state">
            <p>暂无模型配置</p>
            <button className="add-btn" onClick={addModel}>
              <AddIcon size={16} />
              添加第一个模型
            </button>
          </div>
        ) : (
          <div className="models-list">
            {models.map((model, index) => (
              <div key={index} className={`model-card ${editingIndex === index ? 'editing' : ''}`}>
                <div className="card-header">
                  <div className="model-title">
                    <h3>{model.name || `模型 ${index + 1}`}</h3>
                  </div>
                  <div className="card-actions">
                    <button 
                      className="icon-btn edit-btn"
                      onClick={() => setEditingIndex(editingIndex === index ? null : index)}
                      title={editingIndex === index ? '完成编辑' : '编辑'}
                    >
                      {editingIndex === index ? '✓' : <EditIcon size={16} />}
                    </button>
                    <button 
                      className="icon-btn delete-btn"
                      onClick={() => removeModel(index)}
                      title="删除"
                    >
                      <DeleteIcon size={16} />
                    </button>
                  </div>
                </div>

                {(editingIndex === index || !model.name) && (
                  <div className="card-body editing-mode">
                    <div className="form-group">
                      <label>模型名称</label>
                      <input
                        type="text"
                        value={model.name}
                        onChange={(e) => updateModel(index, 'name', e.target.value)}
                        placeholder="例如: qwen3:0.6b"
                      />
                    </div>
                    <div className="form-group">
                      <label>描述</label>
                      <input
                        type="text"
                        value={model.description || ''}
                        onChange={(e) => updateModel(index, 'description', e.target.value)}
                        placeholder="模型描述"
                      />
                    </div>
                    <div className="form-group">
                      <label>API 密钥</label>
                      <input
                        type="password"
                        value={model.api_key}
                        onChange={(e) => updateModel(index, 'api_key', e.target.value)}
                        placeholder="••••••••"
                      />
                    </div>
                    <div className="form-group">
                      <label>基础 URL</label>
                      <input
                        type="text"
                        value={model.base_url}
                        onChange={(e) => updateModel(index, 'base_url', e.target.value)}
                        placeholder="http://localhost:11434/v1"
                      />
                    </div>
                    <div className="form-row">
                      <div className="form-group">
                        <label>温度: {model.temperature}</label>
                        <input
                          type="range"
                          min="0"
                          max="2"
                          step="0.1"
                          value={model.temperature}
                          onChange={(e) => updateModel(index, 'temperature', parseFloat(e.target.value))}
                          title={`温度: ${model.temperature}`}
                        />
                      </div>
                      <div className="form-group">
                        <label>最大 Tokens: {model.max_tokens}</label>
                        <input
                          type="number"
                          min="1"
                          value={model.max_tokens}
                          onChange={(e) => updateModel(index, 'max_tokens', parseInt(e.target.value))}
                          placeholder="40960"
                        />
                      </div>
                    </div>
                  </div>
                )}

                {editingIndex !== index && model.name && (
                  <div className="card-body">
                    {model.description && (
                      <div className="info-row full-width">
                        <span className="info-label">描述</span>
                        <span className="info-value">{model.description}</span>
                      </div>
                    )}
                    <div className="info-row full-width">
                      <span className="info-label">API 地址</span>
                      <span className="info-value url">{model.base_url}</span>
                    </div>
                    <div className="info-row-split">
                      <div className="info-item-compact">
                        <span className="info-label">温度</span>
                        <span className="info-value">{model.temperature}</span>
                      </div>
                      <div className="info-item-compact">
                        <span className="info-label">最大 Tokens</span>
                        <span className="info-value">{model.max_tokens.toLocaleString()}</span>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function EditIcon({ size }: { size: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
      <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
    </svg>
  );
}
