import { useState, useEffect } from 'react';
import './GeneralSettings.css';

interface GeneralConfig {
  workplace: string;
  server: {
    address: string;
    port: number;
  };
}

export default function GeneralSettings() {
  const [config, setConfig] = useState<GeneralConfig>({
    workplace: 'data',
    server: {
      address: '0.0.0.0',
      port: 1314,
    },
  });
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');

  useEffect(() => {
    fetchConfig();
  }, []);

  const fetchConfig = async () => {
    try {
      const response = await fetch('/api/config/general');
      const data = await response.json();
      setConfig(data);
    } catch (error) {
      console.error('Failed to fetch config:', error);
    }
  };

  const handleSave = async () => {
    setLoading(true);
    setMessage('');
    try {
      const response = await fetch('/api/config/general', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(config),
      });

      if (response.ok) {
        setMessage('配置保存成功！');
      } else {
        setMessage('配置保存失败');
      }
    } catch (error) {
      console.error('Failed to save config:', error);
      setMessage('配置保存失败');
    }
    setLoading(false);
  };

  return (
    <div className="general-settings">
      <h2>通用配置</h2>

      <div className="config-section">
        <h3>工作区设置</h3>
        <div className="config-item">
          <label>工作区路径</label>
          <input
            type="text"
            value={config.workplace}
            onChange={(e) => setConfig({ ...config, workplace: e.target.value })}
            placeholder="数据库存储路径"
          />
        </div>
      </div>

      <div className="config-section">
        <h3>服务器设置</h3>
        <div className="config-item">
          <label>服务器地址</label>
          <input
            type="text"
            value={config.server.address}
            onChange={(e) => setConfig({ ...config, server: { ...config.server, address: e.target.value } })}
            placeholder="0.0.0.0"
          />
        </div>
        <div className="config-item">
          <label>服务器端口</label>
          <input
            type="number"
            value={config.server.port}
            onChange={(e) => setConfig({ ...config, server: { ...config.server, port: parseInt(e.target.value) } })}
            placeholder="1314"
          />
        </div>
      </div>

      <div className="config-actions">
        <button className="save-button" onClick={handleSave} disabled={loading}>
          {loading ? '保存中...' : '保存配置'}
        </button>
      </div>

      {message && <div className={`message ${message.includes('成功') ? 'success' : 'error'}`}>{message}</div>}
    </div>
  );
}
