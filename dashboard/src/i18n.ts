import { useState, useEffect } from 'react';

export type Language = 'zh-CN' | 'en-US';

const translations = {
  'zh-CN': {
    common: {
      loading: '加载中...',
      error: '错误',
      success: '成功',
      cancel: '取消',
      confirm: '确认',
      save: '保存',
      delete: '删除',
      edit: '编辑',
      add: '添加',
      search: '搜索',
      noData: '暂无数据',
    },
    sidebar: {
      chat: '对话',
      history: '历史',
      models: '模型',
      skills: '技能',
      capabilities: '能力',
      channels: '通信',
      usage: '用量',
      monitor: '日志',
      cron: '任务',
      advanced: '高级',
      running: '运行中',
      stopped: '已停止',
      stopService: '停止服务',
      startService: '启动服务',
    },
    chat: {
      title: '对话',
      placeholder: '输入消息...',
      send: '发送',
      thinking: '思考中...',
      newChat: '新对话',
      clearHistory: '清空历史',
      connecting: '正在连接服务器...',
      connectionError: '连接错误',
      notConnected: '未连接到服务器',
      sendFailed: '发送消息失败',
      reconnect: '重新连接',
    },
    skills: {
      title: '技能管理',
      description: '管理已安装的技能',
      installed: '已安装',
      available: '可用',
      install: '安装',
      uninstall: '卸载',
      enable: '启用',
      disable: '禁用',
      run: '运行',
      configure: '配置',
      noSkills: '暂无已安装的技能',
    },
    capabilities: {
      title: '系统能力',
      description: '查看系统当前支持的能力',
      models: '模型',
      tools: '工具',
      memory: '记忆',
      learning: '学习',
    },
    channels: {
      title: '通信渠道',
      description: '管理外部通信渠道',
      wechat: '微信',
      telegram: 'Telegram',
      discord: 'Discord',
      webhook: 'Webhook',
      connected: '已连接',
      disconnected: '未连接',
    },
    usage: {
      title: '用量统计',
      description: '查看系统资源使用情况',
      today: '今日',
      week: '本周',
      month: '本月',
      messages: '消息数',
      tokens: 'Token 数',
      storage: '存储',
    },
    monitor: {
      title: '系统监控',
      description: '实时监控系统状态',
      cpu: 'CPU',
      memory: '内存',
      disk: '磁盘',
      network: '网络',
      uptime: '运行时间',
    },
    settings: {
      title: '设置',
      general: '常规',
      advanced: '高级',
      language: '语言',
      theme: '主题',
      darkMode: '深色模式',
      lightMode: '浅色模式',
      autoMode: '跟随系统',
    },
    advanced: {
      title: '高级配置',
      ollamaStatus: 'Ollama 状态',
      ollamaInstalled: '✓ Ollama 已安装',
      ollamaNotInstalled: '✗ Ollama 未安装',
      ollamaRunning: '✓ Ollama 运行中',
      ollamaNotRunning: '⚠ Ollama 未运行',
      installedModels: '已安装模型：',
      checking: '检查中...',
      installOllama: '安装 Ollama',
      ollamaInstalling: 'Ollama 已开始在后台安装，请稍后检查状态',
      ollamaInstallFailed: 'Ollama 安装失败',
      basicConfig: '基础配置',
      ollamaUrl: 'Ollama URL',
      indexModel: '索引模型',
      embeddingModel: 'Embedding 模型',
      leftbrainConfig: '左脑配置（行动）',
      leftbrainDesc: '负责执行任务和工具调用',
      rightbrainConfig: '右脑配置（思考）',
      rightbrainDesc: '负责思考和推理',
      modelName: '模型名称',
      baseUrl: 'Base URL',
      apiKey: 'API Key',
      temperature: '温度',
      maxTokens: '最大 Token 数',
      mustSupportFC: '必须支持 Function Call',
      test: '测试',
      testing: '测试中...',
      modelSupportFC: '模型 {{model}} 支持 Function Call ✓',
      modelNotSupportFC: '模型 {{model}} 不支持 Function Call ✗',
      modelTestFailed: '模型 {{model}} 测试失败',
      tokenBudget: 'Token 预算配置',
      reservedOutputTokens: '预留输出 Token 数',
      reservedOutputTokensDesc: '预留给输出的 Token 数',
      minHistoryRounds: '最小历史对话轮数',
      avgTokensPerRound: '单轮对话平均 Token 数',
      memoryConfig: '记忆配置',
      enableMemory: '启用记忆功能',
      summaryModel: '摘要模型',
      keywordModel: '关键词模型',
      schedule: '定时任务 (Cron)',
      scheduleDesc: '格式：秒 分 时 日 月 周，例如：0 0 12 * * *',
      vectorStore: '向量存储',
      vectorStoreType: '存储类型',
      vectorStoreMemory: '内存存储',
      vectorStoreBadger: 'Badger 存储',
      vectorStoreDataPath: '数据路径',
      save: '保存配置',
      saving: '保存中...',
      saveSuccess: '配置保存成功！',
      saveFailed: '配置保存失败',
    },
  },
  'en-US': {
    common: {
      loading: 'Loading...',
      error: 'Error',
      success: 'Success',
      cancel: 'Cancel',
      confirm: 'Confirm',
      save: 'Save',
      delete: 'Delete',
      edit: 'Edit',
      add: 'Add',
      search: 'Search',
      noData: 'No data',
    },
    sidebar: {
      chat: 'Chat',
      history: 'History',
      models: 'Models',
      skills: 'Skills',
      capabilities: 'Capabilities',
      channels: 'Channels',
      usage: 'Usage',
      monitor: 'Logs',
      cron: 'Scheduled Tasks',
      advanced: 'Advanced',
      running: 'Running',
      stopped: 'Stopped',
      stopService: 'Stop Service',
      startService: 'Start Service',
    },
    chat: {
      title: 'Chat',
      placeholder: 'Type a message...',
      send: 'Send',
      thinking: 'Thinking...',
      newChat: 'New Chat',
      clearHistory: 'Clear History',
      connecting: 'Connecting to server...',
      connectionError: 'Connection error',
      notConnected: 'Not connected to server',
      sendFailed: 'Failed to send message',
      reconnect: 'Reconnect',
    },
    skills: {
      title: 'Skills Management',
      description: 'Manage installed skills',
      installed: 'Installed',
      available: 'Available',
      install: 'Install',
      uninstall: 'Uninstall',
      enable: 'Enable',
      disable: 'Disable',
      run: 'Run',
      configure: 'Configure',
      noSkills: 'No skills installed',
    },
    capabilities: {
      title: 'System Capabilities',
      description: 'View current system capabilities',
      models: 'Models',
      tools: 'Tools',
      memory: 'Memory',
      learning: 'Learning',
    },
    channels: {
      title: 'Communication Channels',
      description: 'Manage external communication channels',
      wechat: 'WeChat',
      telegram: 'Telegram',
      discord: 'Discord',
      webhook: 'Webhook',
      connected: 'Connected',
      disconnected: 'Disconnected',
    },
    usage: {
      title: 'Usage Statistics',
      description: 'View system resource usage',
      today: 'Today',
      week: 'This Week',
      month: 'This Month',
      messages: 'Messages',
      tokens: 'Tokens',
      storage: 'Storage',
    },
    monitor: {
      title: 'System Monitor',
      description: 'Real-time system monitoring',
      cpu: 'CPU',
      memory: 'Memory',
      disk: 'Disk',
      network: 'Network',
      uptime: 'Uptime',
    },
    settings: {
      title: 'Settings',
      general: 'General',
      advanced: 'Advanced',
      language: 'Language',
      theme: 'Theme',
      darkMode: 'Dark Mode',
      lightMode: 'Light Mode',
      autoMode: 'Auto',
    },
    advanced: {
      title: 'Advanced Settings',
      ollamaStatus: 'Ollama Status',
      ollamaInstalled: '✓ Ollama Installed',
      ollamaNotInstalled: '✗ Ollama Not Installed',
      ollamaRunning: '✓ Ollama Running',
      ollamaNotRunning: '⚠ Ollama Not Running',
      installedModels: 'Installed Models:',
      checking: 'Checking...',
      installOllama: 'Install Ollama',
      ollamaInstalling: 'Ollama installation started in background, please check status later',
      ollamaInstallFailed: 'Ollama installation failed',
      basicConfig: 'Basic Configuration',
      ollamaUrl: 'Ollama URL',
      indexModel: 'Index Model',
      embeddingModel: 'Embedding Model',
      leftbrainConfig: 'Leftbrain Configuration (Action)',
      leftbrainDesc: 'Responsible for task execution and tool calls',
      rightbrainConfig: 'Rightbrain Configuration (Thinking)',
      rightbrainDesc: 'Responsible for thinking and reasoning',
      modelName: 'Model Name',
      baseUrl: 'Base URL',
      apiKey: 'API Key',
      temperature: 'Temperature',
      maxTokens: 'Max Tokens',
      mustSupportFC: 'Must support Function Call',
      test: 'Test',
      testing: 'Testing...',
      modelSupportFC: 'Model {{model}} supports Function Call ✓',
      modelNotSupportFC: 'Model {{model}} does not support Function Call ✗',
      modelTestFailed: 'Model {{model}} test failed',
      tokenBudget: 'Token Budget Configuration',
      reservedOutputTokens: 'Reserved Output Tokens',
      reservedOutputTokensDesc: 'Tokens reserved for output',
      minHistoryRounds: 'Min History Rounds',
      avgTokensPerRound: 'Avg Tokens Per Round',
      memoryConfig: 'Memory Configuration',
      enableMemory: 'Enable Memory',
      summaryModel: 'Summary Model',
      keywordModel: 'Keyword Model',
      schedule: 'Schedule (Cron)',
      scheduleDesc: 'Format: sec min hour day month weekday, e.g.: 0 0 12 * * *',
      vectorStore: 'Vector Store',
      vectorStoreType: 'Storage Type',
      vectorStoreMemory: 'Memory Storage',
      vectorStoreBadger: 'Badger Storage',
      vectorStoreDataPath: 'Data Path',
      save: 'Save Configuration',
      saving: 'Saving...',
      saveSuccess: 'Configuration saved successfully!',
      saveFailed: 'Failed to save configuration',
    },
  },
};

class I18n {
  private currentLanguage: Language = 'zh-CN';
  private listeners: Set<() => void> = new Set();

  constructor() {
    const savedLang = localStorage.getItem('mindx-language') as Language;
    if (savedLang && (savedLang === 'zh-CN' || savedLang === 'en-US')) {
      this.currentLanguage = savedLang;
    } else {
      const browserLang = navigator.language;
      if (browserLang.startsWith('zh')) {
        this.currentLanguage = 'zh-CN';
      } else {
        this.currentLanguage = 'en-US';
      }
    }
  }

  getLanguage(): Language {
    return this.currentLanguage;
  }

  setLanguage(lang: Language) {
    this.currentLanguage = lang;
    localStorage.setItem('mindx-language', lang);
    this.listeners.forEach(listener => listener());
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  t(key: string, params?: Record<string, string | number>): string {
    const keys = key.split('.');
    let value: unknown = translations[this.currentLanguage];

    for (const k of keys) {
      if (value && typeof value === 'object' && k in value) {
        value = (value as Record<string, unknown>)[k];
      } else {
        return key;
      }
    }

    if (typeof value !== 'string') {
      return key;
    }

    if (params) {
      return Object.entries(params).reduce(
        (str, [k, v]) => str.replace(new RegExp(`{{${k}}}`, 'g'), String(v)),
        value
      );
    }

    return value;
  }
}

export const i18n = new I18n();

export const useTranslation = () => {
  const [, forceUpdate] = useState({});

  useEffect(() => {
    return i18n.subscribe(() => forceUpdate({}));
  }, []);

  return {
    t: i18n.t.bind(i18n),
    language: i18n.getLanguage(),
    setLanguage: i18n.setLanguage.bind(i18n),
  };
};
