import { SaveIcon, RefreshIcon } from 'tdesign-icons-react';

interface ConfigField {
  key: string;
  label: string;
  type: 'text' | 'password' | 'number' | 'select';
  options?: Array<{ label: string; value: string }>;
}

interface ChannelConfigFormProps {
  channelId: string;
  configValues: Record<string, unknown>;
  loading: boolean;
  onConfigChange: (key: string, value: unknown) => void;
  onSave: (channelId: string) => void;
  onCancel: () => void;
}

function getFieldsForChannel(channelId: string): ConfigField[] {
  switch (channelId) {
    case 'feishu':
      return [
        { key: 'app_id', label: 'App ID', type: 'text' },
        { key: 'app_secret', label: 'App Secret', type: 'password' },
        { key: 'encrypt_key', label: 'Encrypt Key (可选)', type: 'password' },
        { key: 'verification_token', label: 'Verification Token (可选)', type: 'text' },
        { key: 'port', label: '端口', type: 'number' },
        { key: 'path', label: 'Webhook路径', type: 'text' },
      ];
    case 'wechat':
      return [
        { key: 'token', label: 'Token', type: 'text' },
        { key: 'app_id', label: 'App ID', type: 'text' },
        { key: 'app_secret', label: 'App Secret', type: 'password' },
        { key: 'encoding_aes_key', label: 'Encoding AES Key (可选)', type: 'password' },
        { key: 'port', label: '端口', type: 'number' },
        { key: 'path', label: 'Webhook路径', type: 'text' },
        { key: 'type', label: '类型', type: 'text' },
      ];
    case 'qq':
      return [
        { key: 'app_id', label: 'App ID', type: 'text' },
        { key: 'app_secret', label: 'App Secret', type: 'password' },
        { key: 'token', label: 'Access Token', type: 'password' },
        { key: 'port', label: '端口', type: 'number' },
        { key: 'path', label: 'Webhook路径', type: 'text' },
        { key: 'sandbox', label: '沙箱环境', type: 'text' },
      ];
    case 'dingtalk':
      return [
        { key: 'app_key', label: 'App Key', type: 'text' },
        { key: 'app_secret', label: 'App Secret', type: 'password' },
        { key: 'agent_id', label: 'Agent ID', type: 'text' },
        { key: 'encrypt_key', label: 'Encrypt Key (可选)', type: 'password' },
        { key: 'webhook_secret', label: 'Webhook Secret (可选)', type: 'password' },
        { key: 'port', label: '端口', type: 'number' },
        { key: 'path', label: 'Webhook路径', type: 'text' },
      ];
    case 'whatsapp':
      return [
        { key: 'phone_number_id', label: 'Phone Number ID', type: 'text' },
        { key: 'business_id', label: 'Business ID', type: 'text' },
        { key: 'access_token', label: 'Access Token', type: 'password' },
        { key: 'verify_token', label: 'Verify Token', type: 'text' },
        { key: 'port', label: '端口', type: 'number' },
        { key: 'path', label: 'Webhook路径', type: 'text' },
      ];
    case 'facebook':
      return [
        { key: 'page_id', label: 'Page ID', type: 'text' },
        { key: 'page_access_token', label: 'Page Access Token', type: 'password' },
        { key: 'app_secret', label: 'App Secret', type: 'password' },
        { key: 'verify_token', label: 'Verify Token', type: 'text' },
        { key: 'port', label: '端口', type: 'number' },
        { key: 'path', label: 'Webhook路径', type: 'text' },
      ];
    case 'telegram':
      return [
        { key: 'bot_token', label: 'Bot Token', type: 'password' },
        { key: 'webhook_url', label: 'Webhook URL (可选)', type: 'text' },
        { key: 'secret_token', label: 'Secret Token (可选)', type: 'text' },
        { key: 'use_webhook', label: '使用 Webhook', type: 'text' },
        { key: 'port', label: '端口', type: 'number' },
        { key: 'path', label: 'Webhook路径', type: 'text' },
      ];
    case 'imessage':
      return [
        { key: 'imsg_path', label: 'imsg 路径', type: 'text' },
        {
          key: 'region', label: '地区', type: 'select',
          options: [
            { label: '中国 (CN)', value: 'CN' },
            { label: '美国 (US)', value: 'US' },
            { label: '英国 (GB)', value: 'GB' },
            { label: '日本 (JP)', value: 'JP' },
            { label: '韩国 (KR)', value: 'KR' },
            { label: '新加坡 (SG)', value: 'SG' },
            { label: '澳大利亚 (AU)', value: 'AU' },
            { label: '加拿大 (CA)', value: 'CA' },
          ],
        },
        { key: 'debounce', label: '防抖时间（如 250ms）', type: 'text' },
        { key: 'watch_since', label: '从指定消息ID开始监听', type: 'number' },
      ];
    default:
      return [];
  }
}
export default function ChannelConfigForm({
  channelId, configValues, loading, onConfigChange, onSave, onCancel,
}: ChannelConfigFormProps) {
  const fields = getFieldsForChannel(channelId);

  return (
    <div className="config-form">
      {fields.map((field) => (
        <div key={field.key} className="config-field">
          <label>{field.label}</label>
          {field.type === 'select' ? (
            <select
              value={(configValues[field.key] as string) || ''}
              onChange={(e) => onConfigChange(field.key, e.target.value)}
            >
              <option value="">请选择</option>
              {field.options?.map((opt) => (
                <option key={opt.value} value={opt.value}>{opt.label}</option>
              ))}
            </select>
          ) : (
            <input
              type={field.type}
              value={(configValues[field.key] as string) ?? ''}
              onChange={(e) => onConfigChange(field.key, field.type === 'number' ? Number(e.target.value) : e.target.value)}
              placeholder={field.label}
            />
          )}
        </div>
      ))}
      <div className="config-actions">
        <button className="action-btn primary" onClick={() => onSave(channelId)} disabled={loading}>
          <SaveIcon size={16} /> 保存
        </button>
        <button className="action-btn secondary" onClick={onCancel} disabled={loading}>
          取消
        </button>
        <button className="action-btn secondary" onClick={onCancel}>
          <RefreshIcon size={16} /> 重置
        </button>
      </div>
    </div>
  );
}
