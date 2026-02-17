import { ReactNode, useMemo, useEffect, useRef, useState } from 'react';
import { UserIcon, WrenchIcon, Loader2, CheckCircle, AlertCircle } from 'lucide-react';
import logo from '../assets/logo.png';
import ReactMarkdown from 'react-markdown';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';
import remarkGfm from 'remark-gfm';
import remarkBreaks from 'remark-breaks';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';
import 'katex/dist/katex.min.css';
import mermaid from 'mermaid';
import './styles/MessageList.css';

interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  timestamp: number;
  skill?: string;
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

interface MessageListProps {
  messages: Message[];
  streamingMessage?: string;
  isStreaming?: boolean;
  thinkingEvents?: ThinkingEvent[];
}

// 初始化 Mermaid
if (typeof window !== 'undefined') {
  mermaid.initialize({
    startOnLoad: false,
    theme: 'dark',
    securityLevel: 'loose',
    themeVariables: {
      darkMode: true,
      background: '#1f2937',
      primaryColor: '#3b82f6',
      primaryTextColor: '#f9fafb',
      primaryBorderColor: '#374151',
      lineColor: '#6b7280',
      secondaryColor: '#1f2937',
      tertiaryColor: '#111827',
    },
  });
}

// Mermaid 图表组件
function MermaidChart({ chart }: { chart: string }) {
  const [svgContent, setSvgContent] = useState<string>('');
  const [error, setError] = useState<string>('');
  const mermaidRef = useRef<HTMLDivElement>(null);
  const id = useMemo(() => `mermaid-${Math.random().toString(36).substr(2, 9)}`, []);

  useEffect(() => {
    const renderMermaid = async () => {
      try {
        const { svg } = await mermaid.render(id, chart);
        setSvgContent(svg);
        setError('');
      } catch (err) {
        console.error('Mermaid render error:', err);
        setError(err instanceof Error ? err.message : '渲染失败');
        setSvgContent('');
      }
    };

    renderMermaid();
  }, [chart, id]);

  return (
    <div className="mermaid-wrapper">
      {error ? (
        <div className="mermaid-error">
          <p>Mermaid 图表渲染失败:</p>
          <pre>{error}</pre>
        </div>
      ) : (
        <div 
          className="mermaid-chart"
          ref={mermaidRef}
          dangerouslySetInnerHTML={{ __html: svgContent }}
        />
      )}
    </div>
  );
}

// 数学公式组件
function MathFormula({ formula, display }: { formula: string; display?: boolean }) {
  return (
    <span 
      className={display ? 'math-display' : 'math-inline'}
      dangerouslySetInnerHTML={{ __html: formula }}
    />
  );
}

// 检测内容是否为 Markdown
function isMarkdown(content: string): boolean {
  const markdownPatterns = [
    /^#{1,6}\s+/m,           // 标题
    /^\*\*.*\*\*/m,          // 粗体
    /^\*.*\*/m,              // 斜体
    /^```/m,                  // 代码块
    /^`[^`]+`/m,             // 行内代码
    /^\[.*\]\(.*\)/m,        // 链接
    /^-\s+/m,                 // 无序列表
    /^\d+\.\s+/m,            // 有序列表
    /^\|.*\|/m,              // 表格
    /^```mermaid/m,           // Mermaid 图表
    /\$\$[\s\S]*?\$\$/,      // 数学公式(块级)
    /\$[^$]+\$/,             // 数学公式(行内)
  ];
  
  return markdownPatterns.some(pattern => pattern.test(content));
}

// 检测内容是否为 HTML
function isHTML(content: string): boolean {
  return /^<[^>]+>.*<\/[^>]+>$/.test(content) || /<[^>]+>/.test(content);
}

interface CodeProps {
  inline?: boolean;
  className?: string;
  children?: React.ReactNode;
}

// 自定义 Markdown 渲染器组件
function MarkdownRenderer({ content }: { content: string }) {
  return (
    <ReactMarkdown
      remarkPlugins={[remarkGfm, remarkBreaks, remarkMath]}
      rehypePlugins={[rehypeKatex]}
      components={{
        code({ inline, className, children, ...props }: CodeProps) {
          const content = String(children).replace(/\n$/, '');
          const match = /language-(\w+)/.exec(className || '');

          // Mermaid 图表
          if (!inline && match && match[1] === 'mermaid') {
            return <MermaidChart chart={content} />;
          }

          // 普通代码块
          if (!inline && match) {
            return (
              <SyntaxHighlighter
                style={vscDarkPlus}
                language={match[1]}
                PreTag="div"
                customStyle={{
                  borderRadius: '8px',
                  margin: '12px 0',
                }}
              >
                {content}
              </SyntaxHighlighter>
            );
          }

          // 行内代码
          return <code className="inline-code" {...props}>{children}</code>;
        },
        a({ href, children, ...props }) {
          return (
            <a href={href} target="_blank" rel="noopener noreferrer" {...props}>
              {children}
            </a>
          );
        },
        img({ src, alt, ...props }) {
          return (
            <img src={src} alt={alt || ''} className="message-image" {...props} />
          );
        },
        table({ children }) {
          return (
            <div className="table-wrapper">
              <table>{children}</table>
            </div>
          );
        },
        del({ children }) {
          return <del className="strikethrough">{children}</del>;
        },
        input({ checked, ...props }) {
          return (
            <input type="checkbox" checked={checked} readOnly className="task-checkbox" {...props} />
          );
        },
        span({ className, children }) {
          if (className === 'math-inline') {
            return <MathFormula formula={String(children)} display={false} />;
          }
          return <span className={className}>{children}</span>;
        },
      }}
    >
      {content}
    </ReactMarkdown>
  );
}

// HTML 渲染器组件
function HTMLRenderer({ content }: { content: string }) {
  return (
    <div 
      className="html-content"
      dangerouslySetInnerHTML={{ __html: content }}
    />
  );
}

// 智能格式化内容
function formatMessageContent(content: string): ReactNode {
  // 空内容处理
  if (!content || content.trim() === '') {
    return null;
  }

  const trimmedContent = content.trim();

  // 优先检测 HTML
  if (isHTML(trimmedContent)) {
    return <HTMLRenderer content={trimmedContent} />;
  }

  // 检测 Markdown
  if (isMarkdown(trimmedContent)) {
    return <MarkdownRenderer content={trimmedContent} />;
  }

  // 普通文本,只处理换行
  return trimmedContent.split('\n').map((line, index) => (
    <span key={index}>
      {line}
      {index < trimmedContent.split('\n').length - 1 && <br />}
    </span>
  ));
}

function formatTime(timestamp: number): string {
  return new Date(timestamp).toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
  });
}

export default function MessageList({ messages, streamingMessage, isStreaming, thinkingEvents }: MessageListProps) {
  const hasThinkingEvents = thinkingEvents && thinkingEvents.length > 0;
  const isThinking = hasThinkingEvents && thinkingEvents[thinkingEvents.length - 1].type !== 'complete' && thinkingEvents[thinkingEvents.length - 1].type !== 'error';

  const chunkEvents = hasThinkingEvents 
    ? thinkingEvents.filter(e => e.type === 'chunk') 
    : [];
  const otherEvents = hasThinkingEvents 
    ? thinkingEvents.filter(e => e.type !== 'chunk') 
    : [];
  const thinkingContent = chunkEvents.map(e => e.content).join('');

  if (messages.length === 0 && !hasThinkingEvents) {
    return (
      <div className="empty-state">
        <div className="empty-icon">
          <img src={logo} alt="MindX Logo" className="empty-logo" />
        </div>
        <h2>开始新对话</h2>
        <p>输入你的问题，我会尽力帮助你</p>
      </div>
    );
  }

  return (
    <div className="message-list">
      {messages.map((message) => (
        <div key={message.id} className={`message message-${message.role}`}>
          <div className="message-avatar">
            {message.role === 'user' ? (
              <UserIcon size={24} />
            ) : (
              <img src={logo} alt="MindX" className="bot-avatar" />
            )}
          </div>
          <div className="message-content">
            <div className="message-header">
              <span className="message-role">
                {message.role === 'user' ? '你' : 'MindX'}
              </span>
              <span className="message-time">{formatTime(message.timestamp)}</span>
            </div>
            <div className="message-body">
              {formatMessageContent(message.content)}
              {message.skill && (
                <div className="skill-badge">
                  <WrenchIcon size={14} />
                  <span>{message.skill}</span>
                </div>
              )}
            </div>
          </div>
        </div>
      ))}
      {hasThinkingEvents && (
        <div className="message message-assistant thinking">
          <div className="message-avatar">
            <img src={logo} alt="MindX" className="bot-avatar" />
          </div>
          <div className="message-content">
            <div className="message-header">
              <span className="message-role">MindX</span>
            </div>
            <div className="message-body thinking-body">
              <div className="thinking-header-inline">
                {isThinking ? (
                  <>
                    <Loader2 className="spinner" size={14} />
                    <span>正在思考...</span>
                  </>
                ) : thinkingEvents[thinkingEvents.length - 1].type === 'error' ? (
                  <>
                    <AlertCircle size={14} className="error" />
                    <span>思考出错</span>
                  </>
                ) : (
                  <>
                    <CheckCircle size={14} className="success" />
                    <span>思考完成</span>
                  </>
                )}
              </div>
              {thinkingContent && (
                <div className="thinking-content-stream">
                  {thinkingContent}
                  {isThinking && <span className="cursor-blink">|</span>}
                </div>
              )}
              {otherEvents.length > 0 && (
                <div className="thinking-events-list">
                  {otherEvents.map((event, index) => (
                    <div key={index} className={`thinking-event-item event-${event.type}`}>
                      <span className="event-type">{getEventLabel(event.type)}</span>
                      <span className="event-content">{event.content}</span>
                      {event.progress > 0 && event.progress < 100 && (
                        <div className="event-progress-mini">
                          <div className="progress-bar" style={{ width: `${event.progress}%` }} />
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      )}
      {isStreaming && streamingMessage && (
        <div className="message message-assistant streaming">
          <div className="message-avatar">
            <img src={logo} alt="MindX" className="bot-avatar" />
          </div>
          <div className="message-content">
            <div className="message-header">
              <span className="message-role">MindX</span>
            </div>
            <div className="message-body">
              {formatMessageContent(streamingMessage)}
              <span className="cursor-blink">|</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function getEventLabel(type: string): string {
  const labels: Record<string, string> = {
    start: '开始',
    progress: '进度',
    chunk: '输出',
    tool_call: '调用工具',
    tool_result: '工具结果',
    complete: '完成',
    error: '错误',
  };
  return labels[type] || type;
}
