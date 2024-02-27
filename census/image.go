package census

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Travis-Britz/ps2"
)

var DefaultHTTPClient interface {
	Get(string) (*http.Response, error)
} = http.DefaultClient

// Image represents the image struct returned by Census.
type Image struct {
	ImageID     ps2.ImageID `json:"image_id,string"`
	Description string      `json:"description,omitempty"`
	Path        string      `json:"path"`
}

func (i Image) ImageURL() string { return apiBase + i.Path }

// ImageSet represents the image_set collection results,
// which appear to be unique on (ImageSetID, ImageID, TypeID)
type ImageSet struct {
	ImageSetID      ps2.ImageSetID  `json:"image_set_id"`
	ImageID         ps2.ImageID     `json:"image_id"`
	Description     string          `json:"description"`
	TypeID          ps2.ImageTypeID `json:"type_id"`
	TypeDescription string          `json:"type_description"`
	ImagePath       string          `json:"image_path"`
}

func (ImageSet) CollectionName() string { return "image_set" }
func (i ImageSet) ImageURL() string     { return apiBase + i.ImagePath }

// ImageSetDefault represents the image_set_default collection.
// The structure is the same as the image_set collection,
// but the only the default image for each image set is returned.
type ImageSetDefault ImageSet

func (ImageSetDefault) CollectionName() string { return "image_set_default" }

func (i Image) WriteTo(w io.Writer) (int64, error) {
	resp, err := DefaultHTTPClient.Get(i.ImageURL())
	if err != nil {
		return 0, fmt.Errorf("census.Image.WriteTo: image %d: http.Get: %w", i.ImageID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("census.Image.WriteTo: expected HTTP status code 200; got %q", resp.Status)
	}
	responseType := resp.Header.Get("content-type")
	if !strings.HasPrefix(responseType, "image/") {
		return 0, fmt.Errorf("census.Image.WriteTo: expected HTTP Content-Type response header to be an image; got %q", responseType)
	}
	// var b []byte // TODO: buffer the first 512 bytes
	// responseType = http.DetectContentType(b)
	// if !strings.HasPrefix(responseType, "image/") {
	// 	return 0, fmt.Errorf("census.Image.WriteTo: expected HTTP response to contain an image; got %q", responseType)
	// }

	return io.Copy(w, resp.Body)
}
