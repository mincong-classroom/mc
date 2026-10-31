package annotate

import (
	"fmt"
	"image"
	_ "image/gif" // The first frame of a GIF.
	_ "image/jpeg"
	"image/png"
	"os"

	_ "golang.org/x/image/webp"
)

// Load decodes the image of a file: a PNG, a JPEG, a WebP, or the first frame of a GIF.
func Load(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return img, nil
}

// Save writes the image to a PNG file.
func Save(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return fmt.Errorf("encode %s: %w", path, err)
	}
	return f.Close()
}
