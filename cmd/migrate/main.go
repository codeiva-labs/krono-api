// Command migrate is a one-off tool, run manually once, that:
//  1. strips the now-unused "description" field from every category document
//  2. gives every existing user their own owned copy of the default categories,
//     re-pointing their existing subcategories/activities off the old shared
//     (unowned) main categories and onto their new owned ones
//
// Usage: go run ./cmd/migrate
package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"codeiva/krono-api/app/db"
	"codeiva/krono-api/app/model"
	"codeiva/krono-api/config"
)

// normalizeName collapses whitespace differences (e.g. "Social Media/ TV/ Radio"
// vs "Social Media / TV / Radio") so old-name -> new-name matching isn't broken
// by cosmetic spacing changes to the default category list over time.
func normalizeName(name string) string {
	return strings.Join(strings.Fields(name), "")
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf(".env file not found, proceeding with environment variables")
	}

	cfg := config.GetConfig()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := cfg.ConnectMongo(ctx)
	if err != nil {
		log.Fatalf("could not connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	database := cfg.Database(client)
	collections := model.NewCollections(database)

	if err := stripDescriptionField(ctx, collections); err != nil {
		log.Fatalf("failed to strip description field: %v", err)
	}

	if err := migrateUsersToOwnedCategories(ctx, collections); err != nil {
		log.Fatalf("failed to migrate users to owned categories: %v", err)
	}

	log.Println("migration complete")
}

// stripDescriptionField removes the legacy "description" field from every category document.
func stripDescriptionField(ctx context.Context, collections *model.Collections) error {
	result, err := collections.Categories.UpdateMany(ctx,
		bson.M{"description": bson.M{"$exists": true}},
		bson.M{"$unset": bson.M{"description": ""}},
	)
	if err != nil {
		return err
	}
	log.Printf("stripped description field from %d categories", result.ModifiedCount)
	return nil
}

// migrateUsersToOwnedCategories gives every user who doesn't already own a main
// category their own copy of the defaults, then re-points any of that user's
// subcategories/activities that reference the old shared main categories.
func migrateUsersToOwnedCategories(ctx context.Context, collections *model.Collections) error {
	// Legacy shared main categories (is_main:true, user_id:nil), keyed by ID -> name
	legacyCursor, err := collections.Categories.Find(ctx, bson.M{"is_main": true, "user_id": nil})
	if err != nil {
		return err
	}
	var legacy []model.Category
	if err := legacyCursor.All(ctx, &legacy); err != nil {
		return err
	}

	legacyNameByID := make(map[primitive.ObjectID]string, len(legacy))
	legacyIDs := make([]primitive.ObjectID, 0, len(legacy))
	for _, c := range legacy {
		legacyNameByID[c.ID] = c.Name
		legacyIDs = append(legacyIDs, c.ID)
	}
	log.Printf("found %d legacy shared main categories", len(legacy))

	userCursor, err := collections.Users.Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer userCursor.Close(ctx)

	var usersMigrated, usersSkipped, subsRepointed, activitiesRepointed int

	for userCursor.Next(ctx) {
		var u model.User
		if err := userCursor.Decode(&u); err != nil {
			return err
		}

		nameToNewID, seeded, err := ensureOwnedCategories(ctx, collections, u.ID)
		if err != nil {
			return err
		}
		if seeded {
			usersMigrated++
		} else {
			usersSkipped++
		}

		if len(legacyIDs) > 0 {
			n, err := repointCategoryParents(ctx, collections, u.ID, legacyIDs, legacyNameByID, nameToNewID)
			if err != nil {
				return err
			}
			subsRepointed += n

			n, err = repointActivityCategories(ctx, collections, u.ID, legacyIDs, legacyNameByID, nameToNewID)
			if err != nil {
				return err
			}
			activitiesRepointed += n
		}
	}
	if err := userCursor.Err(); err != nil {
		return err
	}

	log.Printf("users migrated: %d, users already owned (skipped): %d, subcategories re-pointed: %d, activities re-pointed: %d",
		usersMigrated, usersSkipped, subsRepointed, activitiesRepointed)
	return nil
}

// ensureOwnedCategories returns a normalized-name -> owned-main-category-id map for the
// user, seeding fresh defaults if they own none yet, or reading their existing owned mains
// otherwise. The bool return reports whether a fresh seed happened (true) or the user
// already owned categories (false) - repointing must run either way, since "already owns
// some main categories" doesn't imply "already repointed" (e.g. a user seeded after the
// category rework can still have older activities/subcategories referencing the pre-rework
// shared categories).
func ensureOwnedCategories(ctx context.Context, collections *model.Collections, userID primitive.ObjectID) (map[string]primitive.ObjectID, bool, error) {
	cursor, err := collections.Categories.Find(ctx, bson.M{"user_id": userID, "is_main": true})
	if err != nil {
		return nil, false, err
	}
	var owned []model.Category
	if err := cursor.All(ctx, &owned); err != nil {
		return nil, false, err
	}

	if len(owned) > 0 {
		nameToID := make(map[string]primitive.ObjectID, len(owned))
		for _, c := range owned {
			nameToID[normalizeName(c.Name)] = c.ID
		}
		return nameToID, false, nil
	}

	nameToID, err := seedOwnedCategories(ctx, collections, userID)
	if err != nil {
		return nil, false, err
	}
	return nameToID, true, nil
}

// seedOwnedCategories inserts the user's owned copy of the defaults and returns a
// normalized-name -> new id map.
func seedOwnedCategories(ctx context.Context, collections *model.Collections, userID primitive.ObjectID) (map[string]primitive.ObjectID, error) {
	now := time.Now().UTC()
	docs := make([]interface{}, len(db.MainCategories))
	for i, cat := range db.MainCategories {
		c := cat
		c.ID = primitive.NilObjectID
		c.UserID = &userID
		c.CreatedAt = now
		c.UpdatedAt = now
		docs[i] = c
	}

	result, err := collections.Categories.InsertMany(ctx, docs)
	if err != nil {
		return nil, err
	}

	nameToNewID := make(map[string]primitive.ObjectID, len(db.MainCategories))
	for i, id := range result.InsertedIDs {
		nameToNewID[normalizeName(db.MainCategories[i].Name)] = id.(primitive.ObjectID)
	}
	return nameToNewID, nil
}

// repointCategoryParents updates this user's subcategories whose parent_id is one of the
// legacy shared mains to instead point at the user's new owned equivalent (matched by name).
func repointCategoryParents(ctx context.Context, collections *model.Collections, userID primitive.ObjectID, legacyIDs []primitive.ObjectID, legacyNameByID map[primitive.ObjectID]string, nameToNewID map[string]primitive.ObjectID) (int, error) {
	cursor, err := collections.Categories.Find(ctx, bson.M{"user_id": userID, "parent_id": bson.M{"$in": legacyIDs}})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var subs []model.Category
	if err := cursor.All(ctx, &subs); err != nil {
		return 0, err
	}

	count := 0
	for _, sub := range subs {
		newID, ok := nameToNewID[normalizeName(legacyNameByID[*sub.ParentID])]
		if !ok {
			continue
		}
		if _, err := collections.Categories.UpdateOne(ctx,
			bson.M{"_id": sub.ID},
			bson.M{"$set": bson.M{"parent_id": newID}},
		); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// repointActivityCategories updates this user's activities that reference a legacy shared
// main category directly (no subcategory) to instead reference the user's owned equivalent.
func repointActivityCategories(ctx context.Context, collections *model.Collections, userID primitive.ObjectID, legacyIDs []primitive.ObjectID, legacyNameByID map[primitive.ObjectID]string, nameToNewID map[string]primitive.ObjectID) (int, error) {
	cursor, err := collections.Activities.Find(ctx, bson.M{"user_id": userID, "category_id": bson.M{"$in": legacyIDs}})
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var activities []model.Activity
	if err := cursor.All(ctx, &activities); err != nil {
		return 0, err
	}

	count := 0
	for _, act := range activities {
		newID, ok := nameToNewID[normalizeName(legacyNameByID[act.CategoryID])]
		if !ok {
			continue
		}
		if _, err := collections.Activities.UpdateOne(ctx,
			bson.M{"_id": act.ID},
			bson.M{"$set": bson.M{"category_id": newID}},
		); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
