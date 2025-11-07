package model

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
)

type EntryWithAccess struct {
	Entry            *irodsclient_fs.Entry            `json:"entry_info"`
	Accesses         []*irodsclient_types.IRODSAccess `json:"accesses,omitempty"`
	ResourceURI      string                           `json:"resource_uri"`
	WebDAVURI        string                           `json:"webdav_uri"`
	DirectoryEntries []EntryWithAccess                `json:"directory_entries,omitempty"`
}

type ListDirectoryOutput struct {
	Directory            *irodsclient_fs.Entry `json:"directory_info"`
	DirectoryResourceURI string                `json:"directory_resource_uri"`
	DirectoryWebDAVURI   string                `json:"directory_webdav_uri"`
	DirectoryEntries     []EntryWithAccess     `json:"directory_entries"`
}

type SearchFilesOutput struct {
	SearchPath      string            `json:"search_path"`
	MatchingEntries []EntryWithAccess `json:"matching_entries"`
}

type SearchFilesByAVUOutput struct {
	SearchAttribute string            `json:"search_attribute"`
	SearchValue     string            `json:"search_value"`
	MatchingEntries []EntryWithAccess `json:"matching_entries"`
}

type GetFileInfoOutput struct {
	MIMEType          string                                    `json:"mime_type"`
	EntryInfo         *irodsclient_fs.Entry                     `json:"entry_info"`
	ResourceURI       string                                    `json:"resource_uri"`
	WebDAVURI         string                                    `json:"webdav_uri"`
	Accesses          []*irodsclient_types.IRODSAccess          `json:"accesses,omitempty"`
	AccessInheritance *irodsclient_types.IRODSAccessInheritance `json:"access_inheritance,omitempty"`
	Metadata          []*irodsclient_types.IRODSMeta            `json:"metadata,omitempty"`
}

type AllowedAPIs struct {
	Path        string   `json:"path"`
	ResourceURI string   `json:"resource_uri"`
	APIs        []string `json:"apis_allowed,omitempty"`
	Allowed     bool     `json:"allowed"`
}

type ListAllowedDirectories struct {
	Directories []AllowedAPIs `json:"directories"`
}
