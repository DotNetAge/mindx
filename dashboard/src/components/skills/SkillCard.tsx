import { SkillInfo, isMCPSkill } from './types';

function getStatusIcon(status: string) {
  switch (status) {
    case 'ready': return '✅';
    case 'running': return '🔄';
    case 'stopped': return '⏹️';
    case 'disabled': return '🚫';
    case 'error': return '❌';
    default: return '⏳';
  }
}

function getFormatTag(skill: SkillInfo) {
  if (isMCPSkill(skill)) return '[MCP]';
  switch (skill.format) {
    case 'standard': return '[std]';
    case 'external': return '[ext]';
    default: return '[?]';
  }
}

interface SkillCardProps {
  skill: SkillInfo;
  actionLoading: boolean;
  onValidate: (skill: SkillInfo) => void;
  onConvert: (skill: SkillInfo) => void;
  onInstall: (skill: SkillInfo) => void;
  onShowEnv: (skill: SkillInfo) => void;
  onToggleEnable: (skill: SkillInfo) => void;
}

export default function SkillCard({
  skill, actionLoading, onValidate, onConvert, onInstall, onShowEnv, onToggleEnable,
}: SkillCardProps) {
  return (
    <div className="skill-card">
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

      <div className="skill-stats">
        <span key="success">成功: {skill.successCount}</span>
        <span key="error">错误: {skill.errorCount}</span>
        <span key="avg">平均: {skill.avgExecutionMs}ms</span>
        {skill.lastRunTime && (
          <span key="last-run">最后运行: {new Date(skill.lastRunTime).toLocaleString()}</span>
        )}
      </div>

      {skill.def.tags && skill.def.tags.length > 0 && (
        <div className="skill-tags">
          {skill.def.tags.map((tag, idx) => (
            <span key={idx} className="tag">{tag}</span>
          ))}
        </div>
      )}
      <div className="skill-actions">
        <button className="action-btn secondary" onClick={() => onValidate(skill)} disabled={actionLoading}>
          验证
        </button>
        {skill.format !== 'standard' && (
          <button className="action-btn warning" onClick={() => onConvert(skill)} disabled={actionLoading}>
            转换格式
          </button>
        )}
        {(skill.missingBins?.length ?? 0) > 0 && (
          <button className="action-btn primary" onClick={() => onInstall(skill)} disabled={actionLoading}>
            安装依赖
          </button>
        )}
        {skill.def.requires?.env && skill.def.requires.env.length > 0 && (
          <button className="action-btn secondary" onClick={() => onShowEnv(skill)} disabled={actionLoading}>
            环境变量
          </button>
        )}
        <button
          className={`action-btn ${skill.def.enabled ? 'danger' : 'success'}`}
          onClick={() => onToggleEnable(skill)}
          disabled={actionLoading}
        >
          {skill.def.enabled ? '禁用' : '启用'}
        </button>
      </div>
    </div>
  );
}
