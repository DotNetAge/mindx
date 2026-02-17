import { useState, useEffect } from 'react';
import { SaveIcon, PlayIcon, StopIcon, RefreshIcon, CheckIcon, CloseIcon } from 'tdesign-icons-react';
import './styles/Channels.css';

const getChannelLogo = (channelId: string) => {
  switch (channelId) {
    case 'feishu':
      return (
        <svg width="32" height="32" viewBox="0 0 1024 1024" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect width="1024" height="1024" rx="128" fill="#3370FF"/>
          <path d="M405.76 230.4a25.6 25.6 0 0 0-25.6 25.6v179.2h179.2a25.6 25.6 0 0 0 0-51.2H431.36V256a25.6 25.6 0 0 0-25.6-25.6z" fill="white"/>
          <path d="M618.24 230.4a25.6 25.6 0 0 0-25.6 25.6v537.6h179.2a25.6 25.6 0 0 0 0-51.2H643.84V256a25.6 25.6 0 0 0-25.6-25.6z" fill="white"/>
          <path d="M291.84 409.6a25.6 25.6 0 0 0-25.6 25.6v358.4h179.2a25.6 25.6 0 0 0 0-51.2H317.44V435.2a25.6 25.6 0 0 0-25.6-25.6z" fill="white"/>
          <path d="M512 409.6a25.6 25.6 0 0 0-25.6 25.6v358.4h179.2a25.6 25.6 0 0 0 0-51.2H537.6V435.2a25.6 25.6 0 0 0-25.6-25.6z" fill="white"/>
        </svg>
      );
    case 'wechat':
      return (
        <svg width="32" height="32" viewBox="0 0 1024 1024" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect width="1024" height="1024" rx="128" fill="#7BB32E"/>
          <path d="M512 192c-176.64 0-320 121.6-320 272 0 76.8 38.4 147.2 102.4 201.6l-32 96 118.4-54.4c38.4 9.6 76.8 14.4 121.6 14.4 176.64 0 320-121.6 320-272 0-150.4-143.36-272-320-272zM371.2 409.6a38.4 38.4 0 1 1 0 76.8 38.4 38.4 0 0 1 0-76.8zM652.8 409.6a38.4 38.4 0 1 1 0 76.8 38.4 38.4 0 0 1 0-76.8z" fill="white"/>
        </svg>
      );
    case 'qq':
      return (
        <svg width="32" height="32" viewBox="0 0 1024 1024" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect width="1024" height="1024" rx="128" fill="#12B7F5"/>
          <path d="M512 192c-128 0-230.4 96-230.4 224 0 44.8 19.2 89.6 51.2 128l-19.2 70.4 83.2-44.8c32 9.6 64 12.8 102.4 12.8 38.4 0 70.4-3.2 102.4-12.8l83.2 44.8-19.2-70.4c32-38.4 51.2-83.2 51.2-128 0-128-102.4-224-230.4-224zM435.2 448a38.4 38.4 0 1 1 0 76.8 38.4 38.4 0 0 1 0-76.8zM588.8 448a38.4 38.4 0 1 1 0 76.8 38.4 38.4 0 0 1 0-76.8z" fill="white"/>
        </svg>
      );
    case 'dingtalk':
      return (
        <svg width="32" height="32" viewBox="0 0 1024 1024" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect width="1024" height="1024" rx="128" fill="#0089FF"/>
          <path d="M512 192c-170.666667 0-309.333333 138.666667-309.333333 309.333333s138.666667 309.333333 309.333333 309.333333 309.333333-138.666667 309.333333-309.333333S682.666667 192 512 192z m64 458.666667h-128v-42.666667h128v42.666667z m32-106.666667h-192v-42.666667h192v42.666667z" fill="white"/>
        </svg>
      );
    case 'whatsapp':
      return (
        <svg width="32" height="32" viewBox="0 0 1024 1024" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect width="1024" height="1024" rx="128" fill="#25D366"/>
          <path d="M512 192c-176.64 0-320 143.36-320 320 0 56.32 14.72 109.44 42.88 156.16L192 832l169.6-44.16A318.72 318.72 0 0 0 512 832c176.64 0 320-143.36 320-320S688.64 192 512 192z" fill="white"/>
          <path d="M512 192c-176.64 0-320 143.36-320 320 0 56.32 14.72 109.44 42.88 156.16L192 832l169.6-44.16A318.72 318.72 0 0 0 512 832c176.64 0 320-143.36 320-320S688.64 192 512 192z" fill="#25D366" fill-opacity="0.1"/>
          <path d="M682.88 595.84c-9.6 24.32-56.32 53.12-78.08 56.96-19.84 3.52-46.4 5.44-76.8-3.52-17.6-5.12-40.32-13.12-69.12-36.48-25.6-20.8-42.56-46.4-47.68-54.4-9.6-14.72-77.44-103.36-77.44-197.44 0-41.6 21.76-74.56 60.48-74.56 17.28 0 35.52 6.4 52.48 18.88s27.52 29.44 30.08 47.36 3.2 37.44-2.24 53.76c-1.92 5.76-9.6 13.44-14.72 20.48-5.12 7.04-10.56 14.72-5.12 28.8 5.44 14.4 24.32 40.32 52.48 65.28 35.84 31.68 65.92 41.6 75.52 45.44 9.6 3.84 15.36 3.2 21.12-1.92 5.76-5.12 24.32-28.16 30.72-37.76 6.4-9.6 12.8-8 21.76-4.8s56.96 26.88 66.56 31.68c9.6 4.8 16 7.04 18.56 11.2 2.56 4.16 2.56 24.32-7.04 48.64z" fill="white"/>
        </svg>
      );
    case 'facebook':
      return (
        <svg width="32" height="32" viewBox="0 0 1024 1024" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect width="1024" height="1024" rx="128" fill="#1877F2"/>
          <path d="M634.88 192h-89.6c-53.76 0-98.56 43.52-98.56 97.28v84.48h-76.8v107.52h76.8v279.04h115.2V481.28h96l15.36-107.52h-111.36v-71.68c0-28.16 23.04-51.2 51.2-51.2h60.16V192z" fill="white"/>
        </svg>
      );
    case 'telegram':
      return (
        <svg width="32" height="32" viewBox="0 0 1024 1024" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect width="1024" height="1024" rx="128" fill="#2CA5E0"/>
          <path d="M512 192c-176.64 0-320 143.36-320 320s143.36 320 320 320 320-143.36 320-320-143.36-320-320-320z m166.4 441.6-44.8-211.2 28.8-27.52c6.4-6.4-1.3-9.6-9.6-5.8l-358.4 225.92-138.24-42.88c-14.72-4.48-15.04-14.4 3.2-21.12l271.36-104.96 124.16-116.48c13.44-12.8 24.32-5.76 15.04 9.6z" fill="white"/>
        </svg>
      );
    case 'imessage':
      return (
        <svg width="32" height="32" viewBox="0 0 1024 1024" fill="none" xmlns="http://www.w3.org/2000/svg">
          <rect width="1024" height="1024" rx="128" fill="#007AFF"/>
          <path d="M512 192c-176.64 0-320 143.36-320 320s143.36 320 320 320c32.64 0 64-4.8 93.44-14.08l109.44 57.6-49.92-95.36C741.12 753.28 768 695.68 768 633.6c0-176.64-143.36-320-320-320z" fill="white"/>
          <path d="M409.6 512a38.4 38.4 0 1 1 0 76.8 38.4 38.4 0 0 1 0-76.8zM614.4 512a38.4 38.4 0 1 1 0 76.8 38.4 38.4 0 0 1 0-76.8z" fill="#007AFF"/>
        </svg>
      );
    default:
      return null;
  }
};

interface ChannelConfig {
  enabled: boolean;
  name: string;
  icon: string;
  config: {
    [key: string]: any;
  };
}

interface ChannelsData {
  enabled_channels: string[];
  channels: {
    [key: string]: ChannelConfig;
  };
}

export default function Channels() {
  const [channels, setChannels] = useState<ChannelsData | null>(null);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [editingChannel, setEditingChannel] = useState<string | null>(null);
  const [configValues, setConfigValues] = useState<{ [key: string]: any }>({});

  useEffect(() => {
    fetchChannels();
  }, []);

  const fetchChannels = async () => {
    try {
      const response = await fetch('/api/channels');
      const data = await response.json();
      setChannels(data);
    } catch (error) {
      console.error('Failed to fetch channels:', error);
      setMessage('加载通道配置失败');
    }
  };

  const handleToggleChannel = async (channelId: string) => {
    if (!channels) return;

    const newState = !channels.channels[channelId].enabled;
    setLoading(true);
    try {
      const response = await fetch(`/api/channels/${channelId}/toggle`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ enabled: newState }),
      });

      if (response.ok) {
        await fetchChannels();
        setMessage(newState ? `已启用 ${channels.channels[channelId].name}` : `已禁用 ${channels.channels[channelId].name}`);
      } else {
        setMessage('操作失败');
      }
    } catch (error) {
      console.error('Failed to toggle channel:', error);
      setMessage('操作失败');
    }
    setLoading(false);
  };

  const handleStartChannel = async (channelId: string) => {
    setLoading(true);
    try {
      const response = await fetch(`/api/channels/${channelId}/start`, {
        method: 'POST',
      });

      if (response.ok) {
        await fetchChannels();
        setMessage(`${channels?.channels[channelId].name} 已启动`);
      } else {
        setMessage('启动失败');
      }
    } catch (error) {
      console.error('Failed to start channel:', error);
      setMessage('启动失败');
    }
    setLoading(false);
  };

  const handleStopChannel = async (channelId: string) => {
    setLoading(true);
    try {
      const response = await fetch(`/api/channels/${channelId}/stop`, {
        method: 'POST',
      });

      if (response.ok) {
        await fetchChannels();
        setMessage(`${channels?.channels[channelId].name} 已停止`);
      } else {
        setMessage('停止失败');
      }
    } catch (error) {
      console.error('Failed to stop channel:', error);
      setMessage('停止失败');
    }
    setLoading(false);
  };

  const handleEditConfig = (channelId: string) => {
    setEditingChannel(channelId);
    setConfigValues({ ...channels?.channels[channelId].config });
  };

  const handleSaveConfig = async (channelId: string) => {
    setLoading(true);
    try {
      const response = await fetch(`/api/channels/${channelId}/config`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(configValues),
      });

      if (response.ok) {
        await fetchChannels();
        setMessage('配置保存成功');
        setEditingChannel(null);
      } else {
        setMessage('配置保存失败');
      }
    } catch (error) {
      console.error('Failed to save config:', error);
      setMessage('配置保存失败');
    }
    setLoading(false);
  };

  const handleCancelEdit = () => {
    setEditingChannel(null);
    setConfigValues({});
  };

  const renderConfigFields = (channelId: string) => {
    if (!channels || editingChannel !== channelId) return null;

    const config = channels.channels[channelId].config;
    const fields: Array<{ key: string; label: string; type: 'text' | 'password' | 'number' | 'select'; options?: Array<{ label: string; value: string }> }> = [];

    if (channelId === 'feishu') {
      fields.push(
        { key: 'app_id', label: 'App ID', type: 'text' },
        { key: 'app_secret', label: 'App Secret', type: 'password' },
        { key: 'encrypt_key', label: 'Encrypt Key (可选)', type: 'password' },
        { key: 'verification_token', label: 'Verification Token (可选)', type: 'text' },
        { key: 'port', label: '端口', type: 'number' },
        { key: 'path', label: 'Webhook路径', type: 'text' },
      );
    } else if (channelId === 'wechat') {
      fields.push(
        { key: 'token', label: 'Token', type: 'text' },
        { key: 'app_id', label: 'App ID', type: 'text' },
        { key: 'app_secret', label: 'App Secret', type: 'password' },
        { key: 'encoding_aes_key', label: 'Encoding AES Key (可选)', type: 'password' },
        { key: 'port', label: '端口', type: 'number' },
        { key: 'path', label: 'Webhook路径', type: 'text' },
        { key: 'type', label: '类型', type: 'text' },
      );
    } else if (channelId === 'qq') {
      fields.push(
        { key: 'app_id', label: 'App ID', type: 'text' },
        { key: 'app_secret', label: 'App Secret', type: 'password' },
        { key: 'token', label: 'Access Token', type: 'password' },
        { key: 'port', label: '端口', type: 'number' },
        { key: 'path', label: 'Webhook路径', type: 'text' },
        { key: 'sandbox', label: '沙箱环境', type: 'text' },
      );
    } else if (channelId === 'dingtalk') {
      fields.push(
        { key: 'app_key', label: 'App Key', type: 'text' },
        { key: 'app_secret', label: 'App Secret', type: 'password' },
        { key: 'agent_id', label: 'Agent ID', type: 'text' },
        { key: 'encrypt_key', label: 'Encrypt Key (可选)', type: 'password' },
        { key: 'webhook_secret', label: 'Webhook Secret (可选)', type: 'password' },
        { key: 'port', label: '端口', type: 'number' },
        { key: 'path', label: 'Webhook路径', type: 'text' },
      );
    } else if (channelId === 'whatsapp') {
      fields.push(
        { key: 'phone_number_id', label: 'Phone Number ID', type: 'text' },
        { key: 'business_id', label: 'Business ID', type: 'text' },
        { key: 'access_token', label: 'Access Token', type: 'password' },
        { key: 'verify_token', label: 'Verify Token', type: 'text' },
        { key: 'port', label: '端口', type: 'number' },
        { key: 'path', label: 'Webhook路径', type: 'text' },
      );
    } else if (channelId === 'facebook') {
      fields.push(
        { key: 'page_id', label: 'Page ID', type: 'text' },
        { key: 'page_access_token', label: 'Page Access Token', type: 'password' },
        { key: 'app_secret', label: 'App Secret', type: 'password' },
        { key: 'verify_token', label: 'Verify Token', type: 'text' },
        { key: 'port', label: '端口', type: 'number' },
        { key: 'path', label: 'Webhook路径', type: 'text' },
      );
    } else if (channelId === 'telegram') {
      fields.push(
        { key: 'bot_token', label: 'Bot Token', type: 'password' },
        { key: 'webhook_url', label: 'Webhook URL (可选)', type: 'text' },
        { key: 'secret_token', label: 'Secret Token (可选)', type: 'text' },
        { key: 'use_webhook', label: '使用 Webhook', type: 'text' },
        { key: 'port', label: '端口', type: 'number' },
        { key: 'path', label: 'Webhook路径', type: 'text' },
      );
    } else if (channelId === 'imessage') {
      fields.push(
        { key: 'imsg_path', label: 'imsg 路径', type: 'text' },
        { 
          key: 'region', 
          label: '地区', 
          type: 'select',
          options: [
            { label: '中国 (CN)', value: 'CN' },
            { label: '美国 (US)', value: 'US' },
            { label: '英国 (GB)', value: 'GB' },
            { label: '日本 (JP)', value: 'JP' },
            { label: '韩国 (KR)', value: 'KR' },
            { label: '新加坡 (SG)', value: 'SG' },
            { label: '澳大利亚 (AU)', value: 'AU' },
            { label: '加拿大 (CA)', value: 'CA' },
          ]
        },
        { key: 'debounce', label: '防抖时间（如 250ms）', type: 'text' },
        { key: 'watch_since', label: '从指定消息ID开始监听', type: 'number' },
      );
    }

    return (
      <div className="config-form">
        {fields.map((field) => (
          <div key={field.key} className="config-field">
            <label>{field.label}</label>
            {field.type === 'select' ? (
              <select
                value={configValues[field.key] || ''}
                onChange={(e) =>
                  setConfigValues({
                    ...configValues,
                    [field.key]: e.target.value,
                  })
                }
              >
                {field.options?.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            ) : (
              <input
                type={field.type}
                value={configValues[field.key] || ''}
                onChange={(e) =>
                  setConfigValues({
                    ...configValues,
                    [field.key]: field.type === 'number' ? parseInt(e.target.value) :
                                field.key === 'sandbox' ? e.target.value === 'true' :
                                e.target.value,
                  })
                }
                placeholder={config[field.key] ? '••••••••' : ''}
              />
            )}
          </div>
        ))}
        <div className="config-actions">
          <button className="cancel-btn" onClick={handleCancelEdit}>
            取消
          </button>
          <button className="save-btn" onClick={() => handleSaveConfig(channelId)} disabled={loading}>
            <SaveIcon size={16} />
            {loading ? '保存中...' : '保存配置'}
          </button>
        </div>
      </div>
    );
  };

  if (!channels) {
    return <div className="channels-container">加载中...</div>;
  }

  return (
    <div className="channels-container">
      <div className="channels-header">
        <h1>通道管理</h1>
        <div className="header-actions">
          <button className="action-btn secondary" onClick={fetchChannels}>
            <RefreshIcon size={16} />
            刷新
          </button>
        </div>
      </div>

      {message && (
        <div className={`message ${message.includes('成功') || message.includes('已启用') || message.includes('已启动') ? 'success' : 'error'}`}>
          {message}
        </div>
      )}

      <div className="channels-list">
        {Object.entries(channels.channels).map(([channelId, channel]) => (
          <div key={channelId} className={`channel-card ${channel.enabled ? 'enabled' : 'disabled'}`}>
            <div className="channel-header">
              <div className="channel-info">
                <div className="channel-logo">{getChannelLogo(channelId)}</div>
                <h2 className="channel-name">{channel.name}</h2>
                <span className={`channel-status ${channel.enabled ? 'active' : 'inactive'}`}>
                  {channel.enabled ? '已启用' : '已禁用'}
                </span>
              </div>
              <div className="channel-actions">
                <button
                  className={`toggle-btn ${channel.enabled ? 'active' : ''}`}
                  onClick={() => handleToggleChannel(channelId)}
                  disabled={loading}
                >
                  {channel.enabled ? <CheckIcon size={16} /> : <CloseIcon size={16} />}
                  {channel.enabled ? '启用' : '禁用'}
                </button>
                {channel.enabled && (
                  <>
                    <button className="action-btn success" onClick={() => handleStartChannel(channelId)} disabled={loading}>
                      <PlayIcon size={16} />
                      启动
                    </button>
                    <button className="action-btn danger" onClick={() => handleStopChannel(channelId)} disabled={loading}>
                      <StopIcon size={16} />
                      停止
                    </button>
                  </>
                )}
                <button className="action-btn secondary" onClick={() => handleEditConfig(channelId)}>
                  配置
                </button>
              </div>
            </div>

            <div className="channel-details">
              <div className="config-summary">
                <span>端口: {channel.config.port}</span>
                <span>路径: {channel.config.path}</span>
              </div>
              {channel.config.description && <p className="channel-description">{channel.config.description}</p>}
            </div>

            {editingChannel === channelId && renderConfigFields(channelId)}
          </div>
        ))}
      </div>
    </div>
  );
}
