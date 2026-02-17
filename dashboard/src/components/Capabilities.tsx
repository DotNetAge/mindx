import { useState, useEffect } from 'react';
import { RefreshIcon, EditIcon, DeleteIcon} from 'tdesign-icons-react';
import CapabilityIcon from './CapabilityIcon';
import './Capabilities.css';

interface Capability {
  name: string;
  title: string;
  icon: string;
  description: string;
  model: string;
  base_url: string;
  api_key: string;
  system_prompt: string;
  tools: string[];
  temperature: number;
  max_tokens: number;
  modality?: string[];
  enabled: boolean;
}

interface CapabilitiesResponse {
  capabilities: Capability[];
  default_capability: string;
  fallback_to_local: boolean;
  description: string;
}

export default function Capabilities() {
  const [capabilities, setCapabilities] = useState<Capability[]>([]);
  const [config, setConfig] = useState({ default_capability: '', fallback_to_local: false, description: '' });
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
      const response = await fetch('/api/capabilities');
      if (!response.ok) throw new Error('获取能力配置失败');
      const data: CapabilitiesResponse = await response.json();
      setCapabilities(data.capabilities || []);
      setConfig({
        default_capability: data.default_capability || '',
        fallback_to_local: data.fallback_to_local || false,
        description: data.description || ''
      });
    } catch (error) {
      console.error('Failed to fetch capabilities:', error);
      setError(error instanceof Error ? error.message : '加载失败');
    } finally {
      setLoading(false);
    }
  };

  const handleToggle = async (name: string) => {
    const capability = capabilities.find(c => c.name === name);
    if (!capability) return;

    const updated = { ...capability, enabled: !capability.enabled };
    try {
      const response = await fetch(`/api/capabilities?name=${name}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updated),
      });
      if (!response.ok) throw new Error('切换状态失败');
      await fetchCapabilities();
    } catch (error) {
      setError(error instanceof Error ? error.message : '操作失败');
    }
  };

  const handleDelete = async (name: string) => {
    if (!window.confirm(`确定要删除能力 "${name}" 吗？`)) return;
    
    setCapabilities(prev => prev.filter(c => c.name !== name));
    
    try {
      const response = await fetch(`/api/capabilities?name=${name}`, {
        method: 'DELETE',
      });
      if (!response.ok) {
        fetchCapabilities();
        throw new Error('删除失败');
      }
    } catch (error) {
      setError(error instanceof Error ? error.message : '删除失败');
    }
  };

  const handleUpdate = async (name: string, updates: Partial<Capability>) => {
    try {
      setLoading(true);
      const response = await fetch(`/api/capabilities?name=${name}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updates),
      });
      if (!response.ok) throw new Error('更新失败');
      setEditingCapability(null);
      await fetchCapabilities();
    } catch (error) {
      setError(error instanceof Error ? error.message : '更新失败');
    } finally {
      setLoading(false);
    }
  };

  const handleSavePrompt = async () => {
    if (!editingPrompt) return;
    try {
      setLoading(true);
      const response = await fetch(`/api/capabilities?name=${editingPrompt.name}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ system_prompt: promptText }),
      });
      if (!response.ok) throw new Error('保存失败');
      setEditingPrompt(null);
      setPromptText('');
      await fetchCapabilities();
    } catch (error) {
      setError(error instanceof Error ? error.message : '保存失败');
    } finally {
      setLoading(false);
    }
  };

  const handleAdd = async (capability: Capability) => {
    try {
      setLoading(true);
      const response = await fetch('/api/capabilities', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(capability),
      });
      if (!response.ok) throw new Error('添加失败');
      setShowAddDialog(false);
      await fetchCapabilities();
    } catch (error) {
      setError(error instanceof Error ? error.message : '添加失败');
    } finally {
      setLoading(false);
    }
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
                  <div className="info-item">
                    <span className="info-label">API地址:</span>
                    <span className="info-value">{capability.base_url || '默认'}</span>
                  </div>
                  <div className="info-item">
                    <span className="info-label">温度:</span>
                    <span className="info-value">{capability.temperature}</span>
                  </div>
                  <div className="info-item">
                    <span className="info-label">最大tokens:</span>
                    <span className="info-value">{capability.max_tokens}</span>
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
          onSave={handleAdd}
          onCancel={() => setShowAddDialog(false)}
          loading={loading}
        />
      )}
    </div>
  );
}

function EditDialog({ capability, onSave, onCancel, loading }: {
  capability: Capability;
  onSave: (updates: Partial<Capability>) => void;
  onCancel: () => void;
  loading: boolean;
}) {
  const [formData, setFormData] = useState({
    title: capability.title,
    icon: capability.icon,
    description: capability.description,
    model: capability.model,
    base_url: capability.base_url,
    api_key: capability.api_key,
    temperature: capability.temperature,
    max_tokens: capability.max_tokens,
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
            <input
              type="text"
              value={formData.icon}
              onChange={(e) => setFormData({ ...formData, icon: e.target.value })}
              placeholder="例如: ✍️"
            />
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
            <input
              type="text"
              value={formData.model}
              onChange={(e) => setFormData({ ...formData, model: e.target.value })}
            />
          </div>
          <div className="form-group">
            <label>API地址</label>
            <input
              type="text"
              value={formData.base_url}
              onChange={(e) => setFormData({ ...formData, base_url: e.target.value })}
              placeholder="https://api.openai.com/v1"
            />
          </div>
          <div className="form-group">
            <label>API Key</label>
            <input
              type="password"
              value={formData.api_key}
              onChange={(e) => setFormData({ ...formData, api_key: e.target.value })}
            />
          </div>
          <div className="form-row">
            <div className="form-group">
              <label>温度</label>
              <input
                type="number"
                step="0.1"
                min="0"
                max="2"
                value={formData.temperature}
                onChange={(e) => setFormData({ ...formData, temperature: parseFloat(e.target.value) })}
              />
            </div>
            <div className="form-group">
              <label>最大tokens</label>
              <input
                type="number"
                value={formData.max_tokens}
                onChange={(e) => setFormData({ ...formData, max_tokens: parseInt(e.target.value) })}
              />
            </div>
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

function AddDialog({ onSave, onCancel, loading }: {
  onSave: (capability: Capability) => void;
  onCancel: () => void;
  loading: boolean;
}) {
  const [formData, setFormData] = useState({
    name: '',
    title: '',
    icon: '',
    description: '',
    model: '',
    base_url: '',
    api_key: '',
    system_prompt: '',
    temperature: 0.7,
    max_tokens: 4096,
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
              placeholder="例如: gpt-4-capability"
            />
          </div>
          <div className="form-group">
            <label>能力标题</label>
            <input
              type="text"
              value={formData.title}
              onChange={(e) => setFormData({ ...formData, title: e.target.value })}
              placeholder="例如: GPT-4 智能助手"
            />
          </div>
          <div className="form-group">
            <label>能力图标</label>
            <input
              type="text"
              value={formData.icon}
              onChange={(e) => setFormData({ ...formData, icon: e.target.value })}
              placeholder="例如: ✍️"
            />
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
            <input
              type="text"
              value={formData.model}
              onChange={(e) => setFormData({ ...formData, model: e.target.value })}
              placeholder="例如: gpt-4o"
            />
          </div>
          <div className="form-group">
            <label>API地址</label>
            <input
              type="text"
              value={formData.base_url}
              onChange={(e) => setFormData({ ...formData, base_url: e.target.value })}
              placeholder="https://api.openai.com/v1"
            />
          </div>
          <div className="form-group">
            <label>API Key</label>
            <input
              type="password"
              value={formData.api_key}
              onChange={(e) => setFormData({ ...formData, api_key: e.target.value })}
              placeholder="sk-..."
            />
          </div>
          <div className="form-row">
            <div className="form-group">
              <label>温度</label>
              <input
                type="number"
                step="0.1"
                min="0"
                max="2"
                value={formData.temperature}
                onChange={(e) => setFormData({ ...formData, temperature: parseFloat(e.target.value) })}
              />
            </div>
            <div className="form-group">
              <label>最大tokens</label>
              <input
                type="number"
                value={formData.max_tokens}
                onChange={(e) => setFormData({ ...formData, max_tokens: parseInt(e.target.value) })}
              />
            </div>
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
