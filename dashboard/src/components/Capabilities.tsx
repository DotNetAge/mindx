import { useState, useEffect } from 'react';
import { RefreshIcon, EditIcon, DeleteIcon} from 'tdesign-icons-react';
import CapabilityIcon from './CapabilityIcon';
import IconPicker from './IconPicker';
import './Capabilities.css';

interface Capability {
  name: string;
  title: string;
  icon: string;
  description: string;
  model: string;
  system_prompt: string;
  tools: string[];
  modality?: string[];
  enabled: boolean;
}

interface ModelConfig {
  name: string;
  description?: string;
  base_url: string;
  api_key: string;
  temperature: number;
  max_tokens: number;
}

interface CapabilitiesConfig {
  capabilities: Capability[];
  default_capability: string;
  fallback_to_local: boolean;
}

interface CapabilitiesResponse {
  capabilities: CapabilitiesConfig;
  models: {
    models: ModelConfig[];
  };
}

export default function Capabilities() {
  const [capabilities, setCapabilities] = useState<Capability[]>([]);
  const [models, setModels] = useState<ModelConfig[]>([]);
  const [config, setConfig] = useState({ default_capability: '', fallback_to_local: false });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [editingCapability, setEditingCapability] = useState<Capability | null>(null);
  const [editingPrompt, setEditingPrompt] = useState<{ name: string; prompt: string } | null>(null);
  const [showAddDialog, setShowAddDialog] = useState(false);
  const [promptText, setPromptText] = useState('');

  useEffect(() => {
    fetchCapabilities();
  }, []);

  const fetchCapabilities = async () => {
    try {
      setLoading(true);
      const response = await fetch('/api/config/capabilities');
      if (!response.ok) throw new Error('获取能力配置失败');
      const data: CapabilitiesResponse = await response.json();
      setCapabilities(data.capabilities?.capabilities || []);
      setConfig({
        default_capability: data.capabilities?.default_capability || '',
        fallback_to_local: data.capabilities?.fallback_to_local || false,
      });
      setModels(data.models?.models || []);
    } catch (error) {
      console.error('Failed to fetch capabilities:', error);
      setError(error instanceof Error ? error.message : '加载失败');
    } finally {
      setLoading(false);
    }
  };

  const saveCapabilities = async (newCapabilities: Capability[]) => {
    try {
      setLoading(true);
      const response = await fetch('/api/config/capabilities', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          capabilities: {
            capabilities: newCapabilities,
            default_capability: config.default_capability,
            fallback_to_local: config.fallback_to_local
          }
        }),
      });
      if (!response.ok) throw new Error('保存失败');
      await fetchCapabilities();
    } catch (error) {
      setError(error instanceof Error ? error.message : '保存失败');
    } finally {
      setLoading(false);
    }
  };

  const handleToggle = async (name: string) => {
    const newCapabilities = capabilities.map(cap => 
      cap.name === name ? { ...cap, enabled: !cap.enabled } : cap
    );
    await saveCapabilities(newCapabilities);
  };

  const handleDelete = async (name: string) => {
    if (!window.confirm(`确定要删除能力 "${name}" 吗？`)) return;
    const newCapabilities = capabilities.filter(c => c.name !== name);
    await saveCapabilities(newCapabilities);
  };

  const handleUpdate = async (name: string, updates: Partial<Capability>) => {
    const newCapabilities = capabilities.map(cap => 
      cap.name === name ? { ...cap, ...updates } : cap
    );
    await saveCapabilities(newCapabilities);
    setEditingCapability(null);
  };

  const handleSavePrompt = async () => {
    if (!editingPrompt) return;
    await handleUpdate(editingPrompt.name, { system_prompt: promptText });
    setEditingPrompt(null);
    setPromptText('');
  };

  const handleAdd = async (capability: Capability) => {
    const newCapabilities = [...capabilities, capability];
    await saveCapabilities(newCapabilities);
    setShowAddDialog(false);
  };

  return (
    <div className="capabilities-page">
      <div className="page-header">
        <h1>能力管理</h1>
        <button className="action-btn" onClick={fetchCapabilities} disabled={loading}>
          <RefreshIcon size={16} />
          刷新
        </button>
      </div>

      {error && (
        <div className="error-banner">
          <span>{error}</span>
          <button className="retry-btn" onClick={() => setError('')}>关闭</button>
        </div>
      )}



      {/* 能力列表 */}
      <div className="capabilities-section">
        <div className="section-header">
          <h2>能力列表 ({capabilities.length})</h2>
          <button className="add-btn" onClick={() => setShowAddDialog(true)}>
            + 添加能力
          </button>
        </div>

        {capabilities.length === 0 ? (
          <div className="empty-state">暂无能力配置</div>
        ) : (
          <div className="capabilities-grid">
            {capabilities.map(capability => (
              <div key={capability.name} className={`capability-card ${!capability.enabled ? 'disabled' : ''}`}>
                <div className="card-header">
                  <div className="capability-title">
                    {capability.icon && <CapabilityIcon iconName={capability.icon} className="capability-icon" size={20} />}
                    <h3>{capability.title || capability.name}</h3>
                    <span className="capability-name-badge">{capability.name}</span>
                    <span className={`status-badge ${capability.enabled ? 'enabled' : 'disabled'}`}>
                      {capability.enabled ? '已启用' : '已禁用'}
                    </span>
                  </div>
                  <label className="toggle-switch">
                    <input
                      type="checkbox"
                      checked={capability.enabled}
                      onChange={() => handleToggle(capability.name)}
                      disabled={loading}
                    />
                    <span className="toggle-slider"></span>
                  </label>
                </div>

                <p className="capability-description">{capability.description}</p>

                <div className="capability-info">
                  <div className="info-item">
                    <span className="info-label">模型:</span>
                    <span className="info-value">{capability.model}</span>
                  </div>
                </div>

                <div className="capability-actions">
                  <button
                    className="icon-btn prompt-btn"
                    onClick={() => {
                      setEditingPrompt({ name: capability.name, prompt: capability.system_prompt });
                      setPromptText(capability.system_prompt);
                    }}
                    title="编辑智能体定义"
                  >
                    <EditIcon size={16} />
                    智能体定义
                  </button>
                  <button
                    className="icon-btn edit-btn"
                    onClick={() => setEditingCapability(capability)}
                    title="编辑配置"
                  >
                    <EditIcon size={16} />
                    编辑
                  </button>
                  <button
                    className="icon-btn delete-btn"
                    onClick={() => handleDelete(capability.name)}
                    title="删除"
                  >
                    <DeleteIcon size={16} />
                    删除
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* 编辑配置弹窗 */}
      {editingCapability && (
        <EditDialog
          capability={editingCapability}
          models={models}
          onSave={(updates) => handleUpdate(editingCapability.name, updates)}
          onCancel={() => setEditingCapability(null)}
          loading={loading}
        />
      )}

      {/* 编辑智能体定义弹窗 */}
      {editingPrompt && (
        <PromptDialog
          name={editingPrompt.name}
          prompt={promptText}
          onChange={setPromptText}
          onSave={handleSavePrompt}
          onCancel={() => {
            setEditingPrompt(null);
            setPromptText('');
          }}
          loading={loading}
        />
      )}

      {/* 添加能力弹窗 */}
      {showAddDialog && (
        <AddDialog
          models={models}
          onSave={handleAdd}
          onCancel={() => setShowAddDialog(false)}
          loading={loading}
        />
      )}
    </div>
  );
}

function EditDialog({ capability, models, onSave, onCancel, loading }: {
  capability: Capability;
  models: ModelConfig[];
  onSave: (updates: Partial<Capability>) => void;
  onCancel: () => void;
  loading: boolean;
}) {
  const [formData, setFormData] = useState({
    title: capability.title,
    icon: capability.icon,
    description: capability.description,
    model: capability.model,
  });

  return (
    <div className="dialog-overlay" onClick={onCancel}>
      <div className="dialog-content" onClick={(e) => e.stopPropagation()}>
        <div className="dialog-header">
          <h3>编辑能力 - {capability.name}</h3>
        </div>
        <div className="dialog-body">
          <div className="form-group">
            <label>能力标题</label>
            <input
              type="text"
              value={formData.title}
              onChange={(e) => setFormData({ ...formData, title: e.target.value })}
            />
          </div>
          <div className="form-group">
            <label>能力图标</label>
            <IconPicker 
              value={formData.icon}
              onChange={(icon) => setFormData({ ...formData, icon })}
              placeholder="选择图标"
            />
            <div className="form-hint">
              支持从下拉列表选择 Ant Design 图标，或手动输入其他图标（格式：图标集:图标名，如 mdi:home, lucide:code）。<br/>
              更多图标请访问: <a href="https://icon-sets.iconify.design/" target="_blank" rel="noopener noreferrer">icon-sets.iconify.design</a>
            </div>
          </div>
          <div className="form-group">
            <label>能力说明</label>
            <input
              type="text"
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
            />
          </div>
          <div className="form-group">
            <label>模型名称</label>
            <select
              value={formData.model}
              onChange={(e) => setFormData({ ...formData, model: e.target.value })}
            >
              {models.map(model => (
                <option key={model.name} value={model.name}>
                  {model.name} {model.description ? `(${model.description})` : ''}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="dialog-footer">
          <button type="button" className="btn-secondary" onClick={onCancel} disabled={loading}>取消</button>
          <button type="button" className="btn-primary" onClick={() => onSave(formData)} disabled={loading}>
            {loading ? '保存中...' : '保存'}
          </button>
        </div>
      </div>
    </div>
  );
}

function PromptDialog({ name, prompt, onChange, onSave, onCancel, loading }: {
  name: string;
  prompt: string;
  onChange: (value: string) => void;
  onSave: () => void;
  onCancel: () => void;
  loading: boolean;
}) {
  return (
    <div className="dialog-overlay" onClick={onCancel}>
      <div className="dialog-content prompt-dialog" onClick={(e) => e.stopPropagation()}>
        <div className="dialog-header">
          <h3>智能体定义 - {name}</h3>
        </div>
        <div className="dialog-body">
          <div className="form-group full-height">
            <label>System Prompt</label>
            <textarea
              value={prompt}
              onChange={(e) => onChange(e.target.value)}
              placeholder="输入智能体的系统提示词，定义其角色、能力和行为规范..."
              spellCheck={false}
            />
            <div className="prompt-stats">
              <span>字符数: {prompt.length}</span>
              <span>估算tokens: {Math.ceil(prompt.length / 3)}</span>
            </div>
          </div>
        </div>
        <div className="dialog-footer">
          <button type="button" className="btn-secondary" onClick={onCancel} disabled={loading}>取消</button>
          <button type="button" className="btn-primary" onClick={onSave} disabled={loading}>
            {loading ? '保存中...' : '保存'}
          </button>
        </div>
      </div>
    </div>
  );
}

function AddDialog({ models, onSave, onCancel, loading }: {
  models: ModelConfig[];
  onSave: (capability: Capability) => void;
  onCancel: () => void;
  loading: boolean;
}) {
  const [formData, setFormData] = useState({
    name: '',
    title: '',
    icon: '',
    description: '',
    model: models[0]?.name || '',
    system_prompt: '',
    tools: [] as string[],
    enabled: true,
  });

  const handleSubmit = () => {
    if (!formData.name || !formData.model || !formData.system_prompt) {
      alert('请填写必填字段：名称、模型、智能体定义');
      return;
    }
    onSave(formData as Capability);
  };

  return (
    <div className="dialog-overlay" onClick={onCancel}>
      <div className="dialog-content" onClick={(e) => e.stopPropagation()}>
        <div className="dialog-header">
          <h3>添加新能力</h3>
        </div>
        <div className="dialog-body">
          <div className="form-group">
            <label>能力名称 *</label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              placeholder="例如: writing"
            />
          </div>
          <div className="form-group">
            <label>能力标题</label>
            <input
              type="text"
              value={formData.title}
              onChange={(e) => setFormData({ ...formData, title: e.target.value })}
              placeholder="例如: 网络爆文"
            />
          </div>
          <div className="form-group">
            <label>能力图标</label>
            <IconPicker 
              value={formData.icon}
              onChange={(icon) => setFormData({ ...formData, icon })}
              placeholder="选择图标"
            />
            <div className="form-hint">
              支持从下拉列表选择 Ant Design 图标，或手动输入其他图标（格式：图标集:图标名，如 mdi:home, lucide:code）。<br/>
              更多图标请访问: <a href="https://icon-sets.iconify.design/" target="_blank" rel="noopener noreferrer">icon-sets.iconify.design</a>
            </div>
          </div>
          <div className="form-group">
            <label>能力说明 *</label>
            <input
              type="text"
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              placeholder="简要描述这个能力的用途"
            />
          </div>
          <div className="form-group">
            <label>智能体定义 (System Prompt) *</label>
            <textarea
              value={formData.system_prompt}
              onChange={(e) => setFormData({ ...formData, system_prompt: e.target.value })}
              rows={4}
              placeholder="定义智能体的角色、能力和行为规范"
            />
          </div>
          <div className="form-group">
            <label>模型名称 *</label>
            <select
              value={formData.model}
              onChange={(e) => setFormData({ ...formData, model: e.target.value })}
            >
              {models.map(model => (
                <option key={model.name} value={model.name}>
                  {model.name} {model.description ? `(${model.description})` : ''}
                </option>
              ))}
            </select>
          </div>
          <div className="form-group">
            <label className="checkbox-label">
              <input
                type="checkbox"
                checked={formData.enabled}
                onChange={(e) => setFormData({ ...formData, enabled: e.target.checked })}
              />
              启用此能力
            </label>
          </div>
        </div>
        <div className="dialog-footer">
          <button type="button" className="btn-secondary" onClick={onCancel} disabled={loading}>取消</button>
          <button type="button" className="btn-primary" onClick={handleSubmit} disabled={loading}>
            {loading ? '添加中...' : '添加'}
          </button>
        </div>
      </div>
    </div>
  );
}
