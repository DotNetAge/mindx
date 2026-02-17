import { useState, KeyboardEvent } from 'react';
import { SendIcon, StopIcon, CloseIcon } from 'tdesign-icons-react';
import CapabilityIcon from './CapabilityIcon';
import './styles/MessageInput.css';

interface Capability {
  name: string;
  title: string;
  icon: string;
  description: string;
  enabled: boolean;
}

interface MessageInputProps {
  onSend: (message: string) => void;
  isLoading?: boolean;
  onStop?: () => void;
  capabilities?: Capability[];
  selectedCapability?: Capability | null;
  onSelectCapability?: (capability: Capability) => void;
  onRemoveCapability?: () => void;
}

export default function MessageInput({ 
  onSend, 
  isLoading, 
  onStop, 
  capabilities = [], 
  selectedCapability, 
  onSelectCapability, 
  onRemoveCapability 
}: MessageInputProps) {
  const [message, setMessage] = useState('');
  const [showMenu, setShowMenu] = useState(true);

  const handleSend = () => {
    if (message.trim() && !isLoading) {
      onSend(message);
      setMessage('');
    }
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const handleCapabilityClick = (capability: Capability) => {
    onSelectCapability?.(capability);
    setShowMenu(false);
  };

  return (
    <div className="message-input-wrapper">
      <div className="message-input-main-container">
        {selectedCapability && (
        <div className="capability-tag">
          <CapabilityIcon iconName={selectedCapability.icon} className="capability-tag-icon" size={16} />
          <span className="capability-tag-text">{selectedCapability.title}</span>
          <button 
            className="capability-tag-close"
            onClick={() => {
              onRemoveCapability?.();
              setShowMenu(true);
            }}
          >
            <CloseIcon size={14} />
          </button>
        </div>
      )}
        
        <div className="message-input-container">
          <textarea
            className="message-input"
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="输入消息... (按 Enter 发送，Shift + Enter 换行)"
            disabled={isLoading}
          />
          <button
            className="send-button"
            onClick={isLoading ? onStop : handleSend}
            disabled={!message.trim() && !isLoading}
            title={isLoading ? '停止生成' : '发送消息'}
          >
            {isLoading ? <StopIcon size={18} /> : <SendIcon size={18} />}
          </button>
        </div>
        
        {showMenu && capabilities.length > 0 && !selectedCapability && (
          <div className="capability-menu">
            <div className="capability-menu-scroll">
              {capabilities.map((capability) => (
              <button
                key={capability.name}
                className="capability-item"
                onClick={() => handleCapabilityClick(capability)}
                title={capability.description}
              >
                <CapabilityIcon iconName={capability.icon} className="capability-item-icon" size={16} />
                {capability.title}
              </button>
            ))}
            </div>
          </div>
        )}
      </div>
      
      <div className="input-footer">
        <span className="footer-hint">
          按 <kbd>Enter</kbd> 发送，<kbd>Shift</kbd> + <kbd>Enter</kbd> 换行
        </span>
        {!showMenu && !selectedCapability && (
          <button 
            className="show-menu-btn"
            onClick={() => setShowMenu(true)}
          >
            显示能力
          </button>
        )}
        {showMenu && capabilities.length > 0 && !selectedCapability && (
          <button 
            className="hide-menu-btn"
            onClick={() => setShowMenu(false)}
          >
            隐藏
          </button>
        )}
      </div>
    </div>
  );
}
