const { CacheKeys } = require('librechat-data-provider');
const { logger, AppService } = require('@librechat/data-schemas');
const { loadAndFormatTools } = require('~/server/services/start/tools');
const loadCustomConfig = require('./loadCustomConfig');
const { setCachedTools } = require('./getCachedTools');
const getLogStores = require('~/cache/getLogStores');
const paths = require('~/config/paths');

const BASE_CONFIG_KEY = '_BASE_';

const loadBaseConfig = async () => {
  /** @type {TCustomConfig} */
  const config = (await loadCustomConfig()) ?? {};
  
  // If config validation failed but we still want to load mcpServers,
  // try to read it directly from YAML as a fallback
  if (!config.mcpServers) {
    try {
      const { loadYaml } = require('@librechat/api');
      const projectRoot = require('path').resolve(__dirname, '..', '..', '..', '..');
      const configPath = process.env.CONFIG_PATH || require('path').resolve(projectRoot, 'librechat.yaml');
      const rawConfig = loadYaml(configPath);
      if (rawConfig && rawConfig.mcpServers) {
        logger.info('[loadBaseConfig] Loading mcpServers from raw YAML as fallback');
        config.mcpServers = rawConfig.mcpServers;
      }
    } catch (error) {
      logger.debug('[loadBaseConfig] Could not load mcpServers from raw YAML:', error.message);
    }
  }
  
  /** @type {Record<string, FunctionTool>} */
  const systemTools = loadAndFormatTools({
    adminFilter: config.filteredTools,
    adminIncluded: config.includedTools,
    directory: paths.structuredTools,
  });
  return AppService({ config, paths, systemTools });
};

/**
 * Get the app configuration based on user context
 * @param {Object} [options]
 * @param {string} [options.role] - User role for role-based config
 * @param {boolean} [options.refresh] - Force refresh the cache
 * @returns {Promise<AppConfig>}
 */
async function getAppConfig(options = {}) {
  const { role, refresh } = options;

  const cache = getLogStores(CacheKeys.APP_CONFIG);
  const cacheKey = role ? role : BASE_CONFIG_KEY;

  if (!refresh) {
    const cached = await cache.get(cacheKey);
    if (cached) {
      return cached;
    }
  }

  let baseConfig = await cache.get(BASE_CONFIG_KEY);
  if (!baseConfig) {
    logger.info('[getAppConfig] App configuration not initialized. Initializing AppService...');
    baseConfig = await loadBaseConfig();

    if (!baseConfig) {
      throw new Error('Failed to initialize app configuration through AppService.');
    }

    if (baseConfig.availableTools) {
      await setCachedTools(baseConfig.availableTools);
    }

    await cache.set(BASE_CONFIG_KEY, baseConfig);
  }

  // For now, return the base config
  // In the future, this is where we'll apply role-based modifications
  if (role) {
    // TODO: Apply role-based config modifications
    // const roleConfig = await applyRoleBasedConfig(baseConfig, role);
    // await cache.set(cacheKey, roleConfig);
    // return roleConfig;
  }

  return baseConfig;
}

/**
 * Clear the app configuration cache
 * @returns {Promise<boolean>}
 */
async function clearAppConfigCache() {
  const cache = getLogStores(CacheKeys.CONFIG_STORE);
  const cacheKey = CacheKeys.APP_CONFIG;
  return await cache.delete(cacheKey);
}

module.exports = {
  getAppConfig,
  clearAppConfigCache,
};
