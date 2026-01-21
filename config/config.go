package config

import (
	"context"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Config struct {
	DB *DBConfig
}

type DBConfig struct {
	// MongoURI is the full MongoDB connection string, e.g. mongodb://localhost:27017
	MongoURI string
	// Database name to use
	Database string
	// ConnectTimeout for establishing connections (in seconds)
	ConnectTimeout int
}

// GetConfig returns default configuration. Environment variables override defaults:
// - MONGO_URI
// - MONGO_DB
// - MONGO_CONNECT_TIMEOUT (seconds)
func GetConfig() *Config {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	db := os.Getenv("MONGO_DB")
	if db == "" {
		db = "krono"
	}

	timeout := 10
	if v := os.Getenv("MONGO_CONNECT_TIMEOUT"); v != "" {
		// ignore parse errors and keep default if invalid
		if t, err := time.ParseDuration(v + "s"); err == nil {
			timeout = int(t / time.Second)
		}
	}

	return &Config{
		DB: &DBConfig{
			MongoURI:       uri,
			Database:       db,
			ConnectTimeout: timeout,
		},
	}
}

// ConnectMongo creates a new *mongo.Client using the configuration and verifies the connection.
func (c *Config) ConnectMongo(ctx context.Context) (*mongo.Client, error) {
	clientOpts := options.Client().ApplyURI(c.DB.MongoURI)

	// derive a context with timeout for connecting
	ct := time.Duration(c.DB.ConnectTimeout) * time.Second
	ctxConnect, cancel := context.WithTimeout(ctx, ct)
	defer cancel()

	client, err := mongo.Connect(ctxConnect, clientOpts)
	if err != nil {
		return nil, err
	}

	// Ping to verify
	pingCtx, pingCancel := context.WithTimeout(ctx, ct)
	defer pingCancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, err
	}

	return client, nil
}

// Database returns a *mongo.Database for the configured database name.
func (c *Config) Database(client *mongo.Client) *mongo.Database {
	return client.Database(c.DB.Database)
}
