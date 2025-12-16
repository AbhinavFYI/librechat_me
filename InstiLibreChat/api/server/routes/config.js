const express = require('express');
const { logger } = require('@librechat/data-schemas');
const { isEnabled, getBalanceConfig } = require('@librechat/api');
const {
  Constants,
  CacheKeys,
  removeNullishValues,
  defaultSocialLogins,
} = require('librechat-data-provider');
const { getLdapConfig } = require('~/server/services/Config/ldap');
const { getAppConfig } = require('~/server/services/Config/app');
const { getProjectByName } = require('~/models/Project');
const { getMCPManager } = require('~/config');
const { getLogStores } = require('~/cache');
const { mcpServersRegistry } = require('@librechat/api');

const router = express.Router();
const emailLoginEnabled =
  process.env.ALLOW_EMAIL_LOGIN === undefined || isEnabled(process.env.ALLOW_EMAIL_LOGIN);
const passwordResetEnabled = isEnabled(process.env.ALLOW_PASSWORD_RESET);

const sharedLinksEnabled =
  process.env.ALLOW_SHARED_LINKS === undefined || isEnabled(process.env.ALLOW_SHARED_LINKS);

const publicSharedLinksEnabled =
  sharedLinksEnabled &&
  (process.env.ALLOW_SHARED_LINKS_PUBLIC === undefined ||
    isEnabled(process.env.ALLOW_SHARED_LINKS_PUBLIC));

const sharePointFilePickerEnabled = isEnabled(process.env.ENABLE_SHAREPOINT_FILEPICKER);
const openidReuseTokens = isEnabled(process.env.OPENID_REUSE_TOKENS);

/**
 * Fetches MCP servers from registry and adds them to the payload.
 * Registry now includes all configured servers (from YAML) plus inspection data when available.
 * Always fetches fresh to avoid caching incomplete initialization state.
 * Also includes servers directly from mcpConfig (YAML) even if not yet initialized.
 */
const getMCPServers = async (payload, appConfig) => {
  try {
    logger.info('[getMCPServers] Starting...');
    logger.info(`[getMCPServers] appConfig exists: ${!!appConfig}`);
    
    // Check mcpConfig value (could be null even if key exists)
    const mcpConfigValue = appConfig?.mcpConfig;
    logger.info(`[getMCPServers] appConfig.mcpConfig value: ${mcpConfigValue === null ? 'null' : mcpConfigValue === undefined ? 'undefined' : typeof mcpConfigValue}`);
    logger.info(`[getMCPServers] appConfig.mcpConfig is truthy: ${!!mcpConfigValue}`);
    
    // Try multiple ways to get mcpConfig
    let mcpConfig = mcpConfigValue;
    
    // If mcpConfig is null/undefined, try config.mcpServers
    if (!mcpConfig && appConfig?.config?.mcpServers) {
      logger.info('[getMCPServers] Using mcpServers from appConfig.config as fallback');
      mcpConfig = appConfig.config.mcpServers;
    }
    
    // If still not found, check if config itself has mcpServers at root level
    if (!mcpConfig && appConfig?.config) {
      logger.info(`[getMCPServers] Checking appConfig.config structure. Keys: ${Object.keys(appConfig.config).join(', ')}`);
      // The config object might have mcpServers directly
      if (appConfig.config.mcpServers) {
        logger.info('[getMCPServers] Found mcpServers in appConfig.config');
        mcpConfig = appConfig.config.mcpServers;
      }
    }
    
    if (!mcpConfig) {
      logger.warn('[getMCPServers] No mcpConfig found. appConfig structure:');
      logger.warn(`[getMCPServers] appConfig keys: ${appConfig ? Object.keys(appConfig).join(', ') : 'appConfig is null'}`);
      if (appConfig?.config) {
        logger.warn(`[getMCPServers] appConfig.config keys: ${Object.keys(appConfig.config).join(', ')}`);
        logger.warn(`[getMCPServers] appConfig.config.mcpServers: ${appConfig.config.mcpServers}`);
      }
      // Log the actual mcpConfig value even if null
      logger.warn(`[getMCPServers] appConfig.mcpConfig raw value: ${JSON.stringify(mcpConfigValue)}`);
      return;
    }
    
    // Initialize payload.mcpServers if it doesn't exist
    if (!payload.mcpServers) {
      payload.mcpServers = {};
    }
    
    // First, add all servers from mcpConfig (YAML) - these are the configured servers
    // This ensures all configured servers appear even if not yet initialized
    if (mcpConfig) {
      const serverNames = Object.keys(mcpConfig);
      logger.info(`[getMCPServers] Found ${serverNames.length} servers in mcpConfig: ${serverNames.join(', ')}`);
      
      for (const serverName in mcpConfig) {
        const yamlConfig = mcpConfig[serverName];
        // Only add if not already present (registry takes precedence)
        if (!payload.mcpServers[serverName]) {
          payload.mcpServers[serverName] = removeNullishValues({
            startup: yamlConfig?.startup,
            chatMenu: yamlConfig?.chatMenu !== false, // Default to true if not specified
            isOAuth: yamlConfig?.requiresOAuth || false,
            customUserVars: yamlConfig?.customUserVars || {},
          });
          logger.info(`[getMCPServers] Added server "${serverName}" from mcpConfig`);
        }
      }
      logger.info(`[getMCPServers] Added ${Object.keys(mcpConfig).length} servers from mcpConfig`);
    }
    
    // Then, try to get additional info from registry (if servers are initialized)
    const mcpManager = getMCPManager();
    if (mcpManager) {
      try {
        const mcpServers = await mcpServersRegistry.getAllServerConfigs();
        if (mcpServers) {
          // Update with registry data (has more info like connection status)
          for (const serverName in mcpServers) {
            const serverConfig = mcpServers[serverName];
            payload.mcpServers[serverName] = {
              ...payload.mcpServers[serverName],
              ...removeNullishValues({
                startup: serverConfig?.startup,
                chatMenu: serverConfig?.chatMenu !== false,
                isOAuth: serverConfig.requiresOAuth,
                customUserVars: serverConfig?.customUserVars,
              }),
            };
          }
          logger.info(`[getMCPServers] Updated with ${Object.keys(mcpServers).length} servers from registry`);
        }
      } catch (registryError) {
        logger.warn('[getMCPServers] Error getting servers from registry (non-fatal):', registryError);
        // Continue - we already have servers from mcpConfig
      }
    }
    
    const finalCount = Object.keys(payload.mcpServers).length;
    logger.info(`[getMCPServers] Final mcpServers count: ${finalCount}`);
    logger.info(`[getMCPServers] Final server names: ${Object.keys(payload.mcpServers).join(', ')}`);
  } catch (error) {
    logger.error('[getMCPServers] Error loading MCP servers:', error);
    logger.error('[getMCPServers] Error stack:', error.stack);
  }
};

router.get('/', async function (req, res) {
  const cache = getLogStores(CacheKeys.CONFIG_STORE);

  const cachedStartupConfig = await cache.get(CacheKeys.STARTUP_CONFIG);
  if (cachedStartupConfig) {
    logger.info('[config route] Using cached startup config, adding MCP servers...');
    const appConfig = await getAppConfig({ role: req.user?.role });
    await getMCPServers(cachedStartupConfig, appConfig);
    // Send the updated config with MCP servers
    res.send(cachedStartupConfig);
    return;
  }

  const isBirthday = () => {
    const today = new Date();
    return today.getMonth() === 1 && today.getDate() === 11;
  };

  const instanceProject = await getProjectByName(Constants.GLOBAL_PROJECT_NAME, '_id');

  const ldap = getLdapConfig();

  try {
    const appConfig = await getAppConfig({ role: req.user?.role });

    const isOpenIdEnabled =
      !!process.env.OPENID_CLIENT_ID &&
      !!process.env.OPENID_CLIENT_SECRET &&
      !!process.env.OPENID_ISSUER &&
      !!process.env.OPENID_SESSION_SECRET;

    const isSamlEnabled =
      !!process.env.SAML_ENTRY_POINT &&
      !!process.env.SAML_ISSUER &&
      !!process.env.SAML_CERT &&
      !!process.env.SAML_SESSION_SECRET;

    const balanceConfig = getBalanceConfig(appConfig);

    /** @type {TStartupConfig} */
    const payload = {
      appTitle: process.env.APP_TITLE || 'LibreChat',
      socialLogins: appConfig?.registration?.socialLogins ?? defaultSocialLogins,
      discordLoginEnabled: !!process.env.DISCORD_CLIENT_ID && !!process.env.DISCORD_CLIENT_SECRET,
      facebookLoginEnabled:
        !!process.env.FACEBOOK_CLIENT_ID && !!process.env.FACEBOOK_CLIENT_SECRET,
      githubLoginEnabled: !!process.env.GITHUB_CLIENT_ID && !!process.env.GITHUB_CLIENT_SECRET,
      googleLoginEnabled: !!process.env.GOOGLE_CLIENT_ID && !!process.env.GOOGLE_CLIENT_SECRET,
      appleLoginEnabled:
        !!process.env.APPLE_CLIENT_ID &&
        !!process.env.APPLE_TEAM_ID &&
        !!process.env.APPLE_KEY_ID &&
        !!process.env.APPLE_PRIVATE_KEY_PATH,
      openidLoginEnabled: isOpenIdEnabled,
      openidLabel: process.env.OPENID_BUTTON_LABEL || 'Continue with OpenID',
      openidImageUrl: process.env.OPENID_IMAGE_URL,
      openidAutoRedirect: isEnabled(process.env.OPENID_AUTO_REDIRECT),
      samlLoginEnabled: !isOpenIdEnabled && isSamlEnabled,
      samlLabel: process.env.SAML_BUTTON_LABEL,
      samlImageUrl: process.env.SAML_IMAGE_URL,
      serverDomain: process.env.DOMAIN_SERVER || 'http://localhost:3080',
      emailLoginEnabled,
      registrationEnabled: !ldap?.enabled && isEnabled(process.env.ALLOW_REGISTRATION),
      socialLoginEnabled: isEnabled(process.env.ALLOW_SOCIAL_LOGIN),
      emailEnabled:
        (!!process.env.EMAIL_SERVICE || !!process.env.EMAIL_HOST) &&
        !!process.env.EMAIL_USERNAME &&
        !!process.env.EMAIL_PASSWORD &&
        !!process.env.EMAIL_FROM,
      passwordResetEnabled,
      showBirthdayIcon:
        isBirthday() ||
        isEnabled(process.env.SHOW_BIRTHDAY_ICON) ||
        process.env.SHOW_BIRTHDAY_ICON === '',
      helpAndFaqURL: process.env.HELP_AND_FAQ_URL || 'https://librechat.ai',
      interface: appConfig?.interfaceConfig,
      turnstile: appConfig?.turnstileConfig,
      modelSpecs: appConfig?.modelSpecs,
      balance: balanceConfig,
      sharedLinksEnabled,
      publicSharedLinksEnabled,
      analyticsGtmId: process.env.ANALYTICS_GTM_ID,
      instanceProjectId: instanceProject._id.toString(),
      bundlerURL: process.env.SANDPACK_BUNDLER_URL,
      staticBundlerURL: process.env.SANDPACK_STATIC_BUNDLER_URL,
      sharePointFilePickerEnabled,
      sharePointBaseUrl: process.env.SHAREPOINT_BASE_URL,
      sharePointPickerGraphScope: process.env.SHAREPOINT_PICKER_GRAPH_SCOPE,
      sharePointPickerSharePointScope: process.env.SHAREPOINT_PICKER_SHAREPOINT_SCOPE,
      openidReuseTokens,
      conversationImportMaxFileSize: process.env.CONVERSATION_IMPORT_MAX_FILE_SIZE_BYTES
        ? parseInt(process.env.CONVERSATION_IMPORT_MAX_FILE_SIZE_BYTES, 10)
        : 0,
    };

    const minPasswordLength = parseInt(process.env.MIN_PASSWORD_LENGTH, 10);
    if (minPasswordLength && !isNaN(minPasswordLength)) {
      payload.minPasswordLength = minPasswordLength;
    }

    const webSearchConfig = appConfig?.webSearch;
    if (
      webSearchConfig != null &&
      (webSearchConfig.searchProvider ||
        webSearchConfig.scraperProvider ||
        webSearchConfig.rerankerType)
    ) {
      payload.webSearch = {};
    }

    if (webSearchConfig?.searchProvider) {
      payload.webSearch.searchProvider = webSearchConfig.searchProvider;
    }
    if (webSearchConfig?.scraperProvider) {
      payload.webSearch.scraperProvider = webSearchConfig.scraperProvider;
    }
    if (webSearchConfig?.rerankerType) {
      payload.webSearch.rerankerType = webSearchConfig.rerankerType;
    }

    if (ldap) {
      payload.ldap = ldap;
    }

    if (typeof process.env.CUSTOM_FOOTER === 'string') {
      payload.customFooter = process.env.CUSTOM_FOOTER;
    }

    // IMPORTANT: Add MCP servers BEFORE caching, so they're included in the cached version
    await getMCPServers(payload, appConfig);
    
    // Cache the payload with MCP servers included
    await cache.set(CacheKeys.STARTUP_CONFIG, payload);
    return res.status(200).send(payload);
  } catch (err) {
    logger.error('Error in startup config', err);
    return res.status(500).send({ error: err.message });
  }
});

module.exports = router;
