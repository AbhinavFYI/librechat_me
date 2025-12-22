import { useState, memo } from 'react';
import { GearIcon } from '@librechat/client';
import { useLocalize } from '~/hooks';
import Settings from './Settings';

function NavSettingsButton() {
  const localize = useLocalize();
  const [showSettings, setShowSettings] = useState(false);

  return (
    <>
      <button
        onClick={() => setShowSettings(true)}
        className="mt-text-sm flex h-auto w-full items-center gap-2 rounded-xl p-2 text-sm transition-all duration-200 ease-in-out hover:bg-surface-hover"
        aria-label={localize('com_nav_settings')}
      >
        <GearIcon className="icon-md text-text-primary" aria-hidden="true" />
        <span className="grow overflow-hidden text-ellipsis whitespace-nowrap text-left text-text-primary">
          {localize('com_nav_settings')}
        </span>
      </button>
      {showSettings && <Settings open={showSettings} onOpenChange={setShowSettings} />}
    </>
  );
}

export default memo(NavSettingsButton);

