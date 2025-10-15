package permission

import (
	"strings"
	"sync"
)

type APIPermission struct {
	Path string
	APIs []string
}

type APIPermissionManager struct {
	apiPermissions     map[string]APIPermission
	apiPermissionsLock sync.RWMutex
}

func NewAPIPermissionManager() *APIPermissionManager {
	return &APIPermissionManager{
		apiPermissions: map[string]APIPermission{},
	}
}

func (manager *APIPermissionManager) Add(irodsPath string, apis ...string) {
	manager.apiPermissionsLock.Lock()
	defer manager.apiPermissionsLock.Unlock()

	if permission, ok := manager.apiPermissions[irodsPath]; ok {
		// exists
		permission.APIs = append(permission.APIs, apis...)
		manager.apiPermissions[irodsPath] = permission
	} else {
		// not exists
		manager.apiPermissions[irodsPath] = APIPermission{
			Path: irodsPath,
			APIs: apis,
		}
	}
}

func (manager *APIPermissionManager) Remove(irodsPath string) {
	manager.apiPermissionsLock.Lock()
	defer manager.apiPermissionsLock.Unlock()

	delete(manager.apiPermissions, irodsPath)
}

func (manager *APIPermissionManager) GetAll() []APIPermission {
	manager.apiPermissionsLock.RLock()
	defer manager.apiPermissionsLock.RUnlock()

	permissions := []APIPermission{}
	for _, apiPermission := range manager.apiPermissions {
		permissions = append(permissions, apiPermission)
	}

	return permissions
}

func (manager *APIPermissionManager) GetForPath(irodsPath string) []string {
	manager.apiPermissionsLock.RLock()
	defer manager.apiPermissionsLock.RUnlock()

	// exact match
	if apiPermission, ok := manager.apiPermissions[irodsPath]; ok {
		// has it
		return apiPermission.APIs
	}

	// wildcard match (*)
	longestMatchPath := ""
	longestMatchAPIs := []string{}
	for _, dirAPIs := range manager.apiPermissions {
		if !strings.HasSuffix(dirAPIs.Path, "/*") {
			continue
		}

		apiPath := strings.TrimSuffix(dirAPIs.Path, "*")
		if strings.HasPrefix(irodsPath, apiPath) {
			if len(apiPath) > len(longestMatchPath) {
				longestMatchPath = apiPath
				longestMatchAPIs = dirAPIs.APIs
			}
		}
	}

	return longestMatchAPIs
}

func (manager *APIPermissionManager) IsAPIAllowed(irodsPath string, apiName string) bool {
	allowedAPIs := manager.GetForPath(irodsPath)
	for _, allowedAPI := range allowedAPIs {
		if allowedAPI == apiName {
			return true
		}
	}
	return false
}
