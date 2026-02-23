import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import ChannelCard from './ChannelCard';
import type { ChannelConfig } from './types';

const makeChannel = (overrides: Partial<ChannelConfig> = {}): ChannelConfig => ({
  enabled: true,
  name: 'WeChat',
  icon: 'wechat',
  config: {
    port: 8081,
    path: '/wechat/webhook',
  },
  ...overrides,
});

describe('ChannelCard', () => {
  const handlers = {
    onToggle: vi.fn(),
    onStart: vi.fn(),
    onStop: vi.fn(),
    onEdit: vi.fn(),
    configForm: null as React.ReactNode,
  };

  it('renders channel name', () => {
    render(<ChannelCard channelId="wechat" channel={makeChannel()} loading={false} {...handlers} />);
    expect(screen.getByText('WeChat')).toBeInTheDocument();
  });

  it('shows enabled status', () => {
    render(<ChannelCard channelId="wechat" channel={makeChannel()} loading={false} {...handlers} />);
    expect(screen.getByText('已启用')).toBeInTheDocument();
  });

  it('shows disabled status', () => {
    render(<ChannelCard channelId="wechat" channel={makeChannel({ enabled: false })} loading={false} {...handlers} />);
    expect(screen.getByText('已禁用')).toBeInTheDocument();
  });

  it('shows start/stop buttons when enabled', () => {
    render(<ChannelCard channelId="wechat" channel={makeChannel()} loading={false} {...handlers} />);
    expect(screen.getByText('启动')).toBeInTheDocument();
    expect(screen.getByText('停止')).toBeInTheDocument();
  });

  it('hides start/stop buttons when disabled', () => {
    render(<ChannelCard channelId="wechat" channel={makeChannel({ enabled: false })} loading={false} {...handlers} />);
    expect(screen.queryByText('启动')).not.toBeInTheDocument();
    expect(screen.queryByText('停止')).not.toBeInTheDocument();
  });

  it('calls onEdit when config button clicked', () => {
    const onEdit = vi.fn();
    render(<ChannelCard channelId="wechat" channel={makeChannel()} loading={false} {...handlers} onEdit={onEdit} />);
    fireEvent.click(screen.getByText('配置'));
    expect(onEdit).toHaveBeenCalledWith('wechat');
  });

  it('renders port and path in config summary', () => {
    render(<ChannelCard channelId="wechat" channel={makeChannel()} loading={false} {...handlers} />);
    expect(screen.getByText('端口: 8081')).toBeInTheDocument();
    expect(screen.getByText('路径: /wechat/webhook')).toBeInTheDocument();
  });
});
