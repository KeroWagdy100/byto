//go:build !windows

package command

func ensureUTF8(s string) string {
	return s
}
