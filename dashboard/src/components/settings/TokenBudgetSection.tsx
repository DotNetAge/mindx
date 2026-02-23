import { useTranslation } from '../../i18n';
import type { TokenBudgetConfig } from './types';

interface TokenBudgetSectionProps {
  config: TokenBudgetConfig;
  onUpdate: (field: keyof TokenBudgetConfig, value: number) => void;
}

export default function TokenBudgetSection({ config, onUpdate }: TokenBudgetSectionProps) {
  const { t } = useTranslation();

  return (
    <div className="config-section">
      <h3>{t('advanced.tokenBudget')}</h3>
      <div className="config-item">
        <label>{t('advanced.reservedOutputTokens')}</label>
        <input
          type="number"
          value={config.reserved_output_tokens}
          onChange={(e) => onUpdate('reserved_output_tokens', parseInt(e.target.value) || 0)}
          title="预留输出 Token 数"
          placeholder="8192"
        />
        <small>{t('advanced.reservedOutputTokensDesc')}</small>
      </div>
      <div className="config-item">
        <label>{t('advanced.minHistoryRounds')}</label>
        <input
          type="number"
          value={config.min_history_rounds}
          onChange={(e) => onUpdate('min_history_rounds', parseInt(e.target.value) || 0)}
          title="最小历史对话轮数"
          placeholder="5"
        />
      </div>
      <div className="config-item">
        <label>{t('advanced.avgTokensPerRound')}</label>
        <input
          type="number"
          value={config.avg_tokens_per_round}
          onChange={(e) => onUpdate('avg_tokens_per_round', parseInt(e.target.value) || 0)}
          title="单轮对话平均 Token 数"
          placeholder="200"
        />
      </div>
    </div>
  );
}
