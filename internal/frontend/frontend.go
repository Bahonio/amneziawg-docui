// Derived from mycelium-mesh/amneziawg-ui (Apache-2.0) and modified by Bahonio.
// SPDX-License-Identifier: Apache-2.0 AND AGPL-3.0-or-later

// Package frontend serves the embedded AWG DocUI browser application.
package frontend

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"path"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/static"
)

// Assets owns a read-only asset filesystem and content-derived validators.
type Assets struct {
	files fs.FS
	tags  map[string]string
}

func Open(files fs.FS) *Assets {
	return &Assets{files: files, tags: assetTags(files)}
}

func (a *Assets) Compression() fiber.Handler {
	return compress.New(compress.Config{Level: compress.LevelBestSpeed})
}

// Mount registers the static application after the REST routes.
func (a *Assets) Mount(app *fiber.App) {
	app.Use(a.cache())
	app.Get("/*", static.New("", static.Config{
		FS:         a.files,
		IndexNames: []string{"index.html"},
		Browse:     false,
	}))
}

func assetTags(files fs.FS) map[string]string {
	tags := map[string]string{}
	_ = fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		tag, hashErr := fileETag(files, name)
		if hashErr != nil {
			return hashErr
		}
		tags["/"+strings.TrimPrefix(path.Clean(name), "./")] = tag
		return nil
	})
	tags["/"] = tags["/index.html"]
	return tags
}

func (a *Assets) cache() fiber.Handler {
	return func(c fiber.Ctx) error {
		tag, ok := a.tags[c.Path()]
		if !ok || c.Method() != fiber.MethodGet {
			return c.Next()
		}
		c.Set(fiber.HeaderCacheControl, "no-cache")
		c.Set(fiber.HeaderETag, tag)
		if etagMatches(c.Get(fiber.HeaderIfNoneMatch), tag) {
			return c.SendStatus(fiber.StatusNotModified)
		}
		c.Request().Header.Del(fiber.HeaderIfModifiedSince)
		err := c.Next()
		c.Response().Header.Del(fiber.HeaderLastModified)
		return err
	}
}

func etagMatches(header, tag string) bool {
	tag = strings.TrimPrefix(tag, "W/")
	for candidate := range strings.SplitSeq(header, ",") {
		candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "W/")
		if candidate == "*" || candidate == tag {
			return true
		}
	}
	return false
}

func fileETag(files fs.FS, name string) (string, error) {
	file, err := files.Open(name)
	if err != nil {
		return "", err
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return "", err
	}
	return `W/"` + hex.EncodeToString(sum.Sum(nil)[:8]) + `"`, nil
}
