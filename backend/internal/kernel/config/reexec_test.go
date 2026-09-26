package config

import (
	"os"
	"os/exec"
	"testing"
)

// reexecSelf 以 TEST_LOAD_FAIL=1 重新运行本测试二进制，触发 Fatal 退出分支。
func reexecSelf(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run", "^TestLoadMissingFileOrSecretExits$")
	cmd.Env = append(os.Environ(), "TEST_LOAD_FAIL=1")
	return cmd
}
