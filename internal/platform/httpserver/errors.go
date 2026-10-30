package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
)

// ErrorHandler turns the last error a handler added with ctx.Error into a JSON response:
//
//	{"errors":{"name":"is required"}}   400, for a body that is malformed or fails validation
//	{"error":"<message>"}               400/401/404/409 for apperrors kinds, 500 for anything else
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// If a handler already wrote a response, don't overwrite it.
		if c.Writer.Written() || len(c.Errors) == 0 {
			return
		}
		err := c.Errors.Last().Err

		switch {
		case handleJSONSyntaxError(c, err), handleJSONTypeError(c, err), handleValidationErrors(c, err), handleAppError(c, err):
		default:
			slog.ErrorContext(c.Request.Context(), "unhandled error", "error", err, "path", c.FullPath())
			writeJSONError(c, http.StatusInternalServerError, "internal server error")
		}
	}
}

// JSON syntax errors → {"errors":{"body":"invalid JSON at byte offset N"}}, or "is required"
// for an empty body and "invalid JSON: unexpected end" for one cut off part-way.
func handleJSONSyntaxError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, io.EOF):
		writeJSONFieldErrors(c, map[string]string{"body": "is required"})
		return true
	case errors.Is(err, io.ErrUnexpectedEOF):
		writeJSONFieldErrors(c, map[string]string{"body": "invalid JSON: unexpected end"})
		return true
	}
	var syn *json.SyntaxError
	if !errors.As(err, &syn) {
		return false
	}
	writeJSONFieldErrors(c, map[string]string{"body": fmt.Sprintf("invalid JSON at byte offset %d", syn.Offset)})
	return true
}

// JSON type errors → {"errors":{"<field>":"invalid type: got \"X\", expected Y"}}
func handleJSONTypeError(c *gin.Context, err error) bool {
	var typ *json.UnmarshalTypeError
	if !errors.As(err, &typ) {
		return false
	}
	field := typ.Field
	if field == "" {
		field = "body"
	}
	writeJSONFieldErrors(c, map[string]string{
		field: fmt.Sprintf("invalid type: got %q, expected %s", typ.Value, typ.Type.String()),
	})
	return true
}

// Binding validation errors → {"errors":{"<json field>":"is required"}}
func handleValidationErrors(c *gin.Context, err error) bool {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return false
	}
	fields := make(map[string]string, len(verrs))
	for _, fe := range verrs {
		// Field() is the JSON name, thanks to the tag name func registered in New.
		fields[fe.Field()] = humanizeValidationMsg(fe)
	}
	writeJSONFieldErrors(c, fields)
	return true
}

func humanizeValidationMsg(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "is required"
	case "min":
		return fmt.Sprintf("must be at least %s", fe.Param())
	case "max":
		return fmt.Sprintf("must be at most %s", fe.Param())
	case "oneof":
		return fmt.Sprintf("must be one of [%s]", fe.Param())
	default:
		return "is invalid"
	}
}

// apperrors kinds → {"error":"<message>"}
func handleAppError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, apperrors.ErrInvalidInput):
		writeJSONError(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, apperrors.ErrUnauthorized):
		c.Header("WWW-Authenticate", "Bearer")
		writeJSONError(c, http.StatusUnauthorized, err.Error())
	case errors.Is(err, apperrors.ErrNotFound):
		writeJSONError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, apperrors.ErrConflict):
		writeJSONError(c, http.StatusConflict, err.Error())
	default:
		return false
	}
	return true
}

func writeJSONError(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message})
}

func writeJSONFieldErrors(c *gin.Context, fields map[string]string) {
	c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"errors": fields})
}
