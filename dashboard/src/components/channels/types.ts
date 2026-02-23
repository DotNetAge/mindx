export interface ChannelConfig {
  enabled: boolean;
  name: string;
  icon: string;
  config: {
    [key: string]: unknown;
  };
}

export interface ChannelsData {
  enabled_channels: string[];
  channels: {
    [key: string]: ChannelConfig;
  };
}
