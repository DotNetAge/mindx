import { useState, useEffect } from 'react';
import {
  ChatIcon,
  LogoGithubIcon,
  ToolsIcon,
  InfoCircleFilledIcon,
  PreciseMonitorIcon,
  ShareIcon,
  ChartIcon,
  TimeFilledIcon,
  SettingIcon
} from 'tdesign-icons-react';
import logo from '../assets/logo.png';
import './styles/Sidebar.css';
import { useTranslation } from '../i18n';

interface SidebarProps {
  activeTab: string;
  onTabChange: (tab: string) => void;
}

export default function Sidebar({ activeTab, onTabChange }: SidebarProps) {
  const [healthy, setHealthy] = useState(true);
  const { t, language, setLanguage } = useTranslation();

  useEffect(() => {
    checkHealth();
    const interval = setInterval(checkHealth, 30000);
    return () => clearInterval(interval);
  }, []);

  const checkHealth = async () => {
    try {
      const response = await fetch('/api/health');
      setHealthy(response.ok);
    } catch {
      setHealthy(false);
    }
  };

  const toggleService = async () => {
    const endpoint = healthy ? '/api/service/stop' : '/api/service/start';
    try {
      const response = await fetch(endpoint, { method: 'POST' });
      if (response.ok) {
        setHealthy(!healthy);
      }
    } catch (error) {
      console.error('Failed to toggle service:', error);
    }
  };

  const toggleLanguage = () => {
    setLanguage(language === 'zh-CN' ? 'en-US' : 'zh-CN');
  };

  const menuItems = [
    { id: 'chat', label: t('sidebar.chat'), icon: <ChatIcon /> },
    { id: 'history', label: t('sidebar.history'), icon: <TimeFilledIcon /> },
    { id: 'skills', label: t('sidebar.skills'), icon: <ToolsIcon /> },
    { id: 'capabilities', label: t('sidebar.capabilities'), icon: <InfoCircleFilledIcon /> },
    { id: 'channels', label: t('sidebar.channels'), icon: <ShareIcon /> },
    { id: 'usage', label: t('sidebar.usage'), icon: <ChartIcon /> },
    { id: 'monitor', label: t('sidebar.monitor'), icon: <PreciseMonitorIcon /> },
    { id: 'advanced', label: t('sidebar.advanced'), icon: <SettingIcon /> },
  ];

  return (
    <aside className="sidebar">
      <div className="sidebar-header">
        <div className="logo">
          <img src={logo} alt="MindX Logo" className="logo-img" />
          <span className="logo-text">心智 | MindX</span>
        </div>
        <button
          className={`service-toggle ${healthy ? 'healthy' : 'unhealthy'}`}
          onClick={toggleService}
          title={healthy ? t('sidebar.stopService') : t('sidebar.startService')}
        >
          <span className={`status-dot ${healthy ? 'running' : 'stopped'}`}></span>
          {healthy ? t('sidebar.running') : t('sidebar.stopped')}
        </button>
      </div>

      <nav className="sidebar-nav">
        {menuItems.map((item) => (
          <button
            key={item.id}
            className={`nav-item ${activeTab === item.id ? 'active' : ''}`}
            onClick={() => onTabChange(item.id)}
          >
            <span className="nav-icon">{item.icon}</span>
            <span className="nav-label">{item.label}</span>
          </button>
        ))}
      </nav>

      <div className="sidebar-footer">
        <button
          className="language-toggle"
          onClick={toggleLanguage}
          title={language === 'zh-CN' ? 'Switch to English' : '切换到中文'}
        >
          {language === 'zh-CN' ? 'EN' : '中文'}
        </button>
        <a href="https://github.com/DotNetAge/mindx.git" target="_blank" rel="noopener noreferrer" className="footer-link">
          <LogoGithubIcon size="20" style={{ marginRight: '5px' }} />
          <span>GitHub</span>
        </a>
      </div>
    </aside>
  );
}
