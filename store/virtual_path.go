package store

import "strings"

type VirtualPath struct {
	CollectionName string
	Path           string
}

func NormalizeVirtualPath(input string) string {
	if strings.HasPrefix(input, "qmd://") {
		withoutPrefix := strings.TrimPrefix(input, "qmd://")
		// Trim leading slashes from collection name
		for strings.HasPrefix(withoutPrefix, "/") {
			withoutPrefix = strings.TrimPrefix(withoutPrefix, "/")
		}
		return "qmd://" + withoutPrefix
	}
	return input
}

func ParseVirtualPath(virtualPath string) *VirtualPath {
	normalized := NormalizeVirtualPath(virtualPath)
	if !strings.HasPrefix(normalized, "qmd://") {
		return nil
	}
	path := strings.TrimPrefix(normalized, "qmd://")
	slashIdx := strings.Index(path, "/")
	if slashIdx == -1 {
		return &VirtualPath{CollectionName: path, Path: ""}
	}
	return &VirtualPath{
		CollectionName: path[:slashIdx],
		Path:           path[slashIdx+1:],
	}
}

func BuildVirtualPath(collectionName, path string) string {
	return "qmd://" + collectionName + "/" + path
}

func IsVirtualPath(path string) bool {
	return strings.HasPrefix(path, "qmd://")
}
