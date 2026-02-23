interface SkillFiltersProps {
  filter: 'all' | 'ready' | 'installed' | 'error';
  formatFilter: 'all' | 'standard' | 'external' | 'mcp';
  onFilterChange: (value: 'all' | 'ready' | 'installed' | 'error') => void;
  onFormatFilterChange: (value: 'all' | 'standard' | 'external' | 'mcp') => void;
}

export default function SkillFilters({ filter, formatFilter, onFilterChange, onFormatFilterChange }: SkillFiltersProps) {
  return (
    <div className="skills-filters">
      <div className="filter-group">
        <label>状态:</label>
        <select value={filter} onChange={(e) => onFilterChange(e.target.value as typeof filter)} title="按状态筛选">
          <option value="all">全部</option>
          <option value="ready">✅ 准备就绪</option>
          <option value="installed">⏳ 已安装</option>
          <option value="error">❌ 错误</option>
        </select>
      </div>
      <div className="filter-group">
        <label>格式:</label>
        <select value={formatFilter} onChange={(e) => onFormatFilterChange(e.target.value as typeof formatFilter)} title="按格式筛选">
          <option value="all">全部</option>
          <option value="standard">[std] 标准</option>
          <option value="external">[ext] 外部</option>
          <option value="mcp">[MCP] MCP 技能</option>
        </select>
      </div>
    </div>
  );
}
