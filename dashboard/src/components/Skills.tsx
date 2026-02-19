import { useState, useEffect } from 'react';
import './styles/Skills.css';

interface InstallMethod {
  id: string;
  kind: string;
  formula?: string;
  package?: string;
  label: string;
}

interface Requires {
  bins?: string[];
  env?: string[];
}

interface SkillMetadata {
  name: string;
  description: string;
  homepage?: string;
  version?: string;
  category?: string;
  tags?: string[];
  emoji?: string;
  os?: string[];
  min_bot_version?: string;
  timeout?: number;
  max_memory?: string;
  enabled?: boolean;
  requires?: Requires;
  primaryEnv?: string;
  install?: InstallMethod[];
  command?: string;
}

function isMCPSkill(skill: SkillInfo): boolean {
  const metadata = skill.def.metadata;
  if (!metadata || !metadata.mcp) return false;
  const mcp = metadata.mcp as { server?: string; tool?: string };
  return !!(mcp.server && mcp.tool);
}

interface SkillInfo {
  def: {
    name: string;
    description: string;
    version?: string;
    category?: string;
    tags?: string[];
    emoji?: string;
    os?: string[];
    enabled?: boolean;
    timeout?: number;
    command?: string;
    requires?: {
      bins?: string[];
      env?: string[];
    };
    install?: InstallMethod[];
    metadata?: Record<string, any>;
  };
  format: 'standard' | 'external' | 'mcp';
  status: 'installed' | 'ready' | 'running' | 'stopped' | 'disabled' | 'error';
  content: string;
  directory: string;
  canRun: boolean;
  missingBins?: string[];
  missingEnv?: string[];
  successCount: number;
  errorCount: number;
  lastRunTime?: string;
  lastError?: string;
  avgExecutionMs: number;
}

interface SkillsResponse {
  skills: SkillInfo[];
  count: number;
  isReIndexing: boolean;
  reIndexError: string;
}

interface DependencyCheckResult {
  binsAvailable: boolean;
  missingBins: string[];
  envAvailable: boolean;
  missingEnv: string[];
  osCompatible: boolean;
  errors: string[];
}

interface ValidationResult {
  canRun: boolean;
  binsValid: boolean;
  envValid: boolean;
  osValid: boolean;
  runtimeValid: boolean;
  missingBins: string[];
  missingEnv: string[];
  errors: Array<{
    code: string;
    message: string;
    skillName?: string;
    suggestion?: string;
  }>;
}

export default function Skills() {
  const [skills, setSkills] = useState<SkillInfo[]>([]);
  const [selectedSkill, setSelectedSkill] = useState<SkillInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [filter, setFilter] = useState<'all' | 'ready' | 'installed' | 'error'>('all');
  const [formatFilter, setFormatFilter] = useState<'all' | 'standard' | 'external' | 'mcp'>('all');
  const [isReIndexing, setIsReIndexing] = useState(false);
  const [reIndexError, setReIndexError] = useState('');

  // Dialog states
  const [showEnvDialog, setShowEnvDialog] = useState(false);
  const [showInstallDialog, setShowInstallDialog] = useState(false);
  const [showConvertDialog, setShowConvertDialog] = useState(false);
  const [envData, setEnvData] = useState<Record<string, string>>({});
  const [actionLoading, setActionLoading] = useState(false);
  const [actionMessage, setActionMessage] = useState('');

  useEffect(() => {
    fetchSkills();
    // 定期检查重索引状态
    const interval = setInterval(() => {
      if (isReIndexing) {
        fetchSkills();
      }
    }, 2000);
    return () => clearInterval(interval);
  }, [isReIndexing]);

  const fetchSkills = async () => {
    try {
      setLoading(true);
      const response = await fetch('/api/skills');
      if (!response.ok) {
        throw new Error('Failed to fetch skills');
      }
      const data: SkillsResponse = await response.json();
      setSkills(data.skills || []);
      setIsReIndexing(data.isReIndexing || false);
      setReIndexError(data.reIndexError || '');
    } catch (error) {
      console.error('Failed to fetch skills:', error);
      setError('加载技能列表失败');
    } finally {
      setLoading(false);
    }
  };

  const fetchDependencies = async (name: string): Promise<DependencyCheckResult> => {
    const response = await fetch(`/api/skills/${name}/dependencies`);
    if (!response.ok) {
      throw new Error('Failed to fetch dependencies');
    }
    return await response.json();
  };

  const fetchEnv = async (name: string): Promise<Record<string, string>> => {
    const response = await fetch(`/api/skills/${name}/env`);
    if (!response.ok) {
      throw new Error('Failed to fetch environment variables');
    }
    return await response.json();
  };

  const handleValidate = async (skill: SkillInfo) => {
    try {
      setActionLoading(true);
      setActionMessage('正在验证...');

      const response = await fetch(`/api/skills/${skill.def.name}/validate`);
      if (!response.ok) {
        throw new Error('Failed to validate skill');
      }
      const result: ValidationResult = await response.json();

      if (result.canRun) {
        alert(`✅ 技能 "${skill.def.name}" 验证通过，可以运行！`);
      } else {
        let msg = `❌ 技能 "${skill.def.name}" 验证失败：\n`;
        if (result.missingBins?.length > 0) {
          msg += `\n缺失二进制文件: ${result.missingBins.join(', ')}`;
        }
        if (result.missingEnv?.length > 0) {
          msg += `\n缺失环境变量: ${result.missingEnv.join(', ')}`;
        }
        if (result.errors?.length > 0) {
          msg += `\n\n错误详情:\n${result.errors.map(e => `- ${e.code}: ${e.message}`).join('\n')}`;
        }
        alert(msg);
      }
    } catch (error) {
      console.error('Failed to validate skill:', error);
      alert('验证失败，请查看控制台错误');
    } finally {
      setActionLoading(false);
      setActionMessage('');
    }
  };

  const handleConvert = async (skill: SkillInfo) => {
    try {
      setActionLoading(true);
      setActionMessage('正在转换格式...');

      const response = await fetch(`/api/skills/${skill.def.name}/convert`, {
        method: 'POST',
      });
      if (!response.ok) {
        throw new Error('Failed to convert skill');
      }

      alert(`✅ 技能 "${skill.def.name}" 已转换为标准格式`);
      setShowConvertDialog(false);
      fetchSkills();
    } catch (error) {
      console.error('Failed to convert skill:', error);
      alert('转换失败，请查看控制台错误');
    } finally {
      setActionLoading(false);
      setActionMessage('');
    }
  };

  const handleInstall = async (skill: SkillInfo) => {
    try {
      setActionLoading(true);
      setActionMessage('正在安装依赖和运行时...');

      // 安装二进制依赖
      if ((skill.missingBins?.length ?? 0) > 0) {
        const depsResponse = await fetch(`/api/skills/${skill.def.name}/install`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({}),
        });
        if (!depsResponse.ok) {
          throw new Error('Failed to install dependencies');
        }
      }

      alert(`✅ 技能 "${skill.def.name}" 依赖安装成功`);
      setShowInstallDialog(false);
      fetchSkills();
    } catch (error) {
      console.error('Failed to install:', error);
      alert('安装失败，请查看控制台错误');
    } finally {
      setActionLoading(false);
      setActionMessage('');
    }
  };

  const handleShowEnv = async (skill: SkillInfo) => {
    try {
      const env = await fetchEnv(skill.def.name);
      setSelectedSkill(skill);
      setEnvData(env);
      setShowEnvDialog(true);
    } catch (error) {
      console.error('Failed to fetch env:', error);
      alert('加载环境变量失败');
    }
  };

  const handleSaveEnv = async () => {
    if (!selectedSkill) return;

    try {
      setActionLoading(true);
      setActionMessage('正在保存环境变量...');

      const response = await fetch(`/api/skills/${selectedSkill.def.name}/env`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(envData),
      });
      if (!response.ok) {
        throw new Error('Failed to save environment variables');
      }

      alert('✅ 环境变量保存成功');
      setShowEnvDialog(false);
      fetchSkills();
    } catch (error) {
      console.error('Failed to save env:', error);
      alert('保存失败，请查看控制台错误');
    } finally {
      setActionLoading(false);
      setActionMessage('');
    }
  };

  const handleToggleEnable = async (skill: SkillInfo) => {
    const action = skill.def.enabled ? 'disable' : 'enable';

    try {
      setActionLoading(true);
      setActionMessage(`${action === 'enable' ? '启用' : '禁用'}中...`);

      const response = await fetch(`/api/skills/${skill.def.name}/${action}`, {
        method: 'POST',
      });
      if (!response.ok) {
        throw new Error(`Failed to ${action} skill`);
      }

      fetchSkills();
    } catch (error) {
      console.error(`Failed to ${action} skill:`, error);
      alert(`${action === 'enable' ? '启用' : '禁用'}失败`);
    } finally {
      setActionLoading(false);
      setActionMessage('');
    }
  };

  const filteredSkills = skills.filter((skill) => {
    if (filter !== 'all' && skill.status !== filter) return false;
    if (formatFilter !== 'all') {
      if (formatFilter === 'mcp') {
        if (!isMCPSkill(skill)) return false;
      } else if (skill.format !== formatFilter) {
        return false;
      }
    }
    return true;
  });

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'ready': return '✅';
      case 'running': return '🔄';
      case 'stopped': return '⏹️';
      case 'disabled': return '🚫';
      case 'error': return '❌';
      default: return '⏳';
    }
  };

  const getFormatTag = (skill: SkillInfo) => {
    if (isMCPSkill(skill)) return '[MCP]';
    switch (skill.format) {
      case 'standard': return '[std]';
      case 'external': return '[ext]';
      default: return '[?]';
    }
  };

  const handleReIndex = async () => {
    try {
      setActionLoading(true);
      setActionMessage('正在启动重索引...');

      const response = await fetch('/api/skills/reindex', { method: 'POST' });
      if (!response.ok) {
        throw new Error('Failed to trigger reindex');
      }

      const data = await response.json();
      setIsReIndexing(true);
      setReIndexError('');
      fetchSkills();
    } catch (error) {
      console.error('Failed to trigger reindex:', error);
      alert('启动重索引失败，请查看控制台错误');
    } finally {
      setActionLoading(false);
      setActionMessage('');
    }
  };

  if (loading) {
    return (
      <div className="settings-container">
        <div className="loading">加载中...</div>
      </div>
    );
  }

  // 如果正在重索引，显示遮罩
  if (isReIndexing) {
    return (
      <div className="settings-container">
        <div className="reindex-overlay">
          <div className="reindex-spinner"></div>
          <p className="reindex-message">正在重索引中......</p>
          {reIndexError && (
            <p className="reindex-error">提示: {reIndexError}</p>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="settings-container">
      <div className="settings-header">
        <h1>技能管理</h1>
        <div className="header-actions">
          <button className="action-btn secondary" onClick={fetchSkills}>
            刷新
          </button>
          <button className="action-btn primary" onClick={handleReIndex} disabled={isReIndexing || actionLoading}>
            重索引
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="skills-filters">
        <div className="filter-group">
          <label>状态:</label>
          <select value={filter} onChange={(e) => setFilter(e.target.value as 'all' | 'ready' | 'installed' | 'error')} title="按状态筛选">
            <option value="all">全部</option>
            <option value="ready">✅ 准备就绪</option>
            <option value="installed">⏳ 已安装</option>
            <option value="error">❌ 错误</option>
          </select>
        </div>
        <div className="filter-group">
          <label>格式:</label>
          <select value={formatFilter} onChange={(e) => setFormatFilter(e.target.value as 'all' | 'standard' | 'external' | 'mcp')} title="按格式筛选">
            <option value="all">全部</option>
            <option value="standard">[std] 标准</option>
            <option value="external">[ext] 外部</option>
            <option value="mcp">[MCP] MCP 技能</option>
          </select>
        </div>
      </div>

      {error && (
        <div className="error-message">{error}</div>
      )}

      <div className="skills-content">
        {filteredSkills.length === 0 ? (
          <div className="empty-state">
            <p>暂无技能</p>
            <small>请确保技能目录下有有效的技能配置</small>
          </div>
        ) : (
          <div className="skills-list">
            {filteredSkills.map((skill) => (
              <div key={skill.def.name} className="skill-card">
                <div className="skill-header">
                  <div className="skill-title">
                    <h3>
                      {skill.def.emoji && <span>{skill.def.emoji} </span>}
                      {skill.def.name}
                    </h3>
                    <span className="skill-version">{skill.def.version || 'N/A'}</span>
                  </div>
                  <div className="skill-badges">
                    <span className={`badge ${isMCPSkill(skill) ? 'format-mcp' : 'format-' + skill.format}`}>
                      {getFormatTag(skill)}
                    </span>
                    <span className={`badge status-${skill.status}`}>
                      {getStatusIcon(skill.status)} {skill.status}
                    </span>
                  </div>
                </div>

                <p className="skill-description">{skill.def.description}</p>

                {/* Missing dependencies */}
                {((skill.missingBins?.length ?? 0) > 0 || (skill.missingEnv?.length ?? 0) > 0) && (
                  <div className="skill-warnings">
                    {(skill.missingBins?.length ?? 0) > 0 && (
                      <div key="missing-bins" className="warning missing-bins">
                        ⚠️ 缺失二进制: {(skill.missingBins ?? []).join(', ')}
                      </div>
                    )}
                    {(skill.missingEnv?.length ?? 0) > 0 && (
                      <div key="missing-env" className="warning missing-env">
                        🔑 缺失环境变量: {(skill.missingEnv ?? []).join(', ')}
                      </div>
                    )}
                  </div>
                )}

                {/* Statistics */}
                <div className="skill-stats">
                  <span key="success">成功: {skill.successCount}</span>
                  <span key="error">错误: {skill.errorCount}</span>
                  <span key="avg">平均: {skill.avgExecutionMs}ms</span>
                  {skill.lastRunTime && (
                    <span key="last-run">最后运行: {new Date(skill.lastRunTime).toLocaleString()}</span>
                  )}
                </div>

                {/* Tags */}
                {skill.def.tags && skill.def.tags.length > 0 && (
                  <div className="skill-tags">
                    {skill.def.tags.map((tag, idx) => (
                      <span key={idx} className="tag">{tag}</span>
                    ))}
                  </div>
                )}

                {/* Actions */}
                <div className="skill-actions">
                  <button
                    className="action-btn secondary"
                    onClick={() => handleValidate(skill)}
                    disabled={actionLoading}
                  >
                    验证
                  </button>
                  {skill.format !== 'standard' && (
                    <button
                      className="action-btn warning"
                      onClick={() => {
                        setSelectedSkill(skill);
                        setShowConvertDialog(true);
                      }}
                      disabled={actionLoading}
                    >
                      转换格式
                    </button>
                  )}
                  {(skill.missingBins?.length ?? 0) > 0 && (
                    <button
                      className="action-btn primary"
                      onClick={() => {
                        setSelectedSkill(skill);
                        setShowInstallDialog(true);
                      }}
                      disabled={actionLoading}
                    >
                      安装依赖
                    </button>
                  )}
                  {skill.def.requires?.env?.length > 0 && (
                    <button
                      className="action-btn secondary"
                      onClick={() => handleShowEnv(skill)}
                      disabled={actionLoading}
                    >
                      环境变量
                    </button>
                  )}
                  <button
                    className={`action-btn ${skill.def.enabled ? 'danger' : 'success'}`}
                    onClick={() => handleToggleEnable(skill)}
                    disabled={actionLoading}
                  >
                    {skill.def.enabled ? '禁用' : '启用'}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Install Dialog */}
      {showInstallDialog && selectedSkill && (
        <div className="dialog-overlay" onClick={() => setShowInstallDialog(false)}>
          <div className="dialog" onClick={(e) => e.stopPropagation()}>
            <h2>安装依赖 - {selectedSkill.def.name}</h2>

            {(selectedSkill.missingBins?.length ?? 0) > 0 && (
              <div className="dialog-section">
                <h3>需要安装的二进制文件:</h3>
                <ul>
                  {(selectedSkill.missingBins ?? []).map((bin, idx) => (
                    <li key={idx}>{bin}</li>
                  ))}
                </ul>
              </div>
            )}

            {selectedSkill.def.install && selectedSkill.def.install.length > 0 && (
              <div className="dialog-section">
                <h3>可用的安装方法:</h3>
                {selectedSkill.def.install.map((method, idx) => (
                  <div key={idx} className="install-method">
                    <strong>{method.label}</strong>
                    <small>类型: {method.kind}</small>
                  </div>
                ))}
              </div>
            )}

            <div className="dialog-actions">
              <button
                className="action-btn secondary"
                onClick={() => setShowInstallDialog(false)}
                disabled={actionLoading}
              >
                取消
              </button>
              <button
                className="action-btn primary"
                onClick={() => handleInstall(selectedSkill)}
                disabled={actionLoading}
              >
                {actionLoading ? '安装中...' : '开始安装'}
              </button>
            </div>

            {actionMessage && <div className="action-message">{actionMessage}</div>}
          </div>
        </div>
      )}

      {/* Convert Dialog */}
      {showConvertDialog && selectedSkill && (
        <div className="dialog-overlay" onClick={() => setShowConvertDialog(false)}>
          <div className="dialog" onClick={(e) => e.stopPropagation()}>
            <h2>转换格式 - {selectedSkill.def.name}</h2>
            <p>当前格式: <strong>{selectedSkill.format}</strong></p>
            <p>目标格式: <strong>标准格式 (standard)</strong></p>

            <div className="dialog-section">
              <h3>转换将:</h3>
              <ul>
                <li>添加缺失的元数据字段</li>
                <li>统一格式为标准YAML frontmatter</li>
                <li>保留原有的Markdown内容</li>
              </ul>
            </div>

            <div className="dialog-actions">
              <button
                className="action-btn secondary"
                onClick={() => setShowConvertDialog(false)}
                disabled={actionLoading}
              >
                取消
              </button>
              <button
                className="action-btn primary"
                onClick={() => handleConvert(selectedSkill)}
                disabled={actionLoading}
              >
                {actionLoading ? '转换中...' : '开始转换'}
              </button>
            </div>

            {actionMessage && <div className="action-message">{actionMessage}</div>}
          </div>
        </div>
      )}

      {/* Environment Variables Dialog */}
      {showEnvDialog && selectedSkill && (
        <div className="dialog-overlay" onClick={() => setShowEnvDialog(false)}>
          <div className="dialog" onClick={(e) => e.stopPropagation()}>
            <h2>环境变量 - {selectedSkill.def.name}</h2>

            <div className="dialog-section">
              {selectedSkill.def.requires?.env?.length > 0 ? (
                <>
                  <h3>需要的环境变量:</h3>
                  {selectedSkill.def.requires.env.map((envVar, idx) => (
                    <div key={idx} className="env-item">
                      <label>{envVar}</label>
                      <input
                        type={envVar.toLowerCase().includes('password') ||
                                 envVar.toLowerCase().includes('secret') ||
                                 envVar.toLowerCase().includes('token')
                               ? 'password'
                               : 'text'}
                        value={envData[envVar] || ''}
                        onChange={(e) => setEnvData({ ...envData, [envVar]: e.target.value })}
                        placeholder={`输入 ${envVar}`}
                      />
                    </div>
                  ))}
                </>
              ) : (
                <p>此技能不需要配置环境变量</p>
              )}
            </div>

            <div className="dialog-actions">
              <button
                className="action-btn secondary"
                onClick={() => setShowEnvDialog(false)}
                disabled={actionLoading}
              >
                取消
              </button>
              <button
                className="action-btn primary"
                onClick={handleSaveEnv}
                disabled={actionLoading}
              >
                {actionLoading ? '保存中...' : '保存'}
              </button>
            </div>

            {actionMessage && <div className="action-message">{actionMessage}</div>}
          </div>
        </div>
      )}

      {/* Loading overlay */}
      {actionLoading && (
        <div className="loading-overlay">
          <div className="loading-spinner"></div>
          <p>{actionMessage || '处理中...'}</p>
        </div>
      )}
    </div>
  );
}
