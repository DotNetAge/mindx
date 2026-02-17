import { useState, useEffect, useRef } from 'react';
import './Monitor.css';

interface LogEntry {
  timestamp: string;
  level: string;
  message: string;
  logger?: string;
  caller?: string;
  extra?: Record<string, any>;
}

interface LogsResponse {
  logs: LogEntry[];
  lastTimestamp: string;
  count: number;
}

type LogLevel = 'debug' | 'info' | 'warn' | 'error' | 'fatal';

export default function Monitor() {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [levelFilter, setLevelFilter] = useState<LogLevel | ''>('');
  const [autoScroll, setAutoScroll] = useState(true);
  const [lastTimestamp, setLastTimestamp] = useState('');
  const [isPolling, setIsPolling] = useState(false);
  const [isFirstLoad, setIsFirstLoad] = useState(true);

  const logsEndRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  // 首次加载完整日志
  useEffect(() => {
    fetchLogs();
  }, []);

  // 增量轮询（仅在有新数据时更新）
  useEffect(() => {
    if (isFirstLoad) return; // 首次加载后才开始轮询

    const interval = setInterval(() => {
      pollNewLogs();
    }, 2000);
    return () => clearInterval(interval);
  }, [lastTimestamp, levelFilter]);

  useEffect(() => {
    if (autoScroll && logsEndRef.current) {
      logsEndRef.current.scrollIntoView({ behavior: 'auto' });
    }
  }, [logs, autoScroll]);

  const fetchLogs = async () => {
    try {
      const params = new URLSearchParams();
      if (levelFilter) {
        params.append('level', levelFilter);
      }
      const response = await fetch(`/api/monitor?${params}`);
      const data: LogsResponse = await response.json();

      const newLogs = data.logs || [];
      setLogs(newLogs);
      setLastTimestamp(data.lastTimestamp || '');
      setIsFirstLoad(false);
    } catch (error) {
      console.error('Failed to fetch logs:', error);
    } finally {
      setLoading(false);
    }
  };

  const pollNewLogs = async () => {
    if (isPolling || !lastTimestamp) return;

    try {
      setIsPolling(true);
      const params = new URLSearchParams();
      if (levelFilter) {
        params.append('level', levelFilter);
      }
      params.append('since', lastTimestamp);
      const response = await fetch(`/api/monitor?${params}`);
      const data: LogsResponse = await response.json();

      const newLogs = data.logs || [];
      if (newLogs.length > 0) {
        setLogs(prev => [...prev, ...newLogs]);
        setLastTimestamp(data.lastTimestamp || '');
      }
    } catch (error) {
      console.error('Failed to poll logs:', error);
    } finally {
      setIsPolling(false);
    }
  };

  const handleScroll = () => {
    if (containerRef.current) {
      const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
      const isNearBottom = scrollHeight - scrollTop - clientHeight < 100;
      setAutoScroll(isNearBottom);
    }
  };

  const scrollToBottom = () => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    setAutoScroll(true);
  };

  const clearLogs = async () => {
    if (!window.confirm('确定要清空所有系统日志吗？此操作不可恢复。')) {
      return;
    }

    try {
      const response = await fetch('/api/monitor', {
        method: 'DELETE',
      });
      const data = await response.json();

      if (data.success) {
        setLogs([]);
        setLastTimestamp('');
        alert('日志已清空');
      } else {
        alert('清空日志失败: ' + (data.error || '未知错误'));
      }
    } catch (error) {
      console.error('Failed to clear logs:', error);
      alert('清空日志失败');
    }
  };

  const filteredLogs = logs.filter(log => {
    const matchesLevel = !levelFilter || log.level.toLowerCase() === levelFilter.toLowerCase();
    const matchesText = filter === '' ||
      log.message.toLowerCase().includes(filter.toLowerCase()) ||
      log.logger?.toLowerCase().includes(filter.toLowerCase()) ||
      log.caller?.toLowerCase().includes(filter.toLowerCase());
    return matchesLevel && matchesText;
  });

  // 清理旧日志，防止内存溢出（保留最近2000条）
  useEffect(() => {
    if (logs.length > 2000) {
      setLogs(prev => prev.slice(prev.length - 2000));
    }
  }, [logs.length]);

  const formatTimestamp = (timestamp: string) => {
    try {
      const date = new Date(timestamp);
      return date.toLocaleTimeString('zh-CN', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
      }) + '.' + date.getMilliseconds().toString().padStart(3, '0');
    } catch {
      return timestamp;
    }
  };

  const formatMessage = (entry: LogEntry) => {
    let msg = entry.message;
    if (entry.extra && Object.keys(entry.extra).length > 0) {
      const extraStr = Object.entries(entry.extra)
        .map(([k, v]) => `${k}=${JSON.stringify(v)}`)
        .join(' ');
      msg += ` ${extraStr}`;
    }
    return msg;
  };

  const getLevelSymbol = (level: string): string => {
    switch (level.toUpperCase()) {
      case 'ERROR': return '✗';
      case 'WARN': return '⚠';
      case 'FATAL': return '☠';
      case 'DEBUG': return '○';
      case 'INFO':
      default: return '●';
    }
  };

  return (
    <div className="terminal-container">
      <div className="terminal-header">
        <div className="terminal-controls">
          <div className="terminal-buttons">
            <span className="terminal-btn close"></span>
            <span className="terminal-btn minimize"></span>
            <span className="terminal-btn maximize"></span>
          </div>
          <div className="terminal-title">Bot Monitor - Log Terminal</div>
        </div>
        <div className="terminal-actions">
          {!autoScroll && (
            <button className="terminal-action-btn" onClick={scrollToBottom} title="滚动到底部">
              ↓ 跳到底部
            </button>
          )}
          <button className="terminal-action-btn" onClick={fetchLogs} title="立即刷新">
            ↻ 刷新
          </button>
          <button className="terminal-action-btn terminal-action-btn-danger" onClick={clearLogs} title="清空日志">
            🗑 清空
          </button>
          <select
            className="terminal-select"
            value={levelFilter}
            onChange={(e) => setLevelFilter(e.target.value as LogLevel | '')}
            title="日志级别过滤"
          >
            <option value="">ALL</option>
            <option value="debug">DEBUG</option>
            <option value="info">INFO</option>
            <option value="warn">WARN</option>
            <option value="error">ERROR</option>
            <option value="fatal">FATAL</option>
          </select>
          <input
            type="text"
            className="terminal-input"
            placeholder="Search logs..."
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
        </div>
      </div>

      <div className="terminal-body" ref={containerRef} onScroll={handleScroll}>
        <div className="terminal-content">
          {loading && logs.length === 0 ? (
            <div className="terminal-line">
              <span className="terminal-timestamp">--:--:--.---</span>
              <span className="terminal-level INFO">● INFO</span>
              <span className="terminal-message">Loading logs...</span>
            </div>
          ) : filteredLogs.length === 0 ? (
            <div className="terminal-line">
              <span className="terminal-timestamp">--:--:--.---</span>
              <span className="terminal-level INFO">● INFO</span>
              <span className="terminal-message">
                {filter || levelFilter ? 'No matching logs found' : 'No logs available'}
              </span>
            </div>
          ) : (
            filteredLogs.map((log, index) => (
              <div key={index} className={`terminal-line ${log.level.toLowerCase()}`}>
                <span className="terminal-timestamp">{formatTimestamp(log.timestamp)}</span>
                <span className={`terminal-level ${log.level.toUpperCase()}`}>
                  {getLevelSymbol(log.level)} {log.level.toUpperCase().padEnd(5)}
                </span>
                {log.logger && <span className="terminal-logger">[{log.logger}]</span>}
                {log.caller && <span className="terminal-caller">{log.caller}</span>}
                <span className="terminal-message">{formatMessage(log)}</span>
              </div>
            ))
          )}
          <div ref={logsEndRef} />
        </div>
      </div>
      
      <div className="terminal-footer">
        <span className="terminal-status">
          {autoScroll ? '🔄 Auto-scroll ON' : '⏸ Auto-scroll OFF'}
        </span>
        <span className="terminal-stats">
          {filteredLogs.length} / {logs.length} logs
        </span>
      </div>
    </div>
  );
}
