package setup

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

func AddToPath(dir string) error {
	if runtime.GOOS != "windows" {
		return nil
	}
	powershell := fmt.Sprintf(
		`$dir='%s';$p=[Environment]::GetEnvironmentVariable('PATH','User');$f=$false;foreach($x in ($p -split ';')){if($x -eq $dir){$f=$true;break}};if(-not$f){[Environment]::SetEnvironmentVariable('PATH',$p+';'+$dir,'User')}`,
		dir,
	)
	cmd := exec.Command("powershell", "-NoProfile", "-Command", powershell)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func CheckInPath(dir string) bool {
	if runtime.GOOS != "windows" {
		return true
	}
	currentPath := os.Getenv("PATH")
	for _, p := range strings.Split(currentPath, ";") {
		if strings.EqualFold(strings.TrimSpace(p), dir) {
			return true
		}
	}
	return false
}
