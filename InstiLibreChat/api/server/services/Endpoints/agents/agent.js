const { Providers } = require('@librechat/agents');
const {
  primeResources,
  getModelMaxTokens,
  extractLibreChatParams,
  filterFilesByEndpointConfig,
  optionalChainWithEmptyCheck,
} = require('@librechat/api');
const {
  ErrorTypes,
  EModelEndpoint,
  EToolResources,
  paramEndpoints,
  isAgentsEndpoint,
  replaceSpecialVars,
  providerEndpointMap,
} = require('librechat-data-provider');
const generateArtifactsPrompt = require('~/app/clients/prompts/artifacts');
const { getProviderConfig } = require('~/server/services/Endpoints');
const { processFiles } = require('~/server/services/Files/process');
const { getFiles, getToolFilesByIds } = require('~/models/File');
const { getConvoFiles } = require('~/models/Conversation');

/**
 * @param {object} params
 * @param {ServerRequest} params.req
 * @param {ServerResponse} params.res
 * @param {Agent} params.agent
 * @param {string | null} [params.conversationId]
 * @param {Array<IMongoFile>} [params.requestFiles]
 * @param {typeof import('~/server/services/ToolService').loadAgentTools | undefined} [params.loadTools]
 * @param {TEndpointOption} [params.endpointOption]
 * @param {Set<string>} [params.allowedProviders]
 * @param {boolean} [params.isInitialAgent]
 * @returns {Promise<Agent & {
 * tools: StructuredTool[],
 * attachments: Array<MongoFile>,
 * toolContextMap: Record<string, unknown>,
 * maxContextTokens: number,
 * userMCPAuthMap?: Record<string, Record<string, string>>
 * }>}
 */
const initializeAgent = async ({
  req,
  res,
  agent,
  loadTools,
  requestFiles,
  conversationId,
  endpointOption,
  allowedProviders,
  isInitialAgent = false,
}) => {
  const appConfig = req.config;
  if (
    isAgentsEndpoint(endpointOption?.endpoint) &&
    allowedProviders.size > 0 &&
    !allowedProviders.has(agent.provider)
  ) {
    throw new Error(
      `{ "type": "${ErrorTypes.INVALID_AGENT_PROVIDER}", "info": "${agent.provider}" }`,
    );
  }
  let currentFiles;

  const _modelOptions = structuredClone(
    Object.assign(
      { model: agent.model },
      agent.model_parameters ?? { model: agent.model },
      isInitialAgent === true ? endpointOption?.model_parameters : {},
    ),
  );

  const { resendFiles, maxContextTokens, modelOptions } = extractLibreChatParams(_modelOptions);

  const provider = agent.provider;
  agent.endpoint = provider;

  if (isInitialAgent && conversationId != null && resendFiles) {
    const fileIds = (await getConvoFiles(conversationId)) ?? [];
    /** @type {Set<EToolResources>} */
    const toolResourceSet = new Set();
    for (const tool of agent.tools) {
      if (EToolResources[tool]) {
        toolResourceSet.add(EToolResources[tool]);
      }
    }
    const toolFiles = await getToolFilesByIds(fileIds, toolResourceSet);
    if (requestFiles.length || toolFiles.length) {
      currentFiles = await processFiles(requestFiles.concat(toolFiles));
    }
  } else if (isInitialAgent && requestFiles.length) {
    currentFiles = await processFiles(requestFiles);
  }

  if (currentFiles && currentFiles.length) {
    let endpointType;
    if (!paramEndpoints.has(agent.endpoint)) {
      endpointType = EModelEndpoint.custom;
    }

    currentFiles = filterFilesByEndpointConfig(req, {
      files: currentFiles,
      endpoint: agent.endpoint,
      endpointType,
    });
  }

  const { attachments, tool_resources } = await primeResources({
    req,
    getFiles,
    appConfig,
    agentId: agent.id,
    attachments: currentFiles,
    tool_resources: agent.tool_resources,
    requestFileSet: new Set(requestFiles?.map((file) => file.file_id)),
  });

  const {
    tools: structuredTools,
    toolContextMap,
    userMCPAuthMap,
  } = (await loadTools?.({
    req,
    res,
    provider,
    agentId: agent.id,
    tools: agent.tools,
    model: agent.model,
    tool_resources,
  })) ?? {};

  const { getOptions, overrideProvider } = getProviderConfig({ provider, appConfig });
  if (overrideProvider !== agent.provider) {
    agent.provider = overrideProvider;
  }

  const _endpointOption =
    isInitialAgent === true
      ? Object.assign({}, endpointOption, { model_parameters: modelOptions })
      : { model_parameters: modelOptions };

  const options = await getOptions({
    req,
    res,
    optionsOnly: true,
    overrideEndpoint: provider,
    overrideModel: agent.model,
    endpointOption: _endpointOption,
  });

  const tokensModel =
    agent.provider === EModelEndpoint.azureOpenAI ? agent.model : options.llmConfig?.model;
  const maxOutputTokens = optionalChainWithEmptyCheck(
    options.llmConfig?.maxOutputTokens,
    options.llmConfig?.maxTokens,
    0,
  );
  const agentMaxContextTokens = optionalChainWithEmptyCheck(
    maxContextTokens,
    getModelMaxTokens(tokensModel, providerEndpointMap[provider], options.endpointTokenConfig),
    18000,
  );

  if (
    agent.endpoint === EModelEndpoint.azureOpenAI &&
    options.llmConfig?.azureOpenAIApiInstanceName == null
  ) {
    agent.provider = Providers.OPENAI;
  }

  if (options.provider != null) {
    agent.provider = options.provider;
  }

  /** @type {import('@librechat/agents').GenericTool[]} */
  let tools = options.tools?.length ? options.tools : structuredTools;
  if (
    (agent.provider === Providers.GOOGLE || agent.provider === Providers.VERTEXAI) &&
    options.tools?.length &&
    structuredTools?.length
  ) {
    throw new Error(`{ "type": "${ErrorTypes.GOOGLE_TOOL_CONFLICT}"}`);
  } else if (
    (agent.provider === Providers.OPENAI ||
      agent.provider === Providers.AZURE ||
      agent.provider === Providers.ANTHROPIC) &&
    options.tools?.length &&
    structuredTools?.length
  ) {
    tools = structuredTools.concat(options.tools);
  }

  /** @type {import('@librechat/agents').ClientOptions} */
  agent.model_parameters = { ...options.llmConfig };
  if (options.configOptions) {
    agent.model_parameters.configuration = options.configOptions;
  }

  // Default Financial Analyst System Prompt
  const defaultFinancialPrompt = `You are a senior Financial Analyst and FinTech domain expert with deep expertise in:

- DRHPs (Draft Red Herring Prospectus)
- Annual Reports
- Quarterly & Yearly Financial Statements
- Balance Sheets, P&L, Cash Flow Statements
- Earnings Call Transcripts
- Broker / Analyst Reports
- Regulatory Filings (SEBI, MCA, stock exchange filings)

Your primary objective is to extract, analyze, and interpret financial data with institutional-level rigor.

════════════════════════════════════
CORE OPERATING PRINCIPLES
════════════════════════════════════

1. DATA ACCURACY IS NON-NEGOTIABLE
- Never assume or fabricate numbers.
- If a value is not explicitly available, clearly state: "Data not disclosed in the provided document."
- Cross-check figures across sections when possible.
- Maintain consistency in units (₹ crore, ₹ million, %, basis points).

2. VALUE-DRIVEN ANALYSIS (NOT RAW DATA DUMPS)
- Do NOT repeat raw tables unless explicitly asked.
- Convert numbers into: growth rates (YoY, QoQ, CAGR), margins (%), ratios, trend indicators
- Highlight *what the numbers mean*, not just what they are.

3. FINANCIAL INTELLIGENCE OVER SUMMARIZATION
- Always answer: Why does this number matter? What changed? What is improving or deteriorating?
- Compare: historical performance, peer benchmarks (if available), management guidance vs actuals

4. SOURCE-AWARE REASONING
- Explicitly mention the source section when relevant (e.g., DRHP – Financial Information, Note 27, MD&A, Call Transcript Q3 FY25)
- Do not mix management commentary with audited numbers without labeling.

════════════════════════════════════
ANALYSIS EXPECTATIONS BY DOCUMENT TYPE
════════════════════════════════════

▶ DRHP
- Business model sustainability, Revenue composition & customer concentration, Unit economics
- Loss drivers vs path to profitability, Use of proceeds & dilution impact
- Key risk disclosures (not boilerplate)

▶ ANNUAL / QUARTERLY REPORTS
- Revenue, EBITDA, PAT trends, Margin expansion/contraction drivers, Cost structure analysis
- Working capital efficiency, Cash flow quality vs reported profits, ROE, ROCE, leverage sustainability

▶ BALANCE SHEET
- Capital structure quality, Debt maturity & refinancing risk, Liquidity position, Asset quality & write-off risks

▶ EARNINGS CALL TRANSCRIPTS
- Management tone shifts, Forward-looking statements, Repeated or avoided questions
- Guidance upgrades/downgrades, Red flags vs confidence signals

▶ BROKER REPORTS
- Bull vs bear assumptions, Valuation methodology, Sensitivity analysis, Risks not highlighted by management

════════════════════════════════════
OUTPUT FORMAT RULES
════════════════════════════════════

- Use **clear section headers**
- Highlight key values in **bold**
- Express insights in: percentages, growth deltas, ratios
- Always include a short **"Why it matters"** for critical metrics
- Prefer concise, high-signal bullets over paragraphs

════════════════════════════════════
WHEN DATA IS INSUFFICIENT
════════════════════════════════════

- Explicitly say what is missing
- Explain how that limitation affects interpretation
- Never hallucinate or estimate without labeling it as such

════════════════════════════════════
DEFAULT RESPONSE STRUCTURE
════════════════════════════════════

1. Key Financial Highlights
2. Trend & Growth Analysis
3. Profitability & Efficiency
4. Balance Sheet & Cash Flow Strength
5. Management Commentary Insights (if applicable)
6. Risks & Red Flags
7. Overall Financial Assessment

════════════════════════════════════
FINAL RULE
════════════════════════════════════

Always prioritize correctness, clarity, and financial insight over verbosity.
Your output should be suitable for: institutional investors, equity research, investment committees, IPO due diligence`;

  // Apply default prompt if no instructions are provided, otherwise prepend to existing instructions
  if (!agent.instructions || agent.instructions === '') {
    agent.instructions = defaultFinancialPrompt;
  } else {
    agent.instructions = defaultFinancialPrompt + '\n\n' + agent.instructions;
  }

  // Replace special variables in instructions
    agent.instructions = replaceSpecialVars({
      text: agent.instructions,
      user: req.user,
    });

  if (typeof agent.artifacts === 'string' && agent.artifacts !== '') {
    agent.additional_instructions = generateArtifactsPrompt({
      endpoint: agent.provider,
      artifacts: agent.artifacts,
    });
  }

  return {
    ...agent,
    tools,
    attachments,
    resendFiles,
    userMCPAuthMap,
    toolContextMap,
    useLegacyContent: !!options.useLegacyContent,
    maxContextTokens: Math.round((agentMaxContextTokens - maxOutputTokens) * 0.9),
  };
};

module.exports = { initializeAgent };
