import { useState, useMemo, useEffect } from 'react';
import { Lightbulb, ChevronDown } from 'lucide-react';
import * as Ariakit from '@ariakit/react';
import { DropdownPopup } from '@librechat/client';
import { useGetStartupConfig } from '~/data-provider';
import { useMCPServerManager } from '~/hooks';
import { useParams } from 'react-router-dom';
import { Constants } from 'librechat-data-provider';

export default function MCPServerSelector() {
  const { data: startupConfig } = useGetStartupConfig();
  const { conversationId } = useParams<{ conversationId?: string }>();
  const mcpServerManager = useMCPServerManager({ conversationId: conversationId || null });
  const [isOpen, setIsOpen] = useState(false);
  
  const { configuredServers, mcpValues, batchToggleServers } = mcpServerManager;

  // Debug: Log startup config to see what's being loaded
  useEffect(() => {
    console.log('[MCP Selector] Startup config:', startupConfig);
    console.log('[MCP Selector] mcpServers from config:', startupConfig?.mcpServers);
    if (startupConfig?.mcpServers) {
      console.log('[MCP Selector] MCP server keys:', Object.keys(startupConfig.mcpServers));
      Object.entries(startupConfig.mcpServers).forEach(([key, value]) => {
        console.log(`[MCP Selector] Server ${key}:`, value);
      });
    }
  }, [startupConfig]);

  const availableServers = useMemo(() => {
    if (!startupConfig?.mcpServers) {
      console.log('[MCP Selector] No MCP servers in startup config');
      console.log('[MCP Selector] Full startupConfig:', startupConfig);
      return [];
    }
    // Show all MCP servers that have chatMenu !== false (or undefined, which means true)
    const servers = Object.keys(startupConfig.mcpServers).filter(
      (server) => {
        const serverConfig = startupConfig.mcpServers?.[server];
        const shouldShow = serverConfig && serverConfig.chatMenu !== false;
        console.log(`[MCP Selector] Server ${server}: chatMenu=${serverConfig?.chatMenu}, shouldShow=${shouldShow}`);
        return shouldShow;
      }
    );
    console.log('[MCP Selector] Available servers after filtering:', servers);
    return servers;
  }, [startupConfig?.mcpServers]);

  const handleToggleServer = (serverName: string) => {
    const currentValues = mcpValues || [];
    if (currentValues.includes(serverName)) {
      // Remove server
      batchToggleServers(currentValues.filter((s) => s !== serverName));
      console.log(`[MCP Selector] Removed server: ${serverName}`);
    } else {
      // Add server - batchToggleServers will handle initialization if needed
      // For SSE servers configured in YAML, they should work directly
      batchToggleServers([...currentValues, serverName]);
      console.log(`[MCP Selector] Added server: ${serverName}`);
    }
  };

  const handleClearAll = () => {
    // Clear all selected MCP servers
    batchToggleServers([]);
    console.log('[MCP Selector] Cleared all servers');
  };

  const menuItems = [
    ...availableServers.map((serverName) => ({
    label: serverName,
    onClick: () => handleToggleServer(serverName),
    icon: mcpValues?.includes(serverName) ? '✓' : '',
      key: `mcp-${serverName}`,
    })),
    // Add separator and Clear All option if any servers are selected
    ...(mcpValues && mcpValues.length > 0 ? [
      { separate: true, key: 'separator' },
      {
        label: '🗑️ Clear All',
        onClick: handleClearAll,
        key: 'clear-all-mcp',
      }
    ] : []),
  ];

  const selectedCount = mcpValues?.length || 0;
  const displayText = selectedCount === 0 
    ? "MCP's" 
    : selectedCount === 1 
      ? mcpValues[0] 
      : `${selectedCount} selected`;

  // Always show the selector, even if no servers are configured yet (for debugging)
  // This helps identify if the issue is with server configuration or component rendering
  if (availableServers.length === 0) {
    console.warn('[MCP Selector] No MCP servers available. Startup config:', startupConfig?.mcpServers);
    // Still render the button but disabled to show it exists
    return (
      <button
        type="button"
        disabled
        className="flex items-center gap-1.5 rounded-lg border border-border-light bg-transparent px-3 py-2 text-sm font-medium text-text-primary opacity-50"
        title="No MCP servers configured"
      >
        <Lightbulb className="h-4 w-4" />
        <span>MCP's (No servers)</span>
      </button>
    );
  }

  console.log('[MCP Selector] Rendering dropdown. isOpen:', isOpen, 'menuItems:', menuItems.length, menuItems);

  return (
    <div className="relative">
      <DropdownPopup
        portal={false}
        menuId="mcp-server-selector"
        isOpen={isOpen}
        setIsOpen={setIsOpen}
        trigger={
          <Ariakit.MenuButton
            className="flex items-center gap-1.5 rounded-lg border border-border-light bg-transparent px-3 py-2 text-sm font-medium text-text-primary transition-all hover:bg-surface-hover"
          >
            <Lightbulb className="h-4 w-4" />
            <span>{displayText}</span>
            <ChevronDown className="h-4 w-4" />
          </Ariakit.MenuButton>
        }
        items={menuItems}
        className="absolute left-0 top-full mt-2 min-w-[200px] z-50"
      />
    </div>
  );
}

