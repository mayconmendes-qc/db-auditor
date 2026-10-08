package mongodb

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestInspectCatalogWithoutReadingDocuments(t *testing.T) {
	uri := os.Getenv("AUDITOR_MONGO_TEST_URI")
	if uri == "" {
		t.Skip("AUDITOR_MONGO_TEST_URI não configurada")
	}
	u, err := url.Parse(uri)
	if err != nil || !strings.HasSuffix(u.Path, "_ci") {
		t.Fatal("integration test requires an isolated _ci database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Disconnect(context.WithoutCancel(ctx)) }()
	db := client.Database(strings.TrimPrefix(u.Path, "/"))
	name := "audit_catalog_test_" + strings.ReplaceAll(time.Now().UTC().Format("150405.000000"), ".", "")
	collection := db.Collection(name)
	if _, err := collection.InsertOne(ctx, bson.M{"private_value": "must-never-appear", "tenant": 1}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = collection.Drop(context.Background()) }()
	if _, err := collection.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "tenant", Value: 1}}}); err != nil {
		t.Fatal(err)
	}
	before, err := collection.CountDocuments(ctx, bson.D{})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := Inspect(ctx, uri, db.Name())
	if err != nil {
		t.Fatal(err)
	}
	after, err := collection.CountDocuments(ctx, bson.D{})
	if err != nil || after != before {
		t.Fatalf("collector changed target data: before=%d after=%d err=%v", before, after, err)
	}
	for _, item := range catalog.Collections {
		if item.Name != name {
			continue
		}
		if item.Documents != before || len(item.Indexes) != 2 {
			t.Fatalf("unexpected catalog facts: %+v", item)
		}
		for _, index := range item.Indexes {
			if strings.Contains(index.Definition, "must-never-appear") || strings.Contains(index.Definition, "private_value") {
				t.Fatal("document values or fields leaked into index metadata")
			}
		}
		return
	}
	t.Fatal("test collection missing from catalog")
}
