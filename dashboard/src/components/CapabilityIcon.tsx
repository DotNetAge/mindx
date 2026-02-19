import { Icon } from '@iconify/react';
import {
  ChatIcon,
  ToolsIcon,
  InfoCircleFilledIcon,
  ShareIcon,
  ChartIcon,
  SettingIcon,
  RefreshIcon,
  EditIcon,
  DeleteIcon,
} from 'tdesign-icons-react';

const iconMap: Record<string, React.ComponentType<any>> = {
  ChatIcon,
  ToolsIcon,
  InfoCircleFilledIcon,
  ShareIcon,
  ChartIcon,
  SettingIcon,
  RefreshIcon,
  EditIcon,
  DeleteIcon,
};

interface CapabilityIconProps {
  iconName: string;
  size?: number;
  className?: string;
}

export default function CapabilityIcon({ iconName, size = 18, className = '' }: CapabilityIconProps) {
  if (!iconName) {
    return null;
  }

  const IconComponent = iconMap[iconName];
  
  if (IconComponent) {
    return <IconComponent size={size} className={className} />;
  }
  
  if (iconName.includes(':')) {
    return <Icon icon={iconName} width={size} height={size} className={className} />;
  }
  
  return <span className={className}>{iconName}</span>;
}
