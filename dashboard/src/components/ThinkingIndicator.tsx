import { useState, useEffect } from 'react';
import { Brain, Loader2, CheckCircle, AlertCircle, Wrench } from 'lucide-react';
import './styles/ThinkingIndicator.css';

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

interface ThinkingIndicatorProps {
  events: ThinkingEvent[];
  isComplete: boolean;
}

export default function ThinkingIndicator({ events, isComplete }: ThinkingIndicatorProps) {
  const [contentExpanded, setContentExpanded] = useState(true);

  useEffect(() => {
    if (events.length > 0) {
      setContentExpanded(true);
    }
  }, [events]);

  if (events.length === 0) {
    return null;
  }

  const latestEvent = events[events.length - 1];
  const isThinking = !isComplete && latestEvent.type !== 'error';

  return (
    <div className="thinking-indicator">
      <div className="thinking-header">
        <div className="thinking-title">
          {isThinking ? (
            <>
              <Loader2 className="spinner" size={16} />
              <span>AI 正在思考</span>
            </>
          ) : latestEvent.type === 'error' ? (
            <>
              <AlertCircle size={16} className="error" />
              <span>思考出错</span>
            </>
          ) : (
            <>
              <CheckCircle size={16} className="success" />
              <span>思考完成</span>
            </>
          )}
        </div>
        <button
          className="toggle-btn"
          onClick={() => setContentExpanded(!contentExpanded)}
          title={contentExpanded ? '收起' : '展开'}
        >
          {contentExpanded ? '▼' : '▶'}
        </button>
      </div>

      {contentExpanded && (
        <div className="thinking-content">
          <div className="thinking-events">
            {events.map((event, index) => (
              <div key={index} className={`thinking-event event-${event.type}`}>
                <div className="event-icon">
                  {getEventIcon(event.type)}
                </div>
                <div className="event-details">
                  <div className="event-content">{event.content}</div>
                  {event.metadata?.tool_name && (
                    <div className="event-tool">
                      <Wrench size={12} />
                      <span>{event.metadata.tool_name}</span>
                    </div>
                  )}
                  {event.progress > 0 && event.progress < 100 && (
                    <div className="event-progress">
                      <div
                        className="progress-bar"
                        style={{ width: `${event.progress}%` }}
                      />
                    </div>
                  )}
                  {event.metadata?.result && (
                    <div className="event-result">
                      <span className="result-label">结果:</span>
                      <span className="result-value">
                        {typeof event.metadata.result === 'string'
                          ? event.metadata.result.substring(0, 100) +
                            (event.metadata.result.length > 100 ? '...' : '')
                          : JSON.stringify(event.metadata.result).substring(0, 100) +
                            '...'}
                      </span>
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function getEventIcon(type: string) {
  switch (type) {
    case 'start':
      return <Brain size={16} className="icon-start" />;
    case 'progress':
      return <Loader2 size={16} className="icon-progress spinner" />;
    case 'tool_call':
      return <Wrench size={16} className="icon-tool" />;
    case 'tool_result':
      return <CheckCircle size={16} className="icon-result" />;
    case 'complete':
      return <CheckCircle size={16} className="icon-complete" />;
    case 'error':
      return <AlertCircle size={16} className="icon-error" />;
    default:
      return <Brain size={16} className="icon-default" />;
  }
}
