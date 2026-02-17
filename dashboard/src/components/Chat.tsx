import { useState, useRef, useEffect, useLayoutEffect, useCallback } from 'react';
import { AddIcon, ChevronDownIcon } from 'tdesign-icons-react';
import MessageList from './MessageList';
import MessageInput from './MessageInput';
import './styles/Chat.css';
import { useTranslation } from '../i18n';
import { useSession, Message } from '../contexts/SessionContext';

interface Conversation {
  id: string;
  title: string;
  timestamp: number;
  messageCount: number;
}

interface ThinkingEvent {
  type: 'start' | 'progress' | 'chunk' | 'tool_call' | 'tool_result' | 'complete' | 'error';
  content: string;
  progress: number;
  timestamp: number;
  metadata?: {
    tool_name?: string;
    arguments?: Record<string, unknown>;
    result?: string;
  };
}

interface WSMessage {
  type: 'connected' | 'message' | 'thinking' | 'pong' | 'error';
  content?: string;
  sessionID?: string;
  timestamp?: number;
  event?: ThinkingEvent;
}

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
  enabled: boolean;
}

export default function Chat() {
  const [isLoading, setIsLoading] = useState(false);
  const [isConnected, setIsConnected] = useState(false);
  const [connectionError, setConnectionError] = useState<string | null>(null);
  const [thinkingEvents, setThinkingEvents] = useState<ThinkingEvent[]>([]);
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [showDropdown, setShowDropdown] = useState(false);
  const [capabilities, setCapabilities] = useState<Capability[]>([]);
  const [selectedCapability, setSelectedCapability] = useState<Capability | null>(null);

  const wsRef = useRef<WebSocket | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);
  const { t } = useTranslation();
  const { 
    messages, 
    setMessages, 
    addMessage, 
    currentSession, 
    createNewSession, 
    loadCurrentSession,
    switchSession 
  } = useSession();

  const connectWebSocket = useCallback(() => {
    const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsHost = import.meta.env.VITE_WS_HOST || window.location.host;
    const wsUrl = `${wsProtocol}//${wsHost}/ws`;

    console.log('Connecting to WebSocket:', wsUrl);

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log('WebSocket connected');
      setIsConnected(true);
      setConnectionError(null);
      loadCurrentSession();
    };

    ws.onmessage = (event) => {
      try {
        const data: WSMessage = JSON.parse(event.data);
        console.log('WebSocket message:', data);

        switch (data.type) {
          case 'connected':
            console.log('Connected with session ID:', data.sessionID);
            break;

          case 'message': {
            const assistantMessage: Message = {
              id: Date.now().toString(),
              role: 'assistant',
              content: data.content || '',
              timestamp: data.timestamp ? data.timestamp * 1000 : Date.now(),
            };
            addMessage(assistantMessage);
            setIsLoading(false);
            break;
          }

          case 'thinking':
            if (data.event) {
              setThinkingEvents((prev) => [...prev, data.event]);
              if (data.event.type === 'complete' || data.event.type === 'error') {
                setTimeout(() => {
                  setThinkingEvents([]);
                }, 3000);
              }
            }
            break;

          case 'pong':
            break;

          case 'error':
            console.error('Server error:', data.content);
            setConnectionError(data.content || t('common.error'));
            break;

          default:
            console.log('Unknown message type:', data.type);
        }
      } catch (error) {
        console.error('Failed to parse message:', error);
      }
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
      setConnectionError(t('chat.connectionError'));
    };

    ws.onclose = () => {
      console.log('WebSocket closed');
      setIsConnected(false);
      setIsLoading(false);
    };
  }, [t, addMessage, loadCurrentSession]);

  useEffect(() => {
    connectWebSocket();

    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    fetch('/api/conversations?limit=20')
      .then(res => res.json())
      .then(data => setConversations(Array.isArray(data) ? data : []))
      .catch(console.error);
  }, [currentSession]);

  useEffect(() => {
    fetch('/api/capabilities')
      .then(res => res.json())
      .then(data => {
        if (data.capabilities) {
          setCapabilities(data.capabilities);
        }
      })
      .catch(console.error);
  }, []);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setShowDropdown(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  useEffect(() => {
    if (!isConnected && !connectionError && wsRef.current?.readyState === WebSocket.CLOSED) {
      const timer = setTimeout(() => {
        console.log('Attempting to reconnect...');
        connectWebSocket();
      }, 3000);
      return () => clearTimeout(timer);
    }
  }, [isConnected, connectionError, connectWebSocket]);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useLayoutEffect(() => {
    scrollToBottom();
  }, [messages, thinkingEvents]);

  const handleSendMessage = async (content: string) => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      setConnectionError(t('chat.notConnected'));
      return;
    }

    let finalContent = content;
    if (selectedCapability) {
      finalContent = `/${selectedCapability.name} ${content}`;
    }

    const userMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content,
      timestamp: Date.now(),
    };

    addMessage(userMessage);
    setIsLoading(true);
    setConnectionError(null);
    setThinkingEvents([]);

    try {
      wsRef.current.send(
        JSON.stringify({
          type: 'message',
          content: finalContent,
          timestamp: Math.floor(Date.now() / 1000),
        })
      );
    } catch (error) {
      console.error('Failed to send message:', error);
      setConnectionError(t('chat.sendFailed'));
      setIsLoading(false);
    }

    setSelectedCapability(null);
  };

  const handleStop = () => {
    setIsLoading(false);
  };

  const handleNewChat = async () => {
    await createNewSession();
    setMessages([]);
    setConnectionError(null);
    setThinkingEvents([]);
  };

  const handleReconnect = () => {
    if (wsRef.current) {
      wsRef.current.close();
    }
    connectWebSocket();
  };

  const handleSwitchSession = async (sessionId: string) => {
    await switchSession(sessionId);
    setShowDropdown(false);
  };

  return (
    <div className="chat-container">
      <div className="chat-header">
        <h1>{t('chat.title')}</h1>
        <div className="header-actions">
          <div className="session-selector" ref={dropdownRef}>
            <button 
              className="session-btn"
              onClick={() => setShowDropdown(!showDropdown)}
            >
              <span>{currentSession ? currentSession.id.slice(0, 8) : '新会话'}</span>
              <ChevronDownIcon size={14} />
            </button>
            {showDropdown && (
              <div className="session-dropdown">
                <div className="dropdown-item" onClick={handleNewChat}>
                  <AddIcon size={14} />
                  <span>新建会话</span>
                </div>
                {conversations.length > 0 && <div className="dropdown-divider" />}
                {conversations.map(conv => (
                  <div 
                    key={conv.id} 
                    className={`dropdown-item ${currentSession?.id === conv.id ? 'active' : ''}`}
                    onClick={() => handleSwitchSession(conv.id)}
                  >
                    <span className="conv-title">{conv.title || conv.id.slice(0, 12)}</span>
                    <span className="conv-count">{conv.messageCount}条</span>
                  </div>
                ))}
              </div>
            )}
          </div>
          <button 
            className="action-btn"
            onClick={handleNewChat}
            title={t('chat.newChat')}
          >
            <AddIcon size={16} />
            {t('chat.newChat')}
          </button>
        </div>
      </div>

      <div className="chat-messages">
        {!isConnected && !connectionError && (
          <div className="loading-history">{t('chat.connecting')}</div>
        )}

        {connectionError && (
          <div className="connection-error">
            <div className="error-message">{connectionError}</div>
            <button onClick={handleReconnect} className="reconnect-btn">{t('chat.reconnect')}</button>
          </div>
        )}

        <MessageList 
          messages={messages}
          streamingMessage=""
          isStreaming={false}
          thinkingEvents={thinkingEvents}
        />

        <div ref={messagesEndRef} />
      </div>

      <div className="chat-input-container">
        <MessageInput 
          onSend={handleSendMessage}
          isLoading={isLoading || !isConnected}
          onStop={handleStop}
          capabilities={capabilities.filter(cap => cap.enabled)}
          selectedCapability={selectedCapability}
          onSelectCapability={setSelectedCapability}
          onRemoveCapability={() => setSelectedCapability(null)}
        />
      </div>
    </div>
  );
}
