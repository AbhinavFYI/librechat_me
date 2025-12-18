package weaviate

import (
	"context"
	"fmt"
	"strconv"

	fylogger "github.com/FyersDev/trading-logger-go"
	"github.com/weaviate/weaviate-go-client/v5/weaviate/graphql"
)

func getFields() []graphql.Field {
	return []graphql.Field{
		//graphql.Field{Name: "content"},
		//graphql.Field{Name: "table"},
		//graphql.Field{Name: "metadata"},
		//graphql.Field{Name: "index"},
		graphql.Field{Name: "content"},
		graphql.Field{Name: "section_title"},
		graphql.Field{Name: "page_number"},
		graphql.Field{Name: "content_type"},
		graphql.Field{Name: "_additional", Fields: []graphql.Field{{Name: "score"}}},
	}
}

func parseResponse(data map[string]interface{}, collection string, score float64) ([]Chunk, error) {
	results := []Chunk{}

	// Get the collection data
	collectionData, ok := data[collection].([]any)
	if !ok {
		return results, nil
	}

	// Parse each result item
	for _, item := range collectionData {
		documentChunk := Chunk{
			Content:      item.(map[string]interface{})["content"].(string),
			SectionTitle: item.(map[string]interface{})["section_title"].(string),
			PageNumber:   int(item.(map[string]interface{})["page_number"].(float64)),
			ContentType:  item.(map[string]interface{})["content_type"].(string),
		}

		score_ := 0.0
		itemMap := item.(map[string]interface{})["_additional"]
		if additional, ok := itemMap.(map[string]interface{}); ok {
			if scoreVal, ok := additional["score"].(string); ok {
				scoreValFloat, err := strconv.ParseFloat(scoreVal, 64)
				if err != nil {
					fmt.Printf("Error parsing score: %v\n", err)
					score = 0.0
				}
				score_ = scoreValFloat
			} else {
				fmt.Printf("Score not found in _additional: %v\n", additional)
			}
		}

		if score_ < score {
			continue
		}
		documentChunk.Score = score_

		results = append(results, documentChunk)
	}
	return results, nil
}

func (w *WeaviateClient) QueryHybridWithCollection(ctx context.Context,
	query string,
	collection string,
	score float64,
	alpha float32) ([]Chunk, error) {
	// Create a new client
	response, err := w.Client.GraphQL().Get().
		WithClassName(collection).
		WithFields(
			getFields()...,
		).
		WithHybrid(
			w.Client.GraphQL().HybridArgumentBuilder().
				WithQuery(query).
				WithAlpha(alpha).
				WithFusionType(graphql.FusionType("relativeScoreFusion")),
		).WithAutocut(2).
		WithLimit(20).
		Do(ctx)

	if err != nil {
		fylogger.ErrorLog(ctx, "failed to query hybrid with collection", err, map[string]interface{}{
			"error":      err,
			"collection": collection,
			"query":      query,
			"score":      score,
			"alpha":      alpha,
			"response":   response,
		})
		return nil, err
	}

	fmt.Println("Response:", response)

	if response.Data != nil {
		data := response.Data["Get"].(map[string]interface{})
		return parseResponse(data, collection, score)
	}
	return nil, nil
}

// DeleteDocumentClasses deletes document classes from Weaviate
func (w *WeaviateClient) DeleteDocumentClasses(ctx context.Context, documentID int64) error {
	classNameText := fmt.Sprintf("document_%d", documentID)
	classNameTable := fmt.Sprintf("document_%d_table", documentID)

	// Delete text class
	err := w.Client.Schema().ClassDeleter().
		WithClassName(classNameText).
		Do(ctx)

	if err != nil {
		// Check if class doesn't exist (not an error in this case)
		exists, _ := w.Client.Schema().ClassExistenceChecker().WithClassName(classNameText).Do(ctx)
		if exists {
			fylogger.ErrorLog(ctx, "failed to delete text class", err, map[string]interface{}{
				"error":      err,
				"class_name": classNameText,
			})
			return fmt.Errorf("failed to delete text class %s: %w", classNameText, err)
		}
	}

	// Delete table class
	err = w.Client.Schema().ClassDeleter().
		WithClassName(classNameTable).
		Do(ctx)

	if err != nil {
		// Check if class doesn't exist (not an error in this case)
		exists, _ := w.Client.Schema().ClassExistenceChecker().WithClassName(classNameTable).Do(ctx)
		if exists {
			fylogger.ErrorLog(ctx, "failed to delete table class", err, map[string]interface{}{
				"error":      err,
				"class_name": classNameTable,
			})
			return fmt.Errorf("failed to delete table class %s: %w", classNameTable, err)
		}
	}

	fylogger.InfoLog(ctx, "successfully deleted document classes", map[string]interface{}{
		"document_id": documentID,
		"text_class":  classNameText,
		"table_class": classNameTable,
	})

	return nil
}
