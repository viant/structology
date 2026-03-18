package unmarshal

import (
	"fmt"
	"strings"
)

type PathError struct {
	Path string
	Err  error
}

func (e *PathError) Error() string {
	return fmt.Sprintf("failed to unmarshal %s, %v", e.Path, e.Err)
}

func wrapPathError(path string, err error) error {
	if err == nil {
		return nil
	}
	if pathErr, ok := err.(*PathError); ok {
		if pathErr.Path != "" {
			if strings.HasPrefix(pathErr.Path, "[") {
				path = path + pathErr.Path
			} else {
				path = path + "." + pathErr.Path
			}
		}
		err = pathErr.Err
	}
	return &PathError{Path: path, Err: err}
}
