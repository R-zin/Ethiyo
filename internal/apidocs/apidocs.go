// Package apidocs serves Ethiyo's OpenAPI 3.1 description. Tests in this
// package and in internal/server keep it in lock-step with the code: every
// /api/v1 route must be documented (and vice versa), and documented schemas
// must match the Go models' JSON fields exactly.
package apidocs

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed openapi.json
var spec []byte

// Spec returns the raw OpenAPI document.
func Spec() []byte { return spec }

// Handler serves the OpenAPI document.
func Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=300")
		c.Data(http.StatusOK, "application/json; charset=utf-8", spec)
	}
}
