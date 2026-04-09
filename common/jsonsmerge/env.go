package jsonsmerge

import "os"

// expandEnv fills environment variables in the value if it's a string, otherwise returns the value as is.
// e.g. "path": "${HOME}/file.txt" will be replaced with the actual path of the file in the user's home directory.
func expandEnv(key string, value interface{}) interface{} {
	if str, ok := value.(string); ok {
		return os.ExpandEnv(str)
	}
	return value
}
