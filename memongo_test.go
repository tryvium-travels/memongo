package memongo_test

import (
	"context"
	"testing"

	"github.com/tryvium-travels/memongo"
	"github.com/tryvium-travels/memongo/memongolog"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
