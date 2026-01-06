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
	AVUs              []*irodsclient_types.IRODSMeta            `json:"avus,omitempty"`
}

type WriteFileOutput struct {
	Path         string `json:"path"`
	Offset       int64  `json:"offset"`
	BytesWritten int    `json:"bytes_written"`
}

type MoveFileOutput struct {
	OldPath      string                `json:"old_path"`
	OldEntryInfo *irodsclient_fs.Entry `json:"old_entry_info"`
	NewPath      string                `json:"new_path"`
	NewEntryInfo *irodsclient_fs.Entry `json:"new_entry_info"`
}

type CopyFileOutput struct {
	SourcePath          string                  `json:"source_path"`
	DestinationPath     string                  `json:"destination_path"`
	SourceEntryInfoList []*irodsclient_fs.Entry `json:"source_entry_info_list"`
	CopiedEntryInfoList []*irodsclient_fs.Entry `json:"copied_entry_info_list"`
}

type MakeDirectoryOutput struct {
	Path      string                `json:"path"`
	EntryInfo *irodsclient_fs.Entry `json:"entry_info"`
}

type RemoveFileOutput struct {
	Path      string                `json:"path"`
	EntryInfo *irodsclient_fs.Entry `json:"entry_info"`
}

type TicketWithRestrictions struct {
	Ticket       *irodsclient_types.IRODSTicket          `json:"ticket"`
	Restrictions *irodsclient_fs.IRODSTicketRestrictions `json:"restrictions,omitempty"`
}

type AVU struct {
	ID        int64  `json:"id,omitempty"`
	Attribute string `json:"attribute"`
	Value     string `json:"value"`
	Unit      string `json:"unit,omitempty"`
}

type ListAVUsOutput struct {
	TargetType string `json:"target_type"`
	Target     string `json:"target"`
	AVUs       []AVU  `json:"avus"`
}

type AddAVUOutput struct {
	TargetType string `json:"target_type"`
	Target     string `json:"target"`
	Attribute  string `json:"attribute"`
}

type DeleteAVUOutput struct {
	TargetType string `json:"target_type"`
	Target     string `json:"target"`
	ID         int64  `json:"id,omitempty"`
	Attribute  string `json:"attribute,omitempty"`
	Value      string `json:"value,omitempty"`
	Unit       string `json:"unit,omitempty"`
}

type ModifyAccessOutput struct {
	Path        string                                 `json:"path"`
	UserName    string                                 `json:"user_name"`
	UserZone    string                                 `json:"user_zone"`
	AccessLevel irodsclient_types.IRODSAccessLevelType `json:"access_level"`
}

type ModifyAccessInheritanceOutput struct {
	Path    string `json:"path"`
	Inherit bool   `json:"inherit"`
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
