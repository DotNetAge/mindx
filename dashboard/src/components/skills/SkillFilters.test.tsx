import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import SkillFilters from './SkillFilters';

describe('SkillFilters', () => {
  const defaultProps = {
    filter: 'all' as const,
    formatFilter: 'all' as const,
    onFilterChange: vi.fn(),
    onFormatFilterChange: vi.fn(),
  };

  it('renders both filter dropdowns', () => {
    render(<SkillFilters {...defaultProps} />);
    const selects = screen.getAllByRole('combobox');
    expect(selects).toHaveLength(2);
  });

  it('calls onFilterChange when status filter changes', () => {
    const onFilterChange = vi.fn();
    render(<SkillFilters {...defaultProps} onFilterChange={onFilterChange} />);
    const statusSelect = screen.getByTitle('按状态筛选');
    fireEvent.change(statusSelect, { target: { value: 'ready' } });
    expect(onFilterChange).toHaveBeenCalledWith('ready');
  });

  it('calls onFormatFilterChange when format filter changes', () => {
    const onFormatFilterChange = vi.fn();
    render(<SkillFilters {...defaultProps} onFormatFilterChange={onFormatFilterChange} />);
    const formatSelect = screen.getByTitle('按格式筛选');
    fireEvent.change(formatSelect, { target: { value: 'mcp' } });
    expect(onFormatFilterChange).toHaveBeenCalledWith('mcp');
  });

  it('reflects current filter values', () => {
    render(<SkillFilters {...defaultProps} filter="error" formatFilter="standard" />);
    expect(screen.getByTitle('按状态筛选')).toHaveValue('error');
    expect(screen.getByTitle('按格式筛选')).toHaveValue('standard');
  });
});
