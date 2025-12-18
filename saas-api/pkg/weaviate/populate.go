package weaviate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/weaviate/weaviate/entities/models"
)

// DefaultPopulateConfig returns default configuration
func DefaultPopulateConfig() *PopulateConfig {
	return &PopulateConfig{
		ChunkSize:        1000,
		ChunkOverlap:     200,
		BatchSize:        10,
		ConsistencyLevel: "ONE",
	}
}

func NewPopulateConfig(className string, chunkSize int, chunkOverlap int, batchSize int, consistencyLevel string) *PopulateConfig {
	return &PopulateConfig{
		ChunkSize:        chunkSize,
		ChunkOverlap:     chunkOverlap,
		BatchSize:        batchSize,
		ConsistencyLevel: consistencyLevel,
	}
}

// ChunkMarkdown splits markdown content into overlapping chunks
func ChunkMarkdown(content string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 1000
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 2
	}

	// Remove excessive whitespace and normalize line breaks
	content = strings.TrimSpace(content)
	if len(content) == 0 {
		return []string{}
	}

	chunks := []string{}
	start := 0
	contentLen := len(content)

	for start < contentLen {
		end := start + chunkSize
		if end > contentLen {
			end = contentLen
		}

		// Try to break at sentence or paragraph boundaries
		if end < contentLen {
			// Look for paragraph break first
			if idx := strings.LastIndex(content[start:end], "\n\n"); idx > chunkSize/2 {
				end = start + idx + 2
			} else if idx := strings.LastIndex(content[start:end], ". "); idx > chunkSize/2 {
				// Look for sentence break
				end = start + idx + 2
			} else if idx := strings.LastIndex(content[start:end], "\n"); idx > chunkSize/2 {
				// Look for line break
				end = start + idx + 1
			}
		}

		chunk := strings.TrimSpace(content[start:end])
		if len(chunk) > 0 {
			chunks = append(chunks, chunk)
		}

		// Move start position with overlap
		start = end - overlap
		if start <= 0 {
			start = end
		}
	}

	return chunks
}

// PopulateFromMarkdown chunks markdown content and inserts into Weaviate
func (w *WeaviateClient) PopulateFromMarkdownChunks(
	ctx context.Context,
	jsonFilePath string,
	config *PopulateConfig,
	documentID int64,
) error {
	if config == nil {
		config = DefaultPopulateConfig()
	}
	// Create document chunks with metadata
	var docChunks []Chunk

	//read junks from json file
	fmt.Println("Reading chunks from: ", jsonFilePath)
	jsonFile, err := os.Open(jsonFilePath)
	if err != nil {
		return fmt.Errorf("failed to open chunks.json: %w", err)
	}
	defer jsonFile.Close()

	fmt.Println("Decoding chunks from: ", jsonFilePath)
	json.NewDecoder(jsonFile).Decode(&docChunks)

	// Batch insert chunks
	return w.BatchInsertChunks(ctx, docChunks, config, documentID)
}

func (w *WeaviateClient) CreateClass(ctx context.Context, className string) error {
	err := w.Client.Schema().ClassCreator().
		WithClass(&models.Class{
			Class: className,
			VectorConfig: map[string]models.VectorConfig{
				"details_vector": {
					Vectorizer: map[string]interface{}{
						"text2vec-transformers": map[string]interface{}{
							//"properties": []string{"content", "table", "metadata"},
							//"properties": []string{"details"},
							"properties": []string{"content", "section_title"},
						},
					},
					VectorIndexType: "hnsw",
				},
			},
		}).
		Do(ctx)

	if err != nil {
		exists, _ := w.Client.Schema().ClassExistenceChecker().WithClassName(className).Do(ctx)
		if exists {
			fmt.Println("Class already exists, skipping creation")
			return nil
		}
		return err
	}
	return nil
}

// BatchInsertChunks inserts document chunks into Weaviate in batches
func (w *WeaviateClient) BatchInsertChunks(
	ctx context.Context,
	chunks []Chunk,
	config *PopulateConfig,
	documentID int64,
) error {
	if config == nil {
		config = DefaultPopulateConfig()
	}

	classNameText := fmt.Sprintf("document_%d", documentID)
	err := w.CreateClass(ctx, classNameText)
	if err != nil {
		return err
	}

	classNameTable := fmt.Sprintf("document_%d_table", documentID)
	err = w.CreateClass(ctx, classNameTable)
	if err != nil {
		return err
	}

	batcher := w.Client.Batch().ObjectsBatcher()

	fmt.Println("Batch inserting chunks for document: ", documentID)
	for i, chunk := range chunks {
		// Prepare properties for Weaviate
		properties := make(map[string]interface{})

		properties["content"] = chunk.Content
		properties["section_title"] = chunk.SectionTitle
		properties["page_number"] = chunk.PageNumber
		properties["content_type"] = chunk.ContentType

		className := classNameText
		if chunk.ContentType == "table" {
			className = classNameTable
		}
		batcher = batcher.WithObjects(&models.Object{
			Class:      className,
			ID:         strfmt.UUID(uuid.New().String()),
			Properties: properties,
		},
		)

		fmt.Println("Batch inserting chunk: ", chunk.ID)
		// Execute batch when reaching batch size or at the end
		if (i+1)%config.BatchSize == 0 || i == len(chunks)-1 {
			fmt.Println("Executing batch")
			_, err := batcher.Do(ctx)
			if err != nil {
				return fmt.Errorf("batch insert failed at chunk %d: %w", i, err)
			}
			// Create new batcher for next batch
			if i < len(chunks)-1 {
				fmt.Println("Creating new batcher for next batch")
				batcher = w.Client.Batch().ObjectsBatcher()
			}
		}
	}
	fmt.Println("Batch insert completed successfully")
	return nil
}
