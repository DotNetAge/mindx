import { useState } from 'react';
import Sidebar from './components/Sidebar';
import Chat from './components/Chat';
import Settings from './components/Settings';
import GeneralSettings from './components/GeneralSettings';
import Skills from './components/Skills';
import Capabilities from './components/Capabilities';
import Models from './components/Models';
import AdvancedSettings from './components/AdvancedSettings';
import Monitor from './components/Monitor';
import Channels from './components/Channels';
import Usage from './components/Usage';
import History from './components/History';
import Cron from './components/Cron';
import MCPServers from './components/MCPServers';
import { SessionProvider } from './contexts/SessionContext';
import './App.css';

function App() {
  const [activeTab, setActiveTab] = useState('chat');

  const renderContent = () => {
    switch (activeTab) {
      case 'chat':
        return <Chat />;
      case 'history':
        return <History />;
      case 'models':
        return <Models />;
      case 'settings':
        return <GeneralSettings />;
      case 'skills':
        return <Skills />;
      case 'capabilities':
        return <Capabilities />;
      case 'advanced':
        return <AdvancedSettings />;
      case 'monitor':
        return <Monitor />;
      case 'channels':
        return <Channels />;
      case 'usage':
        return <Usage />;
      case 'cron':
        return <Cron />;
      case 'mcp':
        return <MCPServers />;
      default:
        return <Settings />;
    }
  };

  return (
    <SessionProvider>
      <div className="App">
        <Sidebar activeTab={activeTab} onTabChange={setActiveTab} />
        <main className="main-content">
          {renderContent()}
        </main>
      </div>
    </SessionProvider>
  );
}

export default App;
