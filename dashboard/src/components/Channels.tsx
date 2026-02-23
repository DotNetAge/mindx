import { useState, useEffect } from "react";
import "./styles/Channels.css";
import { ChannelsData } from "./channels/types";
import ChannelCard from "./channels/ChannelCard";
import ChannelConfigForm from "./channels/ChannelConfigForm";

export default function Channels() {
  const [channels, setChannels] = useState<ChannelsData | null>(null);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");
  const [editingChannel, setEditingChannel] = useState<string | null>(null);
  const [configValues, setConfigValues] = useState<Record<string, unknown>>({});

  useEffect(() => {
    fetchChannels();
  }, []);

  const fetchChannels = async () => {
    try {
      const response = await fetch("/api/channels");
      if (!response.ok)
        throw new Error(`HTTP error! status: ${response.status}`);
      const data = await response.json();
      if (data && data.channels) {
        setChannels(data);
      } else {
        setChannels({ enabled_channels: [], channels: {} });
        setMessage("通道数据格式错误");
      }
    } catch {
      setChannels({ enabled_channels: [], channels: {} });
      setMessage("加载通道配置失败");
    }
  };

  const handleToggleChannel = async (channelId: string) => {
    if (!channels) return;
    const newState = !channels.channels[channelId].enabled;
    setLoading(true);
    try {
      const response = await fetch(`/api/channels/${channelId}/toggle`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled: newState }),
      });
      if (response.ok) {
        await fetchChannels();
        setMessage(
          newState
            ? `已启用 ${channels.channels[channelId].name}`
            : `已禁用 ${channels.channels[channelId].name}`,
        );
      } else {
        setMessage("操作失败");
      }
    } catch {
      setMessage("操作失败");
    }
    setLoading(false);
  };

  const handleStartChannel = async (channelId: string) => {
    setLoading(true);
    try {
      const response = await fetch(`/api/channels/${channelId}/start`, {
        method: "POST",
      });
      if (response.ok) {
        await fetchChannels();
        setMessage(`${channels?.channels[channelId].name} 已启动`);
      } else {
        setMessage("启动失败");
      }
    } catch {
      setMessage("启动失败");
    }
    setLoading(false);
  };

  const handleStopChannel = async (channelId: string) => {
    setLoading(true);
    try {
      const response = await fetch(`/api/channels/${channelId}/stop`, {
        method: "POST",
      });
      if (response.ok) {
        await fetchChannels();
        setMessage(`${channels?.channels[channelId].name} 已停止`);
      } else {
        setMessage("停止失败");
      }
    } catch {
      setMessage("停止失败");
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
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(configValues),
      });
      if (response.ok) {
        await fetchChannels();
        setMessage("配置保存成功");
        setEditingChannel(null);
      } else {
        setMessage("配置保存失败");
      }
    } catch {
      setMessage("配置保存失败");
    }
    setLoading(false);
  };

  const handleCancelEdit = () => {
    setEditingChannel(null);
    setConfigValues({});
  };
  if (!channels) {
    return (
      <div className="channels-container">
        <div className="loading">加载中...</div>
      </div>
    );
  }

  return (
    <div className="channels-container">
      <div className="channels-header">
        <h1>渠道管理</h1>
      </div>

      {message && (
        <div
          className={`message ${message.includes("成功") || message.includes("已启用") || message.includes("已禁用") || message.includes("已启动") || message.includes("已停止") ? "success" : "error"}`}
        >
          {message}
          <button onClick={() => setMessage("")}>×</button>
        </div>
      )}

      <div className="channels-list">
        {Object.entries(channels.channels).map(([channelId, channel]) => (
          <ChannelCard
            key={channelId}
            channelId={channelId}
            channel={channel}
            loading={loading}
            onToggle={handleToggleChannel}
            onStart={handleStartChannel}
            onStop={handleStopChannel}
            onEdit={handleEditConfig}
            configForm={
              editingChannel === channelId ? (
                <ChannelConfigForm
                  channelId={channelId}
                  configValues={configValues}
                  loading={loading}
                  onConfigChange={(key, value) =>
                    setConfigValues({ ...configValues, [key]: value })
                  }
                  onSave={handleSaveConfig}
                  onCancel={handleCancelEdit}
                />
              ) : null
            }
          />
        ))}
      </div>
    </div>
  );
}
