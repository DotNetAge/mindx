import { useState, useEffect } from 'react';
import './styles/Skills.css';
import { SkillInfo, SkillsResponse, ValidationResult, isMCPSkill } from './skills/types';
import SkillFilters from './skills/SkillFilters';
import SkillCard from './skills/SkillCard';
import SkillEnvDialog from './skills/SkillEnvDialog';

export default function Skills() {
  const [skills, setSkills] = useState<SkillInfo[]>([]);
  const [selectedSkill, setSelectedSkill] = useState<SkillInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [filter, setFilter] = useState<'all' | 'ready' | 'installed' | 'error'>('all');
  const [formatFilter, setFormatFilter] = useState<'all' | 'standard' | 'external' | 'mcp'>('all');
  const [isReIndexing, setIsReIndexing] = useState(false);
  const [reIndexError, setReIndexError] = useState('');

  const [showEnvDialog, setShowEnvDialog] = useState(false);
  const [showInstallDialog, setShowInstallDialog] = useState(false);
  const [showConvertDialog, setShowConvertDialog] = useState(false);
  const [envData, setEnvData] = useState<Record<string, string>>({});
  const [actionLoading, setActionLoading] = useState(false);
  const [actionMessage, setActionMessage] = useState('');

  useEffect(() => {
    fetchSkills();
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
      if (!response.ok) throw new Error('Failed to fetch skills');
      const data: SkillsResponse = await response.json();
      setSkills(data.skills || []);
      setIsReIndexing(data.isReIndexing || false);
      setReIndexError(data.reIndexError || '');
    } catch {
      setError('加载技能列表失败');
    } finally {
      setLoading(false);
    }
  };
  const handleValidate = async (skill: SkillInfo) => {
    try {
      setActionLoading(true);
      setActionMessage('正在验证...');
      const response = await fetch(`/api/skills/${skill.def.name}/validate`);
      if (!response.ok) throw new Error('Failed to validate skill');
      const result: ValidationResult = await response.json();
      if (result.canRun) {
        alert(`✅ 技能 "${skill.def.name}" 验证通过，可以运行！`);
      } else {
        let msg = `❌ 技能 "${skill.def.name}" 验证失败：\n`;
        if (result.missingBins?.length > 0) msg += `\n缺失二进制文件: ${result.missingBins.join(', ')}`;
        if (result.missingEnv?.length > 0) msg += `\n缺失环境变量: ${result.missingEnv.join(', ')}`;
        if (result.errors?.length > 0) msg += `\n\n错误详情:\n${result.errors.map(e => `- ${e.code}: ${e.message}`).join('\n')}`;
        alert(msg);
      }
    } catch {
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
      const response = await fetch(`/api/skills/${skill.def.name}/convert`, { method: 'POST' });
      if (!response.ok) throw new Error('Failed to convert skill');
      alert(`✅ 技能 "${skill.def.name}" 已转换为标准格式`);
      setShowConvertDialog(false);
      fetchSkills();
    } catch {
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
      if ((skill.missingBins?.length ?? 0) > 0) {
        const depsResponse = await fetch(`/api/skills/${skill.def.name}/install`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({}),
        });
        if (!depsResponse.ok) throw new Error('Failed to install dependencies');
      }
      alert(`✅ 技能 "${skill.def.name}" 依赖安装成功`);
      setShowInstallDialog(false);
      fetchSkills();
    } catch {
      alert('安装失败，请查看控制台错误');
    } finally {
      setActionLoading(false);
      setActionMessage('');
    }
  };
  const handleShowEnv = async (skill: SkillInfo) => {
    try {
      const response = await fetch(`/api/skills/${skill.def.name}/env`);
      if (!response.ok) throw new Error('Failed to fetch env');
      const env: Record<string, string> = await response.json();
      setSelectedSkill(skill);
      setEnvData(env);
      setShowEnvDialog(true);
    } catch {
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
      if (!response.ok) throw new Error('Failed to save environment variables');
      alert('✅ 环境变量保存成功');
      setShowEnvDialog(false);
      fetchSkills();
    } catch {
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
      const response = await fetch(`/api/skills/${skill.def.name}/${action}`, { method: 'POST' });
      if (!response.ok) throw new Error(`Failed to ${action} skill`);
      fetchSkills();
    } catch {
      alert(`${action === 'enable' ? '启用' : '禁用'}失败`);
    } finally {
      setActionLoading(false);
      setActionMessage('');
    }
  };

  const handleReIndex = async () => {
    try {
      setActionLoading(true);
      setActionMessage('正在启动重索引...');
      const response = await fetch('/api/skills/reindex', { method: 'POST' });
      if (!response.ok) throw new Error('Failed to trigger reindex');
      await response.json();
      setIsReIndexing(true);
      setReIndexError('');
      fetchSkills();
    } catch {
      alert('启动重索引失败，请查看控制台错误');
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
  if (loading) {
    return <div className="settings-container"><div className="loading">加载中...</div></div>;
  }

  if (isReIndexing) {
    return (
      <div className="settings-container">
        <div className="reindex-overlay">
          <div className="reindex-spinner"></div>
          <p className="reindex-message">正在重索引中......</p>
          {reIndexError && <p className="reindex-error">提示: {reIndexError}</p>}
        </div>
      </div>
    );
  }

  return (
    <div className="settings-container">
      <div className="settings-header">
        <h1>技能管理</h1>
        <div className="header-actions">
          <button className="action-btn secondary" onClick={fetchSkills}>刷新</button>
          <button className="action-btn primary" onClick={handleReIndex} disabled={isReIndexing || actionLoading}>重索引</button>
        </div>
      </div>

      <SkillFilters
        filter={filter}
        formatFilter={formatFilter}
        onFilterChange={setFilter}
        onFormatFilterChange={setFormatFilter}
      />

      {error && <div className="error-message">{error}</div>}

      <div className="skills-content">
        {filteredSkills.length === 0 ? (
          <div className="empty-state">
            <p>暂无技能</p>
            <small>请确保技能目录下有有效的技能配置</small>
          </div>
        ) : (
          <div className="skills-list">
            {filteredSkills.map((skill) => (
              <SkillCard
                key={skill.def.name}
                skill={skill}
                actionLoading={actionLoading}
                onValidate={handleValidate}
                onConvert={(s) => { setSelectedSkill(s); setShowConvertDialog(true); }}
                onInstall={(s) => { setSelectedSkill(s); setShowInstallDialog(true); }}
                onShowEnv={handleShowEnv}
                onToggleEnable={handleToggleEnable}
              />
            ))}
          </div>
        )}
      </div>
      {showInstallDialog && selectedSkill && (
        <div className="dialog-overlay" onClick={() => setShowInstallDialog(false)}>
          <div className="dialog" onClick={(e) => e.stopPropagation()}>
            <h2>安装依赖 - {selectedSkill.def.name}</h2>
            <p>将安装以下缺失的依赖:</p>
            <ul>{(selectedSkill.missingBins ?? []).map((bin) => <li key={bin}>{bin}</li>)}</ul>
            <div className="dialog-actions">
              <button className="action-btn secondary" onClick={() => setShowInstallDialog(false)} disabled={actionLoading}>取消</button>
              <button className="action-btn primary" onClick={() => handleInstall(selectedSkill)} disabled={actionLoading}>
                {actionLoading ? '安装中...' : '开始安装'}
              </button>
            </div>
            {actionMessage && <div className="action-message">{actionMessage}</div>}
          </div>
        </div>
      )}

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
              <button className="action-btn secondary" onClick={() => setShowConvertDialog(false)} disabled={actionLoading}>取消</button>
              <button className="action-btn primary" onClick={() => handleConvert(selectedSkill)} disabled={actionLoading}>
                {actionLoading ? '转换中...' : '开始转换'}
              </button>
            </div>
            {actionMessage && <div className="action-message">{actionMessage}</div>}
          </div>
        </div>
      )}

      {showEnvDialog && selectedSkill && (
        <SkillEnvDialog
          skill={selectedSkill}
          envData={envData}
          actionLoading={actionLoading}
          actionMessage={actionMessage}
          onEnvChange={(key, value) => setEnvData({ ...envData, [key]: value })}
          onSave={handleSaveEnv}
          onClose={() => setShowEnvDialog(false)}
        />
      )}

      {actionLoading && (
        <div className="loading-overlay">
          <div className="loading-spinner"></div>
          <p>{actionMessage || '处理中...'}</p>
        </div>
      )}
    </div>
  );
}
