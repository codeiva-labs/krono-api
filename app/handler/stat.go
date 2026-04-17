package handler

import (
	"context"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"

	"codeiva/krono-api/app/model"
	"codeiva/krono-api/pkg/response"
)

type CategoryDuration struct {
	CategoryID   string `json:"category_id"`
	CategoryName string `json:"category_name"`
	Duration     int64  `json:"duration"` // in seconds
}

type DailyStats struct {
	Date          string             `json:"date"` // YYYY-MM-DD format
	Categories    []CategoryDuration `json:"categories"`
	TotalDuration int64              `json:"total_duration"` // total seconds for the day
}

type StatsResponse struct {
	Period    string       `json:"period"`
	StartDate string       `json:"start_date"`
	EndDate   string       `json:"end_date"`
	Timezone  string       `json:"timezone"`
	Days      []DailyStats `json:"days"`
}

// GetDailyStats returns daily activity statistics grouped by category for a specified period
// @author Farhan<farhan@codeiva.com>
func GetDailyStats(collections *model.Collections, w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id")
	if userID == nil {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userObjID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}

	// Get user timezone from header
	timezoneStr := r.Header.Get("X-Timezone")
	if timezoneStr == "" {
		timezoneStr = "UTC" // default to UTC if not provided
	}

	// Parse timezone
	loc, err := time.LoadLocation(timezoneStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid timezone")
		return
	}

	// Parse query parameters
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "week" // default to week
	}

	// Validate period
	if period != "week" && period != "month" && period != "year" {
		response.Error(w, http.StatusBadRequest, "period must be 'week', 'month', or 'year'")
		return
	}

	// Calculate date range based on period in user's timezone
	now := time.Now().In(loc)
	var startDate time.Time

	switch period {
	case "week":
		startDate = now.AddDate(0, 0, -7)
	case "month":
		startDate = now.AddDate(0, -1, 0)
	case "year":
		startDate = now.AddDate(-1, 0, 0)
	}

	// Set time to start of day for startDate and end of day for now in user's timezone
	startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, loc)
	endDate := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, loc)

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// MongoDB aggregation pipeline
	pipeline := mongo.Pipeline{
		// Stage 1: Match activities for this user within date range
		{{Key: "$match", Value: bson.D{
			{Key: "user_id", Value: userObjID},
			{Key: "start_time", Value: bson.D{
				{Key: "$gte", Value: startDate},
				{Key: "$lte", Value: endDate},
			}},
			{Key: "end_time", Value: bson.D{{Key: "$ne", Value: nil}}}, // Only completed activities
			{Key: "duration", Value: bson.D{{Key: "$gt", Value: 0}}},   // Only activities with duration
		}}},
		// Stage 2: Project date without time (convert to user's timezone)
		{{Key: "$project", Value: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "category_id", Value: 1},
			{Key: "duration", Value: 1},
			{Key: "date", Value: bson.D{
				{Key: "$dateToString", Value: bson.D{
					{Key: "format", Value: "%Y-%m-%d"},
					{Key: "date", Value: "$start_time"},
					{Key: "timezone", Value: timezoneStr},
				}},
			}},
		}}},
		// Stage 3: Group by date and category_id, sum durations
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "date", Value: "$date"},
				{Key: "category_id", Value: "$category_id"},
			}},
			{Key: "total_duration", Value: bson.D{{Key: "$sum", Value: "$duration"}}},
		}}},
		// Stage 4: Group by date to collect all categories
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$_id.date"},
			{Key: "categories", Value: bson.D{
				{Key: "$push", Value: bson.D{
					{Key: "category_id", Value: "$_id.category_id"},
					{Key: "duration", Value: "$total_duration"},
				}},
			}},
			{Key: "total_duration", Value: bson.D{{Key: "$sum", Value: "$total_duration"}}},
		}}},
		// Stage 5: Sort by date
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}

	cursor, err := collections.Activities.Aggregate(ctx, pipeline)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch statistics")
		return
	}
	defer cursor.Close(ctx)

	// Structure to decode aggregation results
	type aggregationResult struct {
		Date          string `bson:"_id"`
		TotalDuration int64  `bson:"total_duration"`
		Categories    []struct {
			CategoryID primitive.ObjectID `bson:"category_id"`
			Duration   int64              `bson:"duration"`
		} `bson:"categories"`
	}

	var results []aggregationResult
	if err := cursor.All(ctx, &results); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to decode statistics")
		return
	}

	// Fetch all categories to map names
	categoryMap := make(map[primitive.ObjectID]string)
	categoryCursor, err := collections.Categories.Find(ctx, bson.M{})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch categories")
		return
	}
	defer categoryCursor.Close(ctx)

	var categories []model.Category
	if err := categoryCursor.All(ctx, &categories); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to decode categories")
		return
	}

	for _, cat := range categories {
		categoryMap[cat.ID] = cat.Name
	}

	// Format response
	days := make([]DailyStats, len(results))
	for i, result := range results {
		categoryDurations := make([]CategoryDuration, len(result.Categories))
		for j, cat := range result.Categories {
			categoryDurations[j] = CategoryDuration{
				CategoryID:   cat.CategoryID.Hex(),
				CategoryName: categoryMap[cat.CategoryID],
				Duration:     cat.Duration,
			}
		}

		days[i] = DailyStats{
			Date:          result.Date,
			Categories:    categoryDurations,
			TotalDuration: result.TotalDuration,
		}
	}

	// Build response
	statsResponse := StatsResponse{
		Period:    period,
		StartDate: startDate.Format("2006-01-02"),
		EndDate:   endDate.Format("2006-01-02"),
		Timezone:  timezoneStr,
		Days:      days,
	}

	response.Success(w, statsResponse)
}

// GetMostUsedCategories returns the most used categories for a specified period
// @author Farhan<farhan@codeiva.com>
func GetMostUsedCategories(collections *model.Collections, w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id")
	if userID == nil {
		response.Error(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	userObjID, err := primitive.ObjectIDFromHex(userID.(string))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid user id")
		return
	}

	// Get user timezone from header
	timezoneStr := r.Header.Get("X-Timezone")
	if timezoneStr == "" {
		timezoneStr = "UTC"
	}

	loc, err := time.LoadLocation(timezoneStr)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid timezone")
		return
	}

	// Parse query parameters
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "day"
	}

	if period != "day" && period != "week" && period != "month" && period != "year" {
		response.Error(w, http.StatusBadRequest, "period must be 'day', 'week', 'month', or 'year'")
		return
	}

	now := time.Now().In(loc)
	var startDate time.Time
	var endDate time.Time = now

	switch period {
	case "day":
		// If period is 'day', the start date should be 24 hours ago instead of start of the day
		startDate = now.Add(-24 * time.Hour)
	case "week":
		startDate = now.AddDate(0, 0, -7)
	case "month":
		startDate = now.AddDate(0, -1, 0)
	case "year":
		startDate = now.AddDate(-1, 0, 0)
	}

	if period != "day" {
		startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, loc)
		endDate = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, loc)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	pipeline := mongo.Pipeline{
		// Stage 1: Match activities for this user within date range
		{{Key: "$match", Value: bson.D{
			{Key: "user_id", Value: userObjID},
			{Key: "start_time", Value: bson.D{
				{Key: "$gte", Value: startDate},
				{Key: "$lte", Value: endDate},
			}},
			{Key: "end_time", Value: bson.D{{Key: "$ne", Value: nil}}},
			{Key: "duration", Value: bson.D{{Key: "$gt", Value: 0}}},
		}}},
		// Stage 2: Group by category_id, sum durations and count activities
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$category_id"},
			{Key: "total_duration", Value: bson.D{{Key: "$sum", Value: "$duration"}}},
			{Key: "activity_count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		// Stage 3: Lookup category details
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "categories"},
			{Key: "localField", Value: "_id"},
			{Key: "foreignField", Value: "_id"},
			{Key: "as", Value: "category"},
		}}},
		{{Key: "$unwind", Value: bson.D{
			{Key: "path", Value: "$category"},
			{Key: "preserveNullAndEmptyArrays", Value: true},
		}}},
		// Stage 4: Sort by total duration descending
		{{Key: "$sort", Value: bson.D{{Key: "total_duration", Value: -1}}}},
	}

	cursor, err := collections.Activities.Aggregate(ctx, pipeline)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to fetch statistics")
		return
	}
	defer cursor.Close(ctx)

	type categoryStatResult struct {
		CategoryID    primitive.ObjectID `bson:"_id"`
		TotalDuration int64              `bson:"total_duration"`
		ActivityCount int64              `bson:"activity_count"`
		Category      *model.Category    `bson:"category"`
	}

	var results []categoryStatResult
	if err := cursor.All(ctx, &results); err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to decode statistics")
		return
	}

	type categoryStatResponse struct {
		CategoryID    string  `json:"category_id"`
		CategoryName  string  `json:"category_name"`
		Color         string  `json:"color,omitempty"`
		Icon          string  `json:"icon,omitempty"`
		TotalDuration int64   `json:"total_duration"`
		ActivityCount int64   `json:"activity_count"`
		Percentage    float64 `json:"percentage"`
	}

	// Calculate total duration across all categories
	var grandTotal int64
	for _, r := range results {
		grandTotal += r.TotalDuration
	}

	items := make([]categoryStatResponse, len(results))
	for i, r := range results {
		item := categoryStatResponse{
			CategoryID:    r.CategoryID.Hex(),
			TotalDuration: r.TotalDuration,
			ActivityCount: r.ActivityCount,
		}
		if r.Category != nil {
			item.CategoryName = r.Category.Name
			item.Color = r.Category.Color
			item.Icon = r.Category.Icon
		}
		if grandTotal > 0 {
			item.Percentage = float64(r.TotalDuration) / float64(grandTotal) * 100
		}
		items[i] = item
	}

	response.Success(w, map[string]any{
		"period":         period,
		"start_date":     startDate.Format("2006-01-02"),
		"end_date":       endDate.Format("2006-01-02"),
		"timezone":       timezoneStr,
		"total_duration": grandTotal,
		"categories":     items,
	})
}
