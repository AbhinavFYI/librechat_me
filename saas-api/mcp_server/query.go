package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"saas-api/cmd/configs"
	"saas-api/pkg/weaviate"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var (
	weaviateClient *weaviate.WeaviateClient
)

// SearchMode represents the type of content to search
type SearchMode string

const (
	SearchModeText  SearchMode = "text"
	SearchModeTable SearchMode = "table"

	// Default alpha values for each mode
	DefaultAlphaText  float32 = 0.6
	DefaultAlphaTable float32 = 0.3

	// Max retries for search
	MaxRetries = 3
)

// DocumentSearchArgs represents the arguments for document search
type DocumentSearchArgs struct {
	Query      string     `json:"query"`
	Collection int        `json:"collection"`
	Score      float64    `json:"score"`
	Alpha      float32    `json:"alpha"`
	Mode       SearchMode `json:"mode"`
}

// parseCollectionID normalizes the "collection" argument coming from the MCP request.
// JSON numbers are typically decoded as float64, so we accept multiple numeric types.
func parseCollectionID(raw interface{}) (int, error) {
	switch v := raw.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float32:
		return int(v), nil
	case float64:
		return int(v), nil
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("invalid collection value %q: %w", v, err)
		}
		return int(i), nil
	default:
		return 0, fmt.Errorf("unsupported collection type %T", raw)
	}
}

// Initialize weaviate client
func init() {
	// err := godotenvssm.Load(
	// 	fmt.Sprintf("ssm:/insti/%s",
	// 		os.Getenv("APP_ENV"),
	err := godotenv.Load("/home/ec2-user/librechat_me/saas-api/.env")
	if err != nil {
		// log.Printf(err.Error())
		panic(err)
	}
	config := configs.LoadConfig()
	weaviateClient = weaviate.NewWeaviateClient(config)
}

// DocumentSearchTool defines the MCP tool for document search
var DocumentSearchTool = mcp.Tool{
	Name: "document_search",
	Description: `Search documents in the vector database using hybrid search (semantic + keyword).

IMPORTANT USAGE INSTRUCTIONS:
1. ALWAYS make TWO separate calls - one with mode='text' and one with mode='table' to get complete results.
2. Combine results from both calls to provide comprehensive answers.

RETRY STRATEGY (max 3 retries per mode):
- If no relevant results found, retry with adjusted parameters:
  - Try lowering the score threshold (e.g., 0.5 -> 0.3 -> 0.2)
  - Try adjusting alpha: for text increase toward 0.8, for tables decrease toward 0.2
  - Try rephrasing the query with different keywords or more specific terms

DEFAULT ALPHA VALUES:
- For mode='text': alpha=0.6 (balanced semantic + keyword)
- For mode='table': alpha=0.3 (more keyword-focused for structured data)

Returns relevant document chunks with content, section title, page number, and relevance score.`,
	InputSchema: mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "The search query text to find relevant documents. Be specific and use relevant keywords.",
			},
			"collection": map[string]interface{}{
				"type":        "integer",
				"description": "The base document collection ID (e.g., 1, 4). The mode suffix will be appended automatically.",
			},
			"mode": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"text", "table"},
				"description": "Content type to search: 'text' for paragraphs/prose, 'table' for tabular data. MUST call with BOTH modes separately for complete results.",
				"default":     "text",
			},
			"score": map[string]interface{}{
				"type":        "number",
				"description": "Minimum relevance score threshold (0.0 to 1.0). Start with 0.5, lower if no results. Default is 0.5",
				"default":     0.5,
			},
			"alpha": map[string]interface{}{
				"type":        "number",
				"description": "Balance between semantic (1.0) and keyword (0.0) search. Default: 0.6 for text, 0.3 for tables. Adjust on retry.",
			},
		},
		Required: []string{"query", "collection", "mode"},
	},
}

// HandleDocumentSearch handles the document search tool call
func HandleDocumentSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse arguments
	args := DocumentSearchArgs{
		Score: 0.5,            // default
		Mode:  SearchModeText, // default
	}

	// Convert Arguments to map
	argsMap, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments format"), nil
	}

	// Extract query (required)
	if query, ok := argsMap["query"].(string); ok {
		args.Query = query
	} else {
		return mcp.NewToolResultError("query parameter is required"), nil
	}

	// Extract collection (required)
	rawCollection, ok := argsMap["collection"]
	if !ok {
		return mcp.NewToolResultError("collection parameter is required"), nil
	}
	collection, err := parseCollectionID(rawCollection)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid collection parameter: %v", err)), nil
	}
	args.Collection = collection

	// Extract mode (required)
	if mode, ok := argsMap["mode"].(string); ok {
		if mode == "table" {
			args.Mode = SearchModeTable
		} else {
			args.Mode = SearchModeText
		}
	}

	// Set default alpha based on mode
	if args.Mode == SearchModeTable {
		args.Alpha = DefaultAlphaTable
	} else {
		args.Alpha = DefaultAlphaText
	}

	// Extract score (optional)
	if score, ok := argsMap["score"].(float64); ok {
		args.Score = score
	}

	// Extract alpha (optional - overrides mode default)
	if alpha, ok := argsMap["alpha"].(float64); ok {
		args.Alpha = float32(alpha)
	}

	// Form the full collection name with mode suffix
	fullCollectionName := fmt.Sprintf("Document_%d", args.Collection)

	if args.Mode == SearchModeTable {
		fullCollectionName = fmt.Sprintf("Document_%d_table", args.Collection)
	}

	// Perform the search
	results, err := weaviateClient.QueryHybridWithCollection(ctx, args.Query, fullCollectionName, args.Score, args.Alpha)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	// Build response header with search parameters for retry guidance
	var response strings.Builder
	response.WriteString(fmt.Sprintf("Search Parameters: mode=%s, collection=%s, alpha=%.2f, score_threshold=%.2f\n",
		args.Mode, fullCollectionName, args.Alpha, args.Score))
	response.WriteString(fmt.Sprintf("Query: %s\n\n", args.Query))

	if len(results) == 0 {
		response.WriteString("No documents found matching your query.\n\n")
		response.WriteString("RETRY SUGGESTIONS:\n")
		response.WriteString("- Lower score threshold (try 0.3 or 0.2)\n")
		if args.Mode == SearchModeText {
			response.WriteString("- Increase alpha toward 0.8 for more semantic matching\n")
		} else {
			response.WriteString("- Decrease alpha toward 0.1 for more keyword matching\n")
		}
		response.WriteString("- Rephrase query with different keywords\n")
		response.WriteString(fmt.Sprintf("- Max retries allowed: %d\n", MaxRetries))
		return mcp.NewToolResultText(response.String()), nil
	}

	response.WriteString(fmt.Sprintf("Found %d relevant %s chunks:\n\n", len(results), args.Mode))

	for i, chunk := range results {
		response.WriteString(fmt.Sprintf("--- Result %d (Score: %.2f) ---\n", i+1, chunk.Score))
		response.WriteString(fmt.Sprintf("Section: %s\n", chunk.SectionTitle))
		response.WriteString(fmt.Sprintf("Content Type: %s\n", chunk.ContentType))
		response.WriteString(fmt.Sprintf("Page: %d\n", chunk.PageNumber))
		response.WriteString(fmt.Sprintf("Content:\n%s\n\n", chunk.Content))
	}

	// Reminder to search the other mode
	if args.Mode == SearchModeText {
		response.WriteString("\n💡 REMINDER: Also search with mode='table' to find data in tables.\n")
	} else {
		response.WriteString("\n💡 REMINDER: Also search with mode='text' to find data in paragraphs.\n")
	}

	return mcp.NewToolResultText(response.String()), nil
}

// SearchResultJSON represents the JSON response structure
type SearchResultJSON struct {
	Query          string           `json:"query"`
	Collection     string           `json:"collection"`
	Mode           string           `json:"mode"`
	Alpha          float32          `json:"alpha"`
	ScoreThreshold float64          `json:"score_threshold"`
	ResultCount    int              `json:"result_count"`
	Results        []weaviate.Chunk `json:"results"`
	RetryHints     *RetryHints      `json:"retry_hints,omitempty"`
}

// RetryHints provides guidance for retrying failed searches
type RetryHints struct {
	SuggestedAlpha float32  `json:"suggested_alpha"`
	SuggestedScore float64  `json:"suggested_score"`
	MaxRetries     int      `json:"max_retries"`
	QueryTips      []string `json:"query_tips"`
}

// HandleDocumentSearchJSON returns results as JSON for programmatic use
func HandleDocumentSearchJSON(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse arguments
	args := DocumentSearchArgs{
		Score: 0.5,
		Mode:  SearchModeText,
	}

	// Convert Arguments to map
	argsMap, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments format"), nil
	}

	if query, ok := argsMap["query"].(string); ok {
		args.Query = query
	} else {
		return mcp.NewToolResultError("query parameter is required"), nil
	}

	if collection, ok := argsMap["collection"].(int); ok {
		args.Collection = collection
	} else {
		rawCollection, ok := argsMap["collection"]
		if !ok {
			return mcp.NewToolResultError("collection parameter is required"), nil
		}

		parsed, err := parseCollectionID(rawCollection)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("invalid collection parameter: %v", err)), nil
		}
		args.Collection = parsed
	}

	// Extract mode
	if mode, ok := argsMap["mode"].(string); ok {
		if mode == "table" {
			args.Mode = SearchModeTable
		} else {
			args.Mode = SearchModeText
		}
	}

	// Set default alpha based on mode
	if args.Mode == SearchModeTable {
		args.Alpha = DefaultAlphaTable
	} else {
		args.Alpha = DefaultAlphaText
	}

	if score, ok := argsMap["score"].(float64); ok {
		args.Score = score
	}

	if alpha, ok := argsMap["alpha"].(float64); ok {
		args.Alpha = float32(alpha)
	}

	// Form the full collection name with mode suffix
	fullCollectionName := fmt.Sprintf("Document_%d", args.Collection)
	if args.Mode == SearchModeTable {
		fullCollectionName = fmt.Sprintf("Document_%d_table", args.Collection)
	}

	results, err := weaviateClient.QueryHybridWithCollection(ctx, args.Query, fullCollectionName, args.Score, args.Alpha)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	// Build JSON response
	response := SearchResultJSON{
		Query:          args.Query,
		Collection:     fullCollectionName,
		Mode:           string(args.Mode),
		Alpha:          args.Alpha,
		ScoreThreshold: args.Score,
		ResultCount:    len(results),
		Results:        results,
	}

	// Add retry hints if no results
	if len(results) == 0 {
		var suggestedAlpha float32
		if args.Mode == SearchModeTable {
			suggestedAlpha = 0.1
		} else {
			suggestedAlpha = 0.8
		}
		response.RetryHints = &RetryHints{
			SuggestedAlpha: suggestedAlpha,
			SuggestedScore: 0.3,
			MaxRetries:     MaxRetries,
			QueryTips: []string{
				"Try using more specific keywords",
				"Rephrase the query with synonyms",
				"Use exact terms from the document if known",
			},
		}
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal results: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// DocumentSearchJSONTool defines the MCP tool for document search with JSON output
var DocumentSearchJSONTool = mcp.Tool{
	Name: "document_search_json",
	Description: `Search documents and return results as JSON for programmatic use.

IMPORTANT: Make TWO separate calls with mode='text' and mode='table' for complete results.

RETRY STRATEGY (max 3 retries):
- Response includes retry_hints when no results found
- Adjust alpha and score based on hints
- Rephrase query if needed

DEFAULT ALPHA: 0.6 for text, 0.3 for tables`,
	InputSchema: mcp.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "The search query text",
			},
			"collection": map[string]interface{}{
				"type":        "integer",
				"description": "Base document collection ID (mode suffix added automatically)",
			},
			"mode": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"text", "table"},
				"description": "Content type: 'text' or 'table'. Call BOTH separately.",
				"default":     "text",
			},
			"score": map[string]interface{}{
				"type":        "number",
				"description": "Minimum relevance score (0.0-1.0)",
				"default":     0.5,
			},
			"alpha": map[string]interface{}{
				"type":        "number",
				"description": "Semantic (1.0) vs keyword (0.0) balance",
			},
		},
		Required: []string{"query", "collection", "mode"},
	},
}

// MCPServer wraps the MCP server with SSE support
type MCPServer struct {
	mcpServer *server.MCPServer
	sseServer *server.SSEServer
}

// NewMCPServer creates a new MCP SSE server for document search
func NewMCPServer() *MCPServer {
	// Create the MCP server
	mcpServer := server.NewMCPServer(
		"Document Search MCP Server",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	// Register tools
	mcpServer.AddTool(DocumentSearchTool, HandleDocumentSearch)
	mcpServer.AddTool(DocumentSearchJSONTool, HandleDocumentSearchJSON)

	return &MCPServer{
		mcpServer: mcpServer,
	}
}

// StartSSE starts the SSE server on the specified address
func (s *MCPServer) StartSSE(addr string) error {
	// Create SSE server with configuration
	s.sseServer = server.NewSSEServer(s.mcpServer,
		server.WithBaseURL(fmt.Sprintf("http://%s", addr)),
		server.WithSSEEndpoint("/sse"),
		server.WithMessageEndpoint("/message"),
		server.WithKeepAliveInterval(30*time.Second),
	)

	log.Printf("Starting MCP SSE server on %s", addr)
	log.Printf("SSE endpoint: %s/sse", addr)
	log.Printf("Message endpoint: %s/message", addr)

	return s.sseServer.Start(addr)
}

// StartStdio starts the server in stdio mode (for CLI tools)
func (s *MCPServer) StartStdio() error {
	log.Println("Starting MCP server in stdio mode")
	return server.ServeStdio(s.mcpServer)
}

// Query is a convenience function for direct querying
func Query(ctx context.Context, query string, collection string, mode SearchMode, score float64, alpha float32) ([]weaviate.Chunk, error) {
	// Form full collection name with mode suffix
	fullCollectionName := fmt.Sprintf("%s_%s", collection, mode)
	return weaviateClient.QueryHybridWithCollection(ctx, query, fullCollectionName, score, alpha)
}

// QueryBoth searches both text and table modes and combines results
func QueryBoth(ctx context.Context, query string, collection string, score float64) ([]weaviate.Chunk, []weaviate.Chunk, error) {
	textResults, err := Query(ctx, query, collection, SearchModeText, score, DefaultAlphaText)
	if err != nil {
		return nil, nil, fmt.Errorf("text search failed: %w", err)
	}

	tableResults, err := Query(ctx, query, collection, SearchModeTable, score, DefaultAlphaTable)
	if err != nil {
		return textResults, nil, fmt.Errorf("table search failed: %w", err)
	}

	return textResults, tableResults, nil
}

// StartMCPServer starts the MCP SSE server (backward compatibility)
func StartMCPServer() {
	srv := NewMCPServer()
	if err := srv.StartSSE(":8081"); err != nil {
		log.Fatalf("Failed to start MCP server: %v", err)
	}
}

func main() {
	StartMCPServer()
}
