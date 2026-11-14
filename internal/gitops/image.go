package gitops

import (
	"fmt"
	"path"
	"strings"
)

type imageRef struct {
	name        string
	tag         string
	placeholder string
}

func parseImage(s string) (imageRef, error) {
	if strings.Contains(s, "@") {
		return imageRef{}, fmt.Errorf("image %q: digests not supported, use a tag", s)
	}

	var i imageRef
	slashIndex := strings.LastIndex(s, "/")
	colonIndex := strings.LastIndex(s, ":")

	// No colon after the slash means there is no tag
	if colonIndex <= slashIndex {
		return imageRef{}, fmt.Errorf("image %q must contain a tag", s)
	}

	i.name = s[:colonIndex]
	i.tag = s[colonIndex+1:]

	if i.tag == "" {
		return imageRef{}, fmt.Errorf("image %q: tag cannot be empty", s)
	}

	i.placeholder = path.Base(i.name)

	return i, nil
}
