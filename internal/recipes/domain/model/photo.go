package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// MaxPhotoSize is the largest recipe photo accepted, in bytes. The app resizes photos well below
// it before uploading.
const MaxPhotoSize = 10 << 20

var photoExtensions = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/heic": "heic",
	"image/webp": "webp",
}

var (
	ErrUnsupportedPhoto = fmt.Errorf("%w: a photo must be a JPEG, PNG, HEIC or WebP image", apperrors.ErrInvalidInput)
	ErrPhotoTooLarge    = fmt.Errorf("%w: a photo must be at most %d MB", apperrors.ErrInvalidInput, MaxPhotoSize>>20)
	ErrPhotoNotUploaded = fmt.Errorf("%w: the photo hasn't been uploaded, or its upload link was for another recipe", apperrors.ErrInvalidInput)
)

// PhotoUpload is where the app uploads a photo: an HTTP PUT of the image to URL, with Headers,
// before ExpiresAt. Key is what the app then attaches to the recipe.
type PhotoUpload struct {
	Key       string
	URL       string
	Headers   map[string]string
	ExpiresAt time.Time
}

// StoredPhoto is what the photos bucket holds under a key.
type StoredPhoto struct {
	Size        int64
	ContentType string
}

// OwnerPhotoPrefix is where all the owner's photos are kept, so deleting the account can find them.
func OwnerPhotoPrefix(owner owner.ID) string { return "recipes/" + string(owner) + "/" }

func recipePhotoPrefix(owner owner.ID, recipeID uuid.UUID) string {
	return OwnerPhotoPrefix(owner) + recipeID.String() + "/"
}

// NewPhotoKey names a new photo for a recipe. Each upload gets a key of its own, so a cached old
// photo is never shown in place of a new one.
func NewPhotoKey(owner owner.ID, recipeID uuid.UUID, contentType string) (string, error) {
	extension, ok := photoExtensions[contentType]
	if !ok {
		return "", ErrUnsupportedPhoto
	}
	return recipePhotoPrefix(owner, recipeID) + uuid.NewString() + "." + extension, nil
}

// IsPhotoKeyFor says whether key names a photo uploaded for this recipe.
func IsPhotoKeyFor(key string, owner owner.ID, recipeID uuid.UUID) bool {
	rest, ok := strings.CutPrefix(key, recipePhotoPrefix(owner, recipeID))
	return ok && rest != "" && !strings.Contains(rest, "/")
}

// CheckStoredPhoto checks an uploaded photo is one a recipe can show.
func CheckStoredPhoto(photo StoredPhoto) error {
	if _, ok := photoExtensions[photo.ContentType]; !ok {
		return ErrUnsupportedPhoto
	}
	if photo.Size > MaxPhotoSize {
		return ErrPhotoTooLarge
	}
	return nil
}
