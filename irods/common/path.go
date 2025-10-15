package common

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"golang.org/x/xerrors"
)

func MakeIRODSPath(filesystem *irodsclient_fs.FileSystem, irodsPath string) string {
	account := filesystem.GetAccount()
	homePath := account.GetHomeDirPath()
	if account.IsAnonymousUser() {
		homePath = GetSharedPath()
	}

	return makeIRODSPath(homePath, homePath, account.ClientZone, irodsPath)
}

func makeIRODSPath(cwd string, homedir string, zone string, irodsPath string) string {
	irodsPath = strings.TrimPrefix(irodsPath, "i:")

	if strings.HasPrefix(irodsPath, fmt.Sprintf("/%s/~", zone)) {
		// compat to icommands
		// relative path from user's home
		partLen := 3 + len(zone)
		newPath := path.Join(homedir, irodsPath[partLen:])
		return path.Clean(newPath)
	}

	if strings.HasPrefix(irodsPath, "/") {
		// absolute path
		return path.Clean(irodsPath)
	}

	if strings.HasPrefix(irodsPath, "~") {
		// relative path from user's home
		newPath := path.Join(homedir, irodsPath[1:])
		return path.Clean(newPath)
	}

	// relative path from current woring dir
	newPath := path.Join(cwd, irodsPath)
	return path.Clean(newPath)
}

func MakeLocalPath(localPath string) string {
	absLocalPath, err := filepath.Abs(localPath)
	if err != nil {
		return filepath.Clean(localPath)
	}

	return filepath.Clean(absLocalPath)
}

func MakeTargetIRODSFilePath(filesystem *irodsclient_fs.FileSystem, source string, target string) string {
	if filesystem.ExistsDir(target) {
		// make full file name for target
		filename := GetBasename(source)
		return path.Join(target, filename)
	}
	return target
}

func MakeTargetLocalFilePath(source string, target string) string {
	realTarget, err := ResolveSymlink(target)
	if err != nil {
		return target
	}

	st, err := os.Stat(realTarget)
	if err == nil {
		if st.IsDir() {
			// make full file name for target
			filename := GetBasename(source)
			return filepath.Join(target, filename)
		}
	}
	return target
}

func GetFileExtension(p string) string {
	base := GetBasename(p)

	idx := strings.Index(base, ".")
	if idx >= 0 {
		return p[idx:]
	}
	return p
}

func GetIRODSPathDirname(path string) string {
	p := strings.TrimRight(path, "/")
	idx := strings.LastIndex(p, "/")

	if idx < 0 {
		return p
	} else if idx == 0 {
		return "/"
	} else {
		return p[:idx]
	}
}

func GetIRODSPathBasename(path string) string {
	p := strings.TrimRight(path, "/")
	idx := strings.LastIndex(p, "/")

	if idx < 0 {
		return p
	} else {
		return p[idx+1:]
	}
}

func GetBasename(p string) string {
	p = strings.TrimRight(p, string(os.PathSeparator))
	p = strings.TrimRight(p, "/")

	idx1 := strings.LastIndex(p, string(os.PathSeparator))
	idx2 := strings.LastIndex(p, "/")

	if idx1 < 0 && idx2 < 0 {
		return p
	}

	if idx1 >= idx2 {
		return p[idx1+1:]
	}
	return p[idx2+1:]
}

func FirstDelimeterIndex(p string) int {
	idx1 := strings.Index(p, string(os.PathSeparator))
	idx2 := strings.Index(p, "/")

	if idx1 < 0 && idx2 < 0 {
		return idx1
	}

	if idx1 < 0 {
		return idx2
	}

	if idx2 < 0 {
		return idx1
	}

	if idx1 <= idx2 {
		return idx1
	}

	return idx2
}

func LastDelimeterIndex(p string) int {
	idx1 := strings.LastIndex(p, string(os.PathSeparator))
	idx2 := strings.LastIndex(p, "/")

	if idx1 >= idx2 {
		return idx1
	}

	return idx2
}

func GetDir(p string) string {
	idx1 := strings.LastIndex(p, string(os.PathSeparator))
	idx2 := strings.LastIndex(p, "/")

	if idx1 < 0 && idx2 < 0 {
		return "/"
	}

	if idx1 >= idx2 {
		return p[:idx1]
	}
	return p[:idx2]
}

func ExpandHomeDir(p string) (string, error) {
	// resolve "~/"
	if p == "~" {
		homedir, err := os.UserHomeDir()
		if err != nil {
			return "", xerrors.Errorf("failed to get user home directory: %w", err)
		}

		return filepath.Abs(homedir)
	} else if strings.HasPrefix(p, "~/") {
		homedir, err := os.UserHomeDir()
		if err != nil {
			return "", xerrors.Errorf("failed to get user home directory: %w", err)
		}

		p = filepath.Join(homedir, p[2:])
		return filepath.Abs(p)
	}

	return filepath.Abs(p)
}

func ExistFile(p string) bool {
	realPath, err := ResolveSymlink(p)
	if err != nil {
		return false
	}

	st, err := os.Stat(realPath)
	if err != nil {
		return false
	}

	if !st.IsDir() {
		return true
	}
	return false
}

func ResolveSymlink(p string) (string, error) {
	st, err := os.Lstat(p)
	if err != nil {
		return "", xerrors.Errorf("failed to lstat path %q: %w", p, err)
	}

	if st.Mode()&os.ModeSymlink == os.ModeSymlink {
		// symlink
		new_p, err := filepath.EvalSymlinks(p)
		if err != nil {
			return "", xerrors.Errorf("failed to evaluate symlink path %q: %w", p, err)
		}

		// follow recursively
		new_pp, err := ResolveSymlink(new_p)
		if err != nil {
			return "", xerrors.Errorf("failed to evaluate symlink path %q: %w", new_p, err)
		}

		return new_pp, nil
	}
	return p, nil
}
