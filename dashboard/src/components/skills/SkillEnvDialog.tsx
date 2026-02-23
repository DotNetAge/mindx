import { SkillInfo } from './types';

interface SkillEnvDialogProps {
  skill: SkillInfo;
  envData: Record<string, string>;
  actionLoading: boolean;
  actionMessage: string;
  onEnvChange: (key: string, value: string) => void;
  onSave: () => void;
  onClose: () => void;
}

export default function SkillEnvDialog({
  skill, envData, actionLoading, actionMessage, onEnvChange, onSave, onClose,
}: SkillEnvDialogProps) {
  const requiredEnv = skill.def.requires?.env || [];

  return (
    <div className="dialog-overlay" onClick={onClose}>
      <div className="dialog" onClick={(e) => e.stopPropagation()}>
        <h2>环境变量 - {skill.def.name}</h2>

        <div className="env-form">
          {requiredEnv.length > 0 ? (
            <>
              <p className="env-hint">配置技能所需的环境变量:</p>
              {requiredEnv.map((envKey) => (
                <div key={envKey} className="env-field">
                  <label>{envKey}</label>
                  <input
                    type="text"
                    value={envData[envKey] || ''}
                    onChange={(e) => onEnvChange(envKey, e.target.value)}
                    placeholder={`输入 ${envKey}`}
                  />
                </div>
              ))}
            </>
          ) : (
            <p>此技能不需要配置环境变量</p>
          )}
        </div>

        <div className="dialog-actions">
          <button className="action-btn secondary" onClick={onClose} disabled={actionLoading}>
            取消
          </button>
          <button className="action-btn primary" onClick={onSave} disabled={actionLoading}>
            {actionLoading ? '保存中...' : '保存'}
          </button>
        </div>

        {actionMessage && <div className="action-message">{actionMessage}</div>}
      </div>
    </div>
  );
}
