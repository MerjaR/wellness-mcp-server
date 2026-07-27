package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/MerjaR/wellness-mcp-server/internal/db"
)

// GetRecipeNutritionArgs is the input schema for get_recipe_nutrition.
type GetRecipeNutritionArgs struct {
	RecipeID string `json:"recipe_id" jsonschema:"the recipe's id, as returned by search_recipes"`
}

// IngredientNutrition is one ingredient's contribution to a recipe.
type IngredientNutrition struct {
	Label        string  `json:"label"`
	Quantity     float64 `json:"quantity"`
	Unit         string  `json:"unit"`
	CaloriesKcal float64 `json:"calories_kcal"`
	CarbsG       float64 `json:"carbs_g"`
	ProteinG     float64 `json:"protein_g"`
	FatsG        float64 `json:"fats_g"`
}

// GetRecipeNutritionResult is the structured output for get_recipe_nutrition.
type GetRecipeNutritionResult struct {
	RecipeID    string                `json:"recipe_id"`
	Title       string                `json:"title"`
	Servings    int                   `json:"servings"`
	Total       NutritionTotals       `json:"total"`
	PerServing  NutritionTotals       `json:"per_serving"`
	Ingredients []IngredientNutrition `json:"ingredients"`
}

// NutritionTotals is a simple calories/macros bundle, reused for both
// recipe totals and per-serving figures.
type NutritionTotals struct {
	CaloriesKcal float64 `json:"calories_kcal"`
	CarbsG       float64 `json:"carbs_g"`
	ProteinG     float64 `json:"protein_g"`
	FatsG        float64 `json:"fats_g"`
}

// NewGetRecipeNutritionTool returns the MCP tool handler bound to the
// given database pool.
func NewGetRecipeNutritionTool(database *db.DB) mcp.ToolHandlerFor[GetRecipeNutritionArgs, GetRecipeNutritionResult] {
	return func(ctx context.Context, req *mcp.CallToolRequest, args GetRecipeNutritionArgs) (*mcp.CallToolResult, GetRecipeNutritionResult, error) {
		if args.RecipeID == "" {
			return nil, GetRecipeNutritionResult{}, fmt.Errorf("recipe_id is required")
		}

		var title string
		var servings int
		var total NutritionTotals
		err := database.QueryRow(ctx, `
			SELECT title, servings, total_calories_kcal, total_carbs_g, total_protein_g, total_fats_g
			FROM recipes
			WHERE id = $1
		`, args.RecipeID).Scan(&title, &servings, &total.CaloriesKcal, &total.CarbsG, &total.ProteinG, &total.FatsG)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, GetRecipeNutritionResult{}, fmt.Errorf("no recipe found with id %q", args.RecipeID)
		}
		if err != nil {
			return nil, GetRecipeNutritionResult{}, fmt.Errorf("failed to load recipe: %w", err)
		}

		rows, err := database.Query(ctx, `
			SELECT i.label, ri.quantity, i.unit,
			       i.calories_kcal, i.carbs_g, i.protein_g, i.fats_g
			FROM recipe_ingredients ri
			JOIN ingredients i ON i.id = ri.ingredient_id
			WHERE ri.recipe_id = $1
			ORDER BY i.label
		`, args.RecipeID)
		if err != nil {
			return nil, GetRecipeNutritionResult{}, fmt.Errorf("failed to load ingredients: %w", err)
		}
		defer rows.Close()

		var ingredients []IngredientNutrition
		for rows.Next() {
			var ing IngredientNutrition
			if err := rows.Scan(&ing.Label, &ing.Quantity, &ing.Unit,
				&ing.CaloriesKcal, &ing.CarbsG, &ing.ProteinG, &ing.FatsG); err != nil {
				return nil, GetRecipeNutritionResult{}, fmt.Errorf("failed to scan ingredient row: %w", err)
			}
			ingredients = append(ingredients, ing)
		}
		if err := rows.Err(); err != nil {
			return nil, GetRecipeNutritionResult{}, fmt.Errorf("error iterating ingredient rows: %w", err)
		}
		if ingredients == nil {
			ingredients = []IngredientNutrition{}
		}

		perServingDivisor := float64(servings)
		if perServingDivisor <= 0 {
			perServingDivisor = 1
		}
		perServing := NutritionTotals{
			CaloriesKcal: total.CaloriesKcal / perServingDivisor,
			CarbsG:       total.CarbsG / perServingDivisor,
			ProteinG:     total.ProteinG / perServingDivisor,
			FatsG:        total.FatsG / perServingDivisor,
		}

		output := GetRecipeNutritionResult{
			RecipeID:    args.RecipeID,
			Title:       title,
			Servings:    servings,
			Total:       total,
			PerServing:  perServing,
			Ingredients: ingredients,
		}

		summaryJSON, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return nil, GetRecipeNutritionResult{}, fmt.Errorf("failed to encode result: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(summaryJSON)},
			},
		}, output, nil
	}
}