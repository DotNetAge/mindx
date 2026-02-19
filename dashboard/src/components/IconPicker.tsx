import { useState, useMemo } from 'react';
import { Icon } from '@iconify/react';
import './IconPicker.css';

const ANT_DESIGN_ICONS = [
  'ant-design:home-outlined',
  'ant-design:user-outlined',
  'ant-design:setting-outlined',
  'ant-design:search-outlined',
  'ant-design:edit-outlined',
  'ant-design:delete-outlined',
  'ant-design:plus-outlined',
  'ant-design:minus-outlined',
  'ant-design:check-outlined',
  'ant-design:close-outlined',
  'ant-design:arrow-left-outlined',
  'ant-design:arrow-right-outlined',
  'ant-design:arrow-up-outlined',
  'ant-design:arrow-down-outlined',
  'ant-design:heart-outlined',
  'ant-design:star-outlined',
  'ant-design:message-outlined',
  'ant-design:notification-outlined',
  'ant-design:mail-outlined',
  'ant-design:phone-outlined',
  'ant-design:calendar-outlined',
  'ant-design:clock-circle-outlined',
  'ant-design:folder-outlined',
  'ant-design:file-outlined',
  'ant-design:picture-outlined',
  'ant-design:camera-outlined',
  'ant-design:video-camera-outlined',
  'ant-design:music-outlined',
  'ant-design:link-outlined',
  'ant-design:share-alt-outlined',
  'ant-design:download-outlined',
  'ant-design:upload-outlined',
  'ant-design:cloud-outlined',
  'ant-design:database-outlined',
  'ant-design:code-outlined',
  'ant-design:api-outlined',
  'ant-design:robot-outlined',
  'ant-design:bulb-outlined',
  'ant-design:thunderbolt-outlined',
  'ant-design:rocket-outlined',
  'ant-design:fire-outlined',
  'ant-design:gift-outlined',
  'ant-design:lock-outlined',
  'ant-design:unlock-outlined',
  'ant-design:eye-outlined',
  'ant-design:eye-invisible-outlined',
  'ant-design:filter-outlined',
  'ant-design:sync-outlined',
  'ant-design:history-outlined',
  'ant-design:book-outlined',
  'ant-design:bookmark-outlined',
  'ant-design:tag-outlined',
  'ant-design:tags-outlined',
  'ant-design:highlight-outlined',
  'ant-design:experiment-outlined',
  'ant-design:tool-outlined',
  'ant-design:build-outlined',
  'ant-design:layout-outlined',
  'ant-design:table-outlined',
  'ant-design:profile-outlined',
  'ant-design:team-outlined',
  'ant-design:usergroup-add-outlined',
  'ant-design:pie-chart-outlined',
  'ant-design:bar-chart-outlined',
  'ant-design:line-chart-outlined',
  'ant-design:area-chart-outlined',
  'ant-design:fund-outlined',
  'ant-design:trophy-outlined',
  'ant-design:gold-outlined',
  'ant-design:shopping-cart-outlined',
  'ant-design:shopping-outlined',
  'ant-design:credit-card-outlined',
  'ant-design:wallet-outlined',
  'ant-design:dollar-outlined',
  'ant-design:euro-outlined',
  'ant-design:environment-outlined',
  'ant-design:compass-outlined',
  'ant-design:global-outlined',
  'ant-design:earth-outlined',
  'ant-design:flag-outlined',
  'ant-design:language-outlined',
  'ant-design:translation-outlined',
  'ant-design:info-circle-outlined',
  'ant-design:question-circle-outlined',
  'ant-design:exclamation-circle-outlined',
  'ant-design:check-circle-outlined',
  'ant-design:close-circle-outlined',
  'ant-design:warning-outlined',
  'ant-design:alert-outlined',
  'ant-design:bell-outlined',
  'ant-design:sound-outlined',
  'ant-design:mute-outlined',
  'ant-design:play-circle-outlined',
  'ant-design:pause-circle-outlined',
  'ant-design:fullscreen-outlined',
  'ant-design:fullscreen-exit-outlined',
  'ant-design:zoom-in-outlined',
  'ant-design:zoom-out-outlined',
];

interface IconPickerProps {
  value?: string;
  onChange: (icon: string) => void;
  placeholder?: string;
}

export default function IconPicker({ value, onChange, placeholder = '选择图标' }: IconPickerProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [searchText, setSearchText] = useState('');

  const filteredIcons = useMemo(() => {
    if (!searchText) return ANT_DESIGN_ICONS;
    const search = searchText.toLowerCase();
    return ANT_DESIGN_ICONS.filter(icon => 
      icon.toLowerCase().includes(search)
    );
  }, [searchText]);

  const displayValue = value || '';
  const isIconifyIcon = displayValue.includes(':');

  return (
    <div className="icon-picker-wrapper">
      <div 
        className="icon-picker-trigger"
        onClick={() => setIsOpen(!isOpen)}
      >
        <div className="icon-picker-input">
          {displayValue ? (
            <div className="icon-picker-selected">
              {isIconifyIcon ? (
                <Icon icon={displayValue} width={20} height={20} />
              ) : (
                <span className="icon-picker-text">{displayValue}</span>
              )}
              <span className="icon-picker-value">{displayValue}</span>
            </div>
          ) : (
            <span className="icon-picker-placeholder">{placeholder}</span>
          )}
        </div>
        <span className="icon-picker-arrow">▼</span>
      </div>

      {isOpen && (
        <>
          <div className="icon-picker-overlay" onClick={() => setIsOpen(false)} />
          <div className="icon-picker-dropdown">
            <div className="icon-picker-search">
              <input
                type="text"
                placeholder="搜索图标..."
                value={searchText}
                onChange={(e) => setSearchText(e.target.value)}
                onClick={(e) => e.stopPropagation()}
              />
            </div>
            <div className="icon-picker-grid">
              {filteredIcons.map((icon) => (
                <button
                  key={icon}
                  className={`icon-picker-item ${value === icon ? 'selected' : ''}`}
                  onClick={() => {
                    onChange(icon);
                    setIsOpen(false);
                  }}
                  title={icon}
                >
                  <Icon icon={icon} width={24} height={24} />
                </button>
              ))}
            </div>
            {filteredIcons.length === 0 && (
              <div className="icon-picker-empty">未找到匹配的图标</div>
            )}
          </div>
        </>
      )}
    </div>
  );
}
