import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import SkillCard from './SkillCard';
import type { SkillInfo } from './types';

const makeSkill = (overrides: Partial<SkillInfo> = {}): SkillInfo => ({
  def: {
    name: 'calculator',
    description: 'A simple calculator',
    version: '1.0.0',
    tags: ['math', 'utility'],
    emoji: '🧮',
    enabled: true,
  },
  format: 'standard',
  status: 'ready',
  content: '',
  directory: '/skills/calculator',
  canRun: true,
  successCount: 10,
  errorCount: 1,
  avgExecutionMs: 50,
  ...overrides,
});

describe('SkillCard', () => {
  const handlers = {
    onValidate: vi.fn(),
    onConvert: vi.fn(),
    onInstall: vi.fn(),
    onShowEnv: vi.fn(),
    onToggleEnable: vi.fn(),
  };

  it('renders skill name and description', () => {
    render(<SkillCard skill={makeSkill()} actionLoading={false} {...handlers} />);
    expect(screen.getByText('calculator')).toBeInTheDocument();
    expect(screen.getByText('A simple calculator')).toBeInTheDocument();
  });

  it('renders tags', () => {
    render(<SkillCard skill={makeSkill()} actionLoading={false} {...handlers} />);
    expect(screen.getByText('math')).toBeInTheDocument();
    expect(screen.getByText('utility')).toBeInTheDocument();
  });

  it('renders stats', () => {
    render(<SkillCard skill={makeSkill()} actionLoading={false} {...handlers} />);
    expect(screen.getByText('成功: 10')).toBeInTheDocument();
    expect(screen.getByText('错误: 1')).toBeInTheDocument();
    expect(screen.getByText('平均: 50ms')).toBeInTheDocument();
  });

  it('shows missing bins warning', () => {
    const skill = makeSkill({ missingBins: ['ffmpeg'] });
    render(<SkillCard skill={skill} actionLoading={false} {...handlers} />);
    expect(screen.getByText(/缺失二进制.*ffmpeg/)).toBeInTheDocument();
  });

  it('calls onValidate when validate button clicked', () => {
    const onValidate = vi.fn();
    const skill = makeSkill();
    render(<SkillCard skill={skill} actionLoading={false} {...handlers} onValidate={onValidate} />);
    fireEvent.click(screen.getByText('验证'));
    expect(onValidate).toHaveBeenCalledWith(skill);
  });

  it('shows convert button for non-standard format', () => {
    const skill = makeSkill({ format: 'external' });
    render(<SkillCard skill={skill} actionLoading={false} {...handlers} />);
    expect(screen.getByText('转换格式')).toBeInTheDocument();
  });

  it('hides convert button for standard format', () => {
    render(<SkillCard skill={makeSkill()} actionLoading={false} {...handlers} />);
    expect(screen.queryByText('转换格式')).not.toBeInTheDocument();
  });

  it('disables buttons when actionLoading is true', () => {
    render(<SkillCard skill={makeSkill()} actionLoading={true} {...handlers} />);
    expect(screen.getByText('验证')).toBeDisabled();
  });
});
