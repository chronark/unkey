package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unkeyed/unkey/apps/api/pkg/cache"
	"github.com/unkeyed/unkey/apps/api/pkg/database"
	"github.com/unkeyed/unkey/apps/api/pkg/entities"
	"github.com/unkeyed/unkey/apps/api/pkg/logging"
	"github.com/unkeyed/unkey/apps/api/pkg/testutil"
	"github.com/unkeyed/unkey/apps/api/pkg/tracing"
	"github.com/unkeyed/unkey/apps/api/pkg/uid"
)

func TestGetApi_Exists(t *testing.T) {

	resources := testutil.SetupResources(t)

	db, err := database.New(database.Config{
		Logger: logging.NewNoopLogger(),

		PrimaryUs: os.Getenv("DATABASE_DSN"),
	})
	require.NoError(t, err)

	srv := New(Config{
		Logger:   logging.NewNoopLogger(),
		KeyCache: cache.NewNoopCache[entities.Key](),
		ApiCache: cache.NewNoopCache[entities.Api](),
		Database: db,
		Tracer:   tracing.NewNoop(),
	})

	req := httptest.NewRequest("GET", fmt.Sprintf("/v1/apis/%s", resources.UserApi.Id), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", resources.UnkeyKey))

	res, err := srv.app.Test(req)
	require.NoError(t, err)
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, 200, res.StatusCode)

	successResponse := GetApiResponse{}
	err = json.Unmarshal(body, &successResponse)
	require.NoError(t, err)

	require.Equal(t, resources.UserApi.Id, successResponse.Id)
	require.Equal(t, resources.UserApi.Name, successResponse.Name)
	require.Equal(t, resources.UserApi.WorkspaceId, successResponse.WorkspaceId)

}

func TestGetApi_NotFound(t *testing.T) {

	resources := testutil.SetupResources(t)

	srv := New(Config{
		Logger:   logging.NewNoopLogger(),
		KeyCache: cache.NewNoopCache[entities.Key](),
		ApiCache: cache.NewNoopCache[entities.Api](),
		Database: resources.Database,
		Tracer:   tracing.NewNoop(),
	})

	fakeApiId := uid.Api()

	req := httptest.NewRequest("GET", fmt.Sprintf("/v1/apis/%s", fakeApiId), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", resources.UnkeyKey))

	res, err := srv.app.Test(req)
	require.NoError(t, err)
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)

	require.Equal(t, 404, res.StatusCode)

	errorResponse := ErrorResponse{}
	err = json.Unmarshal(body, &errorResponse)
	require.NoError(t, err)

	require.Equal(t, NOT_FOUND, errorResponse.Code)
	require.Equal(t, fmt.Sprintf("unable to find api: %s", fakeApiId), errorResponse.Error)

}

func TestGetApi_WithIpWhitelist(t *testing.T) {
	ctx := context.Background()
	resources := testutil.SetupResources(t)

	db, err := database.New(database.Config{
		Logger:    logging.NewNoopLogger(),
		PrimaryUs: os.Getenv("DATABASE_DSN"),
	})
	require.NoError(t, err)

	srv := New(Config{
		Logger:   logging.NewNoopLogger(),
		KeyCache: cache.NewNoopCache[entities.Key](),
		ApiCache: cache.NewNoopCache[entities.Api](),
		Database: db,
		Tracer:   tracing.NewNoop(),
	})

	api := entities.Api{
		Id:          uid.Api(),
		Name:        "test",
		WorkspaceId: resources.UserWorkspace.Id,
		IpWhitelist: []string{"127.0.0.1", "1.1.1.1"},
	}

	err = db.CreateApi(ctx, api)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", fmt.Sprintf("/v1/apis/%s", api.Id), nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", resources.UnkeyKey))

	res, err := srv.app.Test(req)
	require.NoError(t, err)
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, 200, res.StatusCode)

	successResponse := GetApiResponse{}
	err = json.Unmarshal(body, &successResponse)
	require.NoError(t, err)

	require.Equal(t, api.Id, successResponse.Id)
	require.Equal(t, api.Name, successResponse.Name)
	require.Equal(t, api.WorkspaceId, successResponse.WorkspaceId)
	require.Equal(t, api.IpWhitelist, successResponse.IpWhitelist)

}
