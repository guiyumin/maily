import React from 'react';
import { PanelRightClose, PanelRightOpen } from 'lucide-react';
import { useLocale } from '@calendar/locale';
import { Button } from '@/components/ui/button';

interface SidebarHeaderProps {
  isCollapsed: boolean;
  onCollapseToggle: () => void;
}

export const SidebarHeader: React.FC<SidebarHeaderProps> = ({
  isCollapsed,
  onCollapseToggle,
}) => {
  const { t } = useLocale();
  return (
    <div className="flex items-center px-2 py-2">
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={isCollapsed ? t('expandSidebar') : t('collapseSidebar')}
        onClick={onCollapseToggle}
      >
        {isCollapsed ? (
          <PanelRightClose className="h-4 w-4 text-gray-500 dark:text-gray-400" />
        ) : (
          <PanelRightOpen className="h-4 w-4 text-gray-500 dark:text-gray-400" />
        )}
      </Button>
      {!isCollapsed && (
        <div className='flex flex-1 justify-between items-center'>
          <span className="text-sm font-semibold text-gray-700 dark:text-gray-200">
            {t('calendars')}
          </span>
        </div>
      )}
    </div>
  );
};
