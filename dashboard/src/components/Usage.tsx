import { useState, useEffect } from 'react';
import { RefreshIcon } from 'tdesign-icons-react';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import './Usage.css';

interface TokenUsageByModelSummary {
  model: string;
  total_requests: number;
  total_duration: number;
  avg_duration_per_request: number;
  total_tokens: number;
  total_prompt_tokens: number;
  total_completion_tokens: number;
  avg_tokens_per_request: number;
}

export default function Usage() {
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<TokenUsageByModelSummary[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchTokenUsage();
  }, []);

  const fetchTokenUsage = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch('/api/token-usage/by-model');
      if (!response.ok) {
        throw new Error('获取 Token 使用数据失败');
      }
      const result = await response.json();
      setData(result.data || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : '获取数据失败');
      console.error('Failed to fetch token usage:', err);
    } finally {
      setLoading(false);
    }
  };

  // 准备图表数据
  const chartData = data.map(item => ({
    name: item.model,
    '总 Token 数': item.total_tokens,
    'Prompt Tokens': item.total_prompt_tokens,
    'Completion Tokens': item.total_completion_tokens,
  }));

  // 计算总计
  const totalTokens = data.reduce((sum, item) => sum + item.total_tokens, 0);
  const totalRequests = data.reduce((sum, item) => sum + item.total_requests, 0);
  const totalDuration = data.reduce((sum, item) => sum + item.total_duration, 0);

  const formatNumber = (num: number) => num.toLocaleString();

  return (
    <div className="usage-page">
      <div className="page-header">
        <h1>Token 用量</h1>
        <button
          className="action-btn"
          onClick={fetchTokenUsage}
          disabled={loading}
        >
          <RefreshIcon size={16} />
          刷新
        </button>
      </div>

      {error && (
        <div className="error-banner">
          <span>{error}</span>
          <button className="retry-btn" onClick={fetchTokenUsage}>重试</button>
        </div>
      )}

      {/* 统计卡片 */}
      <div className="stats-grid">
        <div className="stat-card">
          <div className="stat-content">
            <div className="stat-label">总 Token 数</div>
            <div className="stat-value">{formatNumber(totalTokens)}</div>
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-content">
            <div className="stat-label">总请求数</div>
            <div className="stat-value">{formatNumber(totalRequests)}</div>
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-content">
            <div className="stat-label">总时长(秒)</div>
            <div className="stat-value">{(totalDuration / 1000).toFixed(1)}</div>
          </div>
        </div>
      </div>

      {/* 图表 */}
      <div className="chart-section">
        <h2>模型用量对比</h2>
        <div className="chart-container">
          {data.length === 0 ? (
            <div className="empty-state">暂无数据</div>
          ) : (
            <ResponsiveContainer width="100%" height={350}>
              <BarChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
                <XAxis 
                  dataKey="name" 
                  stroke="#9ca3af"
                  tick={{ fill: '#9ca3af', fontSize: 12 }}
                />
                <YAxis 
                  stroke="#9ca3af"
                  tick={{ fill: '#9ca3af', fontSize: 12 }}
                />
                <Tooltip 
                  contentStyle={{ 
                    backgroundColor: '#1f2937', 
                    border: '1px solid #374151',
                    color: '#f9fafb',
                    borderRadius: '6px',
                  }}
                />
                <Legend 
                  wrapperStyle={{ color: '#9ca3af', fontSize: 12 }}
                  iconType="rect"
                />
                <Bar dataKey="总 Token 数" fill="#3b82f6" radius={[4, 4, 0, 0]} />
                <Bar dataKey="Prompt Tokens" fill="#10b981" radius={[4, 4, 0, 0]} />
                <Bar dataKey="Completion Tokens" fill="#f59e0b" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>

      {/* 详细表格 */}
      <div className="table-section">
        <h2>详细数据</h2>
        <div className="table-container">
          <table>
            <thead>
              <tr>
                <th>模型</th>
                <th>请求数</th>
                <th>总 Tokens</th>
                <th>Prompt</th>
                <th>Completion</th>
                <th>平均/请求</th>
                <th>平均时长(ms)</th>
              </tr>
            </thead>
            <tbody>
              {data.map((item, index) => (
                <tr key={index}>
                  <td className="model-name">{item.model}</td>
                  <td>{formatNumber(item.total_requests)}</td>
                  <td>{formatNumber(item.total_tokens)}</td>
                  <td>{formatNumber(item.total_prompt_tokens)}</td>
                  <td>{formatNumber(item.total_completion_tokens)}</td>
                  <td>{formatNumber(Math.round(item.avg_tokens_per_request))}</td>
                  <td>{Math.round(item.avg_duration_per_request)}</td>
                </tr>
              ))}
              {data.length === 0 && (
                <tr>
                  <td colSpan={7} className="empty-row">
                    {loading ? '加载中...' : '暂无数据'}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
