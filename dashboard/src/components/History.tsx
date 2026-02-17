import { useState, useEffect } from 'react';
import { SearchIcon, DeleteIcon, RefreshIcon, PlayIcon } from 'tdesign-icons-react';
import './styles/History.css';
import { useSession } from '../contexts/SessionContext';

interface Conversation {
  id: string;
  title: string;
  timestamp: number;
  messageCount: number;
  start_time?: string;
  end_time?: string;
}

interface HistoryProps {
  onSwitchSession?: () => void;
}

export default function History({ onSwitchSession }: HistoryProps) {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [switching, setSwitching] = useState<string | null>(null);
  
  const { switchSession, currentSession } = useSession();

  useEffect(() => {
    fetchConversations();
  }, []);

  const fetchConversations = async () => {
    try {
      const response = await fetch('/api/conversations?limit=100');
      const data = await response.json();
      setConversations(Array.isArray(data) ? data : []);
      setLoading(false);
    } catch (error) {
      console.error('Failed to fetch conversations:', error);
      setLoading(false);
    }
  };

  const filteredConversations = conversations.filter(conv =>
    (conv.title || '').toLowerCase().includes(searchQuery.toLowerCase())
  );

  const handleDelete = async (id: string) => {
    if (!window.confirm('确定要删除这个对话吗？')) {
      return;
    }
    
    try {
      const response = await fetch(`/api/conversations/${id}`, {
        method: 'DELETE',
      });
      
      if (response.ok) {
        setConversations((prev) => prev.filter((conv) => conv.id !== id));
        if (selectedId === id) {
          setSelectedId(null);
        }
      } else {
        console.error('Failed to delete conversation');
      }
    } catch (error) {
      console.error('Error deleting conversation:', error);
    }
  };

  const handleSwitch = async (id: string) => {
    setSwitching(id);
    setSelectedId(id);
    
    try {
      await switchSession(id);
      if (onSwitchSession) {
        onSwitchSession();
      }
    } catch (error) {
      console.error('Error switching conversation:', error);
    } finally {
      setSwitching(null);
    }
  };

  const formatDate = (timestamp: number): string => {
    const date = new Date(timestamp * 1000);
    const now = new Date();
    const diff = now.getTime() - date.getTime();
    const days = Math.floor(diff / 86400000);

    if (days === 0) return '今天';
    if (days === 1) return '昨天';
    if (days < 7) return `${days}天前`;
    return date.toLocaleDateString('zh-CN');
  };

  return (
    <div className="history-container">
      <div className="history-header">
        <h1>历史对话</h1>
        <div className="header-actions">
          <button className="action-btn" onClick={fetchConversations} aria-label="刷新">
            <RefreshIcon size={16} />
            刷新
          </button>
        </div>
      </div>

      <div className="search-bar">
        <SearchIcon size={18} />
        <input
          type="text"
          placeholder="搜索对话..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="search-input"
        />
      </div>

      {loading ? (
        <div className="loading">加载中...</div>
      ) : (
        <div className="conversation-list">
          {filteredConversations.length === 0 ? (
            <div className="empty-state">
              {searchQuery ? '没有找到匹配的对话' : '暂无对话'}
            </div>
          ) : (
            filteredConversations.map((conv) => (
              <div
                key={conv.id}
                className={`conversation-item ${selectedId === conv.id ? 'selected' : ''} ${currentSession?.id === conv.id ? 'current' : ''}`}
              >
                <div className="conversation-main" onClick={() => setSelectedId(conv.id)}>
                  <h3 className="conversation-title">{conv.title}</h3>
                  <p className="conversation-meta">
                    {formatDate(conv.timestamp)} · {conv.messageCount} 条消息
                    {currentSession?.id === conv.id && ' · 当前会话'}
                  </p>
                </div>
                <div className="conversation-actions">
                  {currentSession?.id !== conv.id && (
                    <button
                      className="switch-btn"
                      aria-label="切换到此对话"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleSwitch(conv.id);
                      }}
                      disabled={switching === conv.id}
                      title="切换到此对话"
                    >
                      <PlayIcon size={16} />
                    </button>
                  )}
                  <button
                    className="delete-btn"
                    aria-label="删除对话"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleDelete(conv.id);
                    }}
                  >
                    <DeleteIcon size={16} />
                  </button>
                </div>
              </div>
            ))
          )}
        </div>
      )}
    </div>
  );
}
