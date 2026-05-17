package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var validKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type createCustomFieldRequest struct {
	Name         string          `json:"name"          binding:"required"`
	Key          string          `json:"key"           binding:"required"`
	FieldType    string          `json:"field_type"    binding:"required"`
	Options      json.RawMessage `json:"options"`
	DisplayOrder int             `json:"display_order"`
}

type reorderItem struct {
	ID    uuid.UUID `json:"id"    binding:"required"`
	Order int       `json:"order"`
}

func ListCustomFields(repo repository.CustomFieldRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		fields, err := repo.List()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list custom fields"})
			return
		}
		if fields == nil {
			fields = []models.CustomFieldDefinition{}
		}
		c.JSON(http.StatusOK, fields)
	}
}

func CreateCustomField(repo repository.CustomFieldRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createCustomFieldRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if !validKeyPattern.MatchString(req.Key) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "key must be snake_case starting with a letter"})
			return
		}

		switch req.FieldType {
		case models.FieldTypeString, models.FieldTypeNumber, models.FieldTypeDropdown:
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "field_type must be string, number, or dropdown"})
			return
		}

		if req.FieldType == models.FieldTypeDropdown {
			var opts []models.DropdownOption
			if len(req.Options) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "dropdown fields require at least one option"})
				return
			}
			if err := json.Unmarshal(req.Options, &opts); err != nil || len(opts) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "dropdown fields require at least one option"})
				return
			}
		}

		options := req.Options
		if len(options) == 0 {
			options = json.RawMessage("[]")
		}

		def := &models.CustomFieldDefinition{
			Name:         req.Name,
			Key:          req.Key,
			FieldType:    req.FieldType,
			Options:      options,
			DisplayOrder: req.DisplayOrder,
		}

		if err := repo.Create(def); err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "a field with that key already exists"})
			return
		}

		c.JSON(http.StatusCreated, def)
	}
}

func UpdateCustomField(repo repository.CustomFieldRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		var req createCustomFieldRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if !validKeyPattern.MatchString(req.Key) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "key must be snake_case starting with a letter"})
			return
		}

		switch req.FieldType {
		case models.FieldTypeString, models.FieldTypeNumber, models.FieldTypeDropdown:
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "field_type must be string, number, or dropdown"})
			return
		}

		options := req.Options
		if len(options) == 0 {
			options = json.RawMessage("[]")
		}

		def := &models.CustomFieldDefinition{
			ID:           id,
			Name:         req.Name,
			Key:          req.Key,
			FieldType:    req.FieldType,
			Options:      options,
			DisplayOrder: req.DisplayOrder,
		}

		if err := repo.Update(def); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update custom field"})
			return
		}

		c.JSON(http.StatusOK, def)
	}
}

func DeleteCustomField(repo repository.CustomFieldRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		// Look up the field to get its key for usage check
		var key string
		fields, listErr := repo.List()
		if listErr == nil {
			for _, f := range fields {
				if f.ID == id {
					key = f.Key
					break
				}
			}
		}

		if key != "" {
			count, countErr := repo.CountUsage(key)
			if countErr == nil && count > 0 {
				c.JSON(http.StatusConflict, gin.H{"error": "field is in use", "count": count})
				return
			}
		}

		if err := repo.Delete(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete custom field"})
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func ReorderCustomFields(repo repository.CustomFieldRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var items []reorderItem
		if err := c.ShouldBindJSON(&items); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		reorderItems := make([]repository.ReorderItem, len(items))
		for i, item := range items {
			reorderItems[i] = repository.ReorderItem{ID: item.ID, Order: item.Order}
		}

		if err := repo.Reorder(reorderItems); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reorder custom fields"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}
