package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://oad:oad@localhost:5432/oad?sslmode=disable"
	}

	ctx := context.Background()

	var conn *pgx.Conn
	var err error

	// Wait for DB to be available
	for i := 0; i < 60; i++ {
		conn, err = pgx.Connect(ctx, dbURL)
		if err == nil && conn.Ping(ctx) == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	log.Println("Connected to database. Waiting for SCIM sync to complete...")

	// The blueprint has 4 users and 3 groups = 7 external identities for 'authentik' provider.
	expectedIdentities := 7
	timeout := time.After(120 * time.Second)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var count int
	syncComplete := false

	for !syncComplete {
		select {
		case <-timeout:
			log.Fatalf("Timeout waiting for SCIM sync. Found %d/%d identities.", count, expectedIdentities)
		case <-ticker.C:
			err = conn.QueryRow(ctx, "SELECT count(*) FROM entity_external_identity WHERE provider_name = 'authentik'").Scan(&count)
			if err != nil {
				log.Fatalf("Error querying count: %v", err)
			}
			log.Printf("Found %d/%d identities...", count, expectedIdentities)
			if count >= expectedIdentities {
				syncComplete = true
			}
		}
	}

	log.Println("SCIM sync complete. Running assertions...")

	// 1. Assert Users exist
	expectedUsers := []string{
		"admin@oad.dev",
		"editor@oad.dev",
		"viewer@oad.dev",
		"pdp@oad.dev",
	}

	for _, userName := range expectedUsers {
		var entityID string
		err = conn.QueryRow(ctx, `
			SELECT e.id
			FROM entity e
			WHERE e.properties->>'userName' = $1
		`, userName).Scan(&entityID)
		if err != nil {
			log.Fatalf("Assertion failed: user with userName %s not found: %v", userName, err)
		}
		log.Printf("✔ User %s found (Entity ID: %s)", userName, entityID)
	}

	// 2. Assert Groups exist
	expectedGroups := []string{
		"oad-admin",
		"oad-editor",
		"oad-viewer",
	}

	for _, groupName := range expectedGroups {
		var entityID string
		err = conn.QueryRow(ctx, `
			SELECT e.id
			FROM entity e
			WHERE e.properties->>'displayName' = $1
		`, groupName).Scan(&entityID)
		if err != nil {
			log.Fatalf("Assertion failed: group with displayName %s not found: %v", groupName, err)
		}
		log.Printf("✔ Group %s found (Entity ID: %s)", groupName, entityID)
	}

	// 3. Assert Relations
	// admin@oad.dev -> member_of -> oad-admin
	// editor@oad.dev -> member_of -> oad-editor
	// viewer@oad.dev -> member_of -> oad-viewer
	// pdp@oad.dev -> member_of -> oad-viewer

	relationsToAssert := []struct {
		userName  string
		groupName string
	}{
		{"admin@oad.dev", "oad-admin"},
		{"editor@oad.dev", "oad-editor"},
		{"viewer@oad.dev", "oad-viewer"},
		{"pdp@oad.dev", "oad-viewer"},
	}

	for _, rel := range relationsToAssert {
		var exists bool
		err = conn.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1
				FROM relation r
				JOIN entity subj ON r.subject_entity_id = subj.id
				JOIN entity tgt ON r.target_entity_id = tgt.id
				WHERE r.relation_type = 'member_of'
				  AND subj.properties->>'userName' = $1
				  AND tgt.properties->>'displayName' = $2
			)
		`, rel.userName, rel.groupName).Scan(&exists)
		
		if err != nil {
			log.Fatalf("Error querying relation %s -> %s: %v", rel.userName, rel.groupName, err)
		}
		if !exists {
			log.Fatalf("Assertion failed: relation member_of from %s to %s not found", rel.userName, rel.groupName)
		}
		log.Printf("✔ Relation member_of %s -> %s found", rel.userName, rel.groupName)
	}

	log.Println("All assertions passed successfully!")
}
