//go:build !unix

package resolve

import "os"

// untrustedReason trusts every .ghx: Windows has no uid or mode bits to check.
func untrustedReason(os.FileInfo) string {
	return ""
}
