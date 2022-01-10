package memongo_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/tryvium-travels/memongo"
	"github.com/tryvium-travels/memongo/memongolog"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func TestDefaultOptions(t *testing.T) {
	versions := []string{"4.4.7", "5.0.0"}

	for _, version := range versions {
		t.Run(version, func(t *testing.T) {
			server, err := memongo.StartWithOptions(&memongo.Options{
				MongoVersion: version,
				LogLevel:     memongolog.LogLevelDebug,
			})
			require.NoError(t, err)
			defer server.Stop()

			client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(server.URI()))
			require.NoError(t, err)

			require.NoError(t, client.Ping(context.Background(), nil))
		})
	}
}

func TestWithReplica(t *testing.T) {
	versions := []string{"4.4.7", "5.0.0"}

	for _, version := range versions {
		t.Run(version, func(t *testing.T) {
			server, err := memongo.StartWithOptions(&memongo.Options{
				MongoVersion:     version,
				LogLevel:         memongolog.LogLevelDebug,
				ShouldUseReplica: true,
			})
			require.NoError(t, err)
			defer server.Stop()

			uri := fmt.Sprintf("%s%s", server.URI(), "/retryWrites=false")
			client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
			if err != nil {
				t.Logf("err Connect: %v", err)
			}

			require.NoError(t, err)
			require.NoError(t, client.Ping(context.Background(), readpref.Primary()))
		})
	}
}

func TestWithAuth(t *testing.T) {
	versions := []string{"4.4.7", "5.0.0"}

	for _, version := range versions {
		t.Run(version, func(t *testing.T) {
			server, err := memongo.StartWithOptions(&memongo.Options{
				MongoVersion: version,
				LogLevel:     memongolog.LogLevelDebug,
				Auth:         true,
			})
			require.NoError(t, err)
			defer server.Stop()

			client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(server.URI()))
			require.NoError(t, err)

			require.NoError(t, client.Ping(context.Background(), nil))

			// Create a default user admin / 12345 to test auth.
			admin := client.Database("admin")
			res := admin.RunCommand(context.Background(), bson.D{
				{Key: "createUser", Value: "admin"},
				{Key: "pwd", Value: "12345"},
				{Key: "roles", Value: []bson.M{
					{"role": "userAdminAnyDatabase", "db": "admin"},
				}},
			})
			require.NoError(t, res.Err())

			// Verify we cannot connect without auth
			client2, err := mongo.Connect(context.Background(), options.Client().ApplyURI(server.URI()))
			require.NoError(t, err)

			require.NoError(t, client2.Ping(context.Background(), nil))
			_, err = client2.ListDatabaseNames(context.Background(), bson.D{})
			require.EqualError(t, err, "(Unauthorized) command listDatabases requires authentication")

			// Now connect again with auth
			opts := options.Client().ApplyURI(server.URI())
			opts.Auth = &options.Credential{
				Username:   "admin",
				Password:   "12345",
				AuthSource: "admin",
			}
			client3, err := mongo.Connect(context.Background(), opts)
			require.NoError(t, err)

			require.NoError(t, client3.Ping(context.Background(), nil))
			_, err = client3.ListDatabaseNames(context.Background(), bson.D{})
			require.NoError(t, err)
		})
	}
}
