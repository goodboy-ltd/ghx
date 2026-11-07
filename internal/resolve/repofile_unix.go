//go:build unix

package resolve

import (
	"fmt"
	"os"
	"syscall"
)

func untrustedReason(info os.FileInfo) string {
	return ownershipReason(info, os.Getuid())
}

// ownershipReason rejects a .ghx that another user owns or could rewrite, like ssh does
// for its config files: such a file could switch your gh account.
func ownershipReason(info os.FileInfo, uid int) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "its owner cannot be read"
	}
	if int(stat.Uid) != uid {
		return fmt.Sprintf("owned by uid %d, not by you (uid %d)", stat.Uid, uid)
	}
	mode := info.Mode().Perm()
	if mode&0o002 != 0 {
		return fmt.Sprintf("world-writable, mode %04o", mode)
	}
	if mode&0o020 != 0 {
		return fmt.Sprintf("group-writable, mode %04o", mode)
	}
	return ""
}
