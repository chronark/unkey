package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/chronark/unkey/apps/api/pkg/entities"
	"github.com/chronark/unkey/apps/api/pkg/hash"
	"github.com/chronark/unkey/apps/api/pkg/uid"
	"github.com/gofiber/fiber/v2"
)

type CreateKeyRequest struct {
	ApiId      string         `json:"apiId" validate:"required"`
	Prefix     string         `json:"prefix"`
	ByteLength int            `json:"byteLength"`
	OwnerId    string         `json:"ownerId"`
	Meta       map[string]any `json:"meta"`
	Expires    int64          `json:"expires"`
	Ratelimit  *struct {
		Type           string `json:"type"`
		Limit          int64  `json:"limit"`
		RefillRate     int64  `json:"refillRate"`
		RefillInterval int64  `json:"refillInterval"`
	} `json:"ratelimit"`
}

type CreateKeyResponse struct {
	Key   string `json:"key"`
	KeyId string `json:"keyId"`
}

func (s *Server) createKey(c *fiber.Ctx) error {
	ctx := c.UserContext()

	req := CreateKeyRequest{
		// These act as default
		ByteLength: 16,
	}
	err := c.BodyParser(&req)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse{
			Code:  BAD_REQUEST,
			Error: fmt.Sprintf("unable to parse body: %s", err.Error()),
		})
	}

	err = s.validator.Struct(req)
	if err != nil {
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse{
			Code:  BAD_REQUEST,
			Error: fmt.Sprintf("unable to validate body: %s", err.Error()),
		})
	}

	authHash, err := getKeyHash(c.Get("Authorization"))
	if err != nil {
		return err
	}

	authKey, err := s.db.GetKeyByHash(ctx, authHash)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrorResponse{
			Code:  INTERNAL_SERVER_ERROR,
			Error: fmt.Sprintf("unable to find key: %s", err.Error()),
		})
	}

	if authKey.ForWorkspaceId == "" {
		return c.Status(http.StatusBadRequest).JSON(ErrorResponse{
			Code:  BAD_REQUEST,
			Error: "wrong key type",
		})
	}

	if req.Expires > 0 && req.Expires < time.Now().UnixMilli() {
		return c.Status(http.StatusBadRequest).JSON(
			ErrorResponse{
				Code:  BAD_REQUEST,
				Error: "'expires' must be in the future, did you pass in a timestamp in seconds instead of milliseconds?",
			})
	}

	api, err := s.db.GetApi(ctx, req.ApiId)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrorResponse{
			Code:  INTERNAL_SERVER_ERROR,
			Error: fmt.Sprintf("unable to find api: %s", err.Error()),
		})
	}
	if api.WorkspaceId != authKey.ForWorkspaceId {
		return c.Status(http.StatusUnauthorized).JSON(ErrorResponse{
			Code:  UNAUTHORIZED,
			Error: "access to workspace denied",
		})
	}

	keyValue := uid.New(req.ByteLength, req.Prefix)
	split := strings.Split(keyValue, "_")
	keyHash := hash.Sha256(keyValue)

	newKey := entities.Key{
		Id:          uid.Key(),
		ApiId:       req.ApiId,
		WorkspaceId: authKey.WorkspaceId,
		Hash:        keyHash,
		Start:       split[len(split)-1][:4],
		OwnerId:     req.OwnerId,
		Meta:        req.Meta,
		CreatedAt:   time.Now(),
	}
	if req.Expires > 0 {
		newKey.Expires = time.UnixMilli(req.Expires)
	}
	if req.Ratelimit != nil {
		newKey.Ratelimit = &entities.Ratelimit{
			Type:           req.Ratelimit.Type,
			Limit:          req.Ratelimit.Limit,
			RefillRate:     req.Ratelimit.RefillRate,
			RefillInterval: req.Ratelimit.RefillInterval,
		}
	}

	err = s.db.CreateKey(ctx, newKey)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(ErrorResponse{
			Code:  INTERNAL_SERVER_ERROR,
			Error: fmt.Sprintf("unable to store key: %s", err.Error()),
		})
	}

	return c.JSON(CreateKeyResponse{
		Key:   keyValue,
		KeyId: newKey.Id,
	})
}
