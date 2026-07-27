// Package tools implements the MCP tools exposing wellness-platform
// recipe data. The SQL filter-building here mirrors the pattern used
// in the wellness-platform's own SearchRecipes handler, trimmed down
// to the filters most useful for an agent caller.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MerjaR/wellness-mcp-server/internal/db"
)

// SearchRecipesArgs is the input schema for the search_recipes tool.
type SearchRecipesArgs struct {
	Query          string   `json:"query,omitempty" jsonschema:"free-text search across title, cuisine, and summary"`
	Cuisine        string   `json:"cuisine,omitempty" jsonschema:"exact cuisine match, e.g. italian"`
	MealType       string   `json:"meal_type,omitempty" jsonschema:"one of: breakfast, lunch, dinner, snack"`
	DietaryTags    []string `json:"dietary_tags,omitempty" jsonschema:"recipe must have ALL of these tags, e.g. vegetarian, high-protein"`
	MaxTimeMinutes int      `json:"max_time_minutes,omitempty" jsonschema:"only recipes at or under this prep time"`
	Limit          int      `json:"limit,omitempty" jsonschema:"max results to return, default 10, capped at 25"`
}

// RecipeResult is a single recipe row returned to the caller.
type RecipeResult struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Cuisine         string   `json:"cuisine"`
	Meal            string   `json:"meal"`
	Servings        int      `json:"servings"`
	Summary         string   `json:"summary"`
	TimeMinutes     int      `json:"time_minutes"`
	DifficultyLevel string   `json:"difficulty_level"`
	DietaryTags     []string `json:"dietary_tags"`
	Allergens       []string `json:"allergens"`
	CaloriesKcal    float64  `json:"calories_kcal"`
	ProteinG        float64  `json:"protein_g"`
	CarbsG          float64  `json:"carbs_g"`
	FatsG           float64  `json:"fats_g"`
}

// SearchRecipesResult is the structured output for search_recipes.
// Wrapped in an object (rather than a bare slice) because the MCP SDK
// generates an object-shaped output schema by default.
type SearchRecipesResult struct {
	Recipes []RecipeResult `json:"recipes"`
	Count   int            `json:"count"`
}

// NewSearchRecipesTool returns the MCP tool handler bound to the given
// database pool.
func NewSearchRecipesTool(database *db.DB) mcp.ToolHandlerFor[SearchRecipesArgs, SearchRecipesResult] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args SearchRecipesArgs) (*mcp.CallToolResult, SearchRecipesResult, error) {
		where := []string{"1=1"}
		params := []any{}
		argIdx := 1

		query := strings.TrimSpace(args.Query)
		if query != "" {
			where = append(where, fmt.Sprintf(
				`(LOWER(title) LIKE LOWER($%d) OR LOWER(cuisine) LIKE LOWER($%d) OR LOWER(summary) LIKE LOWER($%d))`,
				argIdx, argIdx, argIdx,
			))
			params = append(params, "%"+query+"%")
			argIdx++
		}

		if args.Cuisine != "" {
			where = append(where, fmt.Sprintf(`LOWER(cuisine) = LOWER($%d)`, argIdx))
			params = append(params, args.Cuisine)
			argIdx++
		}

		if args.MealType != "" {
			where = append(where, fmt.Sprintf(`LOWER(meal) = LOWER($%d)`, argIdx))
			params = append(params, args.MealType)
			argIdx++
		}

		if args.MaxTimeMinutes > 0 {
			where = append(where, fmt.Sprintf(`time_minutes <= $%d`, argIdx))
			params = append(params, args.MaxTimeMinutes)
			argIdx++
		}

		for _, tag := range args.DietaryTags {
			where = append(where, fmt.Sprintf(`dietary_tags @> $%d::jsonb`, argIdx))
			params = append(params, fmt.Sprintf(`["%s"]`, tag))
			argIdx++
		}

		limit := args.Limit
		if limit <= 0 {
			limit = 10
		}
		if limit > 25 {
			limit = 25
		}

		whereClause := strings.Join(where, " AND ")
		selectQuery := fmt.Sprintf(`
			SELECT id, title, cuisine, meal, servings, summary, time_minutes,
			       difficulty_level, dietary_tags, allergens,
			       total_calories_kcal, total_protein_g, total_carbs_g, total_fats_g
			FROM recipes
			WHERE %s
			ORDER BY title ASC
			LIMIT $%d
		`, whereClause, argIdx)
		params = append(params, limit)

		rows, err := database.Query(ctx, selectQuery, params...)
		if err != nil {
			return nil, SearchRecipesResult{}, fmt.Errorf("recipe search failed: %w", err)
		}
		defer rows.Close()

		var results []RecipeResult
		for rows.Next() {
			var r RecipeResult
			var dietaryTagsJSON, allergensJSON []byte
			if err := rows.Scan(
				&r.ID, &r.Title, &r.Cuisine, &r.Meal, &r.Servings,
				&r.Summary, &r.TimeMinutes, &r.DifficultyLevel,
				&dietaryTagsJSON, &allergensJSON,
				&r.CaloriesKcal, &r.ProteinG, &r.CarbsG, &r.FatsG,
			); err != nil {
				// Surface the error instead of silently dropping the row —
				// a scan failure usually means a column/type mismatch worth
				// seeing, not a row worth skipping quietly.
				return nil, SearchRecipesResult{}, fmt.Errorf("failed to scan recipe row: %w", err)
			}
			json.Unmarshal(dietaryTagsJSON, &r.DietaryTags)
			json.Unmarshal(allergensJSON, &r.Allergens)
			results = append(results, r)
		}
		if err := rows.Err(); err != nil {
			return nil, SearchRecipesResult{}, fmt.Errorf("error iterating recipe rows: %w", err)
		}
		if results == nil {
			results = []RecipeResult{}
		}

		resultJSON, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return nil, SearchRecipesResult{}, fmt.Errorf("failed to encode results: %w", err)
		}

		output := SearchRecipesResult{Recipes: results, Count: len(results)}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Found %d recipe(s):\n%s", len(results), string(resultJSON))},
			},
		}, output, nil
	}
}