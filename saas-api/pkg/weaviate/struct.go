package weaviate

import "github.com/weaviate/weaviate-go-client/v5/weaviate"

type WeaviateClient struct {
	*weaviate.Client
}

type SearchResult any

// DocumentChunk represents a chunk of markdown document with metadata
type DocumentChunk struct {
	ID           string                 `json:"id"`
	Content      string                 `json:"content"`
	ChunkIndex   int                    `json:"chunkIndex"`
	DocumentID   string                 `json:"documentId"`
	DocumentName string                 `json:"documentName"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type Document struct {
	Name string `json:"name"`
	ID   string `json:"id"`
	//Type    string      `json:"document_type"`
	//Format  string      `json:"document_format"`
	//Year    int         `json:"document_year"`
	//Quarter interface{} `json:"document_quarter"`
	Chunks []struct {
		ID      string `json:"id"`
		Content string `json:"content"`
		Index   int    `json:"index"`
		Table   *struct {
			ID     string     `json:"id"`
			Title  string     `json:"title"`
			Header [][]string `json:"header"`
			Body   [][]string `json:"body"`
		} `json:"table,omitempty"`
		Metadata *struct {
			SectionTitle string `json:"section_title"`
			ContentType  string `json:"content_type"`
		} `json:"metadata"`
	} `json:"chunks"`
}

type Chunk struct {
	ID           string  `json:"chunk_id,omitempty"`
	Content      string  `json:"content,omitempty"`
	ContentType  string  `json:"content_type,omitempty"`
	PageNumber   int     `json:"page_number,omitempty"`
	SectionTitle string  `json:"section_title,omitempty"`
	ChunkIndex   int     `json:"chunk_index,omitempty"`
	Score        float64 `json:"score,omitempty"`
}

// PopulateConfig holds configuration for populating data
type PopulateConfig struct {
	ClassName        string // Weaviate class name to insert data into
	ChunkSize        int    // Size of each chunk in characters
	ChunkOverlap     int    // Overlap between chunks
	BatchSize        int    // Number of objects to batch insert
	ConsistencyLevel string // Consistency level for writes (ONE, QUORUM, ALL)
}
