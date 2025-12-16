import { useState, memo, useEffect } from 'react';
import { useRecoilState } from 'recoil';
import * as Select from '@ariakit/react/select';
import { FileText, LogOut } from 'lucide-react';
import { LinkIcon, GearIcon, DropdownMenuSeparator, Avatar } from '@librechat/client';
import { useGetStartupConfig, useGetUserBalance } from '~/data-provider';
import FilesView from '~/components/Chat/Input/Files/FilesView';
import { useAuthContext } from '~/hooks/AuthContext';
import { useLocalize } from '~/hooks';
import { saasApi } from '~/services/saasApi';
import Settings from './Settings';
import store from '~/store';

function AccountSettings() {
  const localize = useLocalize();
  const { user, isAuthenticated, logout } = useAuthContext();
  const { data: startupConfig } = useGetStartupConfig();
  const balanceQuery = useGetUserBalance({
    enabled: !!isAuthenticated && startupConfig?.balance?.enabled,
  });
  const [showSettings, setShowSettings] = useState(false);
  const [showFiles, setShowFiles] = useRecoilState(store.showFiles);
  const [userInfo, setUserInfo] = useState<any>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const loadUserInfo = async () => {
      try {
        const token = localStorage.getItem('access_token');
        if (!token) {
          setIsLoading(false);
          return;
        }
        const data = await saasApi.getMe();
        setUserInfo(data);
      } catch (error) {
        console.error('Error loading user info:', error);
      } finally {
        setIsLoading(false);
      }
    };

    if (isAuthenticated) {
      loadUserInfo();
    } else {
      setIsLoading(false);
    }
  }, [isAuthenticated]);

  // Only show AccountSettings for super admins
  // Check explicitly for true - hide for org admins and regular users
  const isSuperAdmin = userInfo?.is_super_admin === true;

  // Debug log
  useEffect(() => {
    if (!isLoading && userInfo) {
      console.log('[AccountSettings] Render check:', {
        isSuperAdmin,
        is_super_admin: userInfo?.is_super_admin,
        org_role: userInfo?.org_role,
        org_id: userInfo?.org_id,
        willShow: isSuperAdmin
      });
    }
  }, [isLoading, userInfo, isSuperAdmin]);

  // Don't show while loading
  if (isLoading) {
    return null;
  }

  // Don't show if userInfo is not loaded
  if (!userInfo) {
    return null;
  }

  // Only show if explicitly a super admin (is_super_admin === true)
  // Hide for org admins (org_role === 'admin' but is_super_admin !== true) and regular users
  if (!isSuperAdmin) {
    return null;
  }

  return (
    <Select.SelectProvider>
      <Select.Select
        aria-label={localize('com_nav_account_settings')}
        data-testid="nav-user"
        className="mt-text-sm flex h-auto w-full items-center gap-2 rounded-xl p-2 text-sm transition-all duration-200 ease-in-out hover:bg-surface-hover"
      >
        <div className="-ml-0.9 -mt-0.8 h-8 w-8 flex-shrink-0">
          <div className="relative flex">
            <Avatar user={user} size={32} />
          </div>
        </div>
        <div
          className="mt-2 grow overflow-hidden text-ellipsis whitespace-nowrap text-left text-text-primary"
          style={{ marginTop: '0', marginLeft: '0' }}
        >
          {user?.name ?? user?.username ?? localize('com_nav_user')}
        </div>
      </Select.Select>
      <Select.SelectPopover
        className="popover-ui w-[235px]"
        style={{
          transformOrigin: 'bottom',
          marginRight: '0px',
          translate: '0px',
        }}
      >
        <div className="text-token-text-secondary ml-3 mr-2 py-2 text-sm" role="note">
          {user?.email ?? localize('com_nav_user')}
        </div>
        <DropdownMenuSeparator />
        {startupConfig?.balance?.enabled === true && balanceQuery.data != null && (
          <>
            <div className="text-token-text-secondary ml-3 mr-2 py-2 text-sm" role="note">
              {localize('com_nav_balance')}:{' '}
              {new Intl.NumberFormat().format(Math.round(balanceQuery.data.tokenCredits))}
            </div>
            <DropdownMenuSeparator />
          </>
        )}
        <Select.SelectItem
          value=""
          onClick={() => setShowFiles(true)}
          className="select-item text-sm"
        >
          <FileText className="icon-md" aria-hidden="true" />
          {localize('com_nav_my_files')}
        </Select.SelectItem>
        {startupConfig?.helpAndFaqURL !== '/' && (
          <Select.SelectItem
            value=""
            onClick={() => window.open(startupConfig?.helpAndFaqURL, '_blank')}
            className="select-item text-sm"
          >
            <LinkIcon aria-hidden="true" />
            {localize('com_nav_help_faq')}
          </Select.SelectItem>
        )}
        {isSuperAdmin && (
          <>
        <Select.SelectItem
          value=""
          onClick={() => setShowSettings(true)}
          className="select-item text-sm"
        >
          <GearIcon className="icon-md" aria-hidden="true" />
          {localize('com_nav_settings')}
        </Select.SelectItem>
        <Select.SelectItem
          value=""
          onClick={() => {
            const baseHref = document.querySelector('base')?.getAttribute('href') || '/';
            window.location.href = `${baseHref}admin`;
          }}
          className="select-item text-sm"
        >
          <GearIcon className="icon-md" aria-hidden="true" />
          Admin Dashboard
        </Select.SelectItem>
          </>
        )}
        <DropdownMenuSeparator />
        <Select.SelectItem
          aria-selected={true}
          onClick={() => logout()}
          value="logout"
          className="select-item text-sm"
        >
          <LogOut className="icon-md" />
          {localize('com_nav_log_out')}
        </Select.SelectItem>
      </Select.SelectPopover>
      {showFiles && <FilesView open={showFiles} onOpenChange={setShowFiles} />}
      {showSettings && <Settings open={showSettings} onOpenChange={setShowSettings} />}
    </Select.SelectProvider>
  );
}

export default memo(AccountSettings);
