package irods

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/irods-mcp-server/common"
	irods_common "github.com/cyverse/irods-mcp-server/irods/common"
	"github.com/cyverse/irods-mcp-server/irods/model"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/xerrors"
)

const (
	IRODSFileSystemName = "iRODS"
)

type Filesystem struct {
	mcpServer *IRODSMCPServer
	config    *common.Config
}

func NewFilesystem(svr *IRODSMCPServer) ResourceAPI {
	return &Filesystem{
		mcpServer: svr,
		config:    svr.GetConfig(),
	}
}

func (r *Filesystem) GetScheme() string {
	return irods_common.IRODSScheme
}

func (r *Filesystem) GetURI() string {
	return r.GetScheme() + "://"
}

func (r *Filesystem) GetName() string {
	return IRODSFileSystemName
}

func (r *Filesystem) GetDescription() string {
	return `Access to files (data-objects) and directories (collections) on the iRODS`
}

func (r *Filesystem) GetResource() mcp.Resource {
	return mcp.NewResource(
		r.GetURI(),
		r.GetName(),
		mcp.WithResourceDescription(r.GetDescription()),
	)
}

func (r *Filesystem) GetHandler() server.ResourceHandlerFunc {
	return r.Handler
}

func (r *Filesystem) GetAccessiblePaths() []string {
	homePath := irods_common.GetHomePath(r.config)
	sharedPath := irods_common.GetSharedPath(r.config)

	return []string{
		homePath + "/*",
		sharedPath,
		sharedPath + "/*",
	}
}

func (r *Filesystem) Handler(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := request.Params.URI

	parsedURL, err := url.Parse(uri)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse URI %q: %w", uri, err)
	}

	// Check if the URI is valid
	if strings.ToLower(parsedURL.Scheme) != r.GetScheme() {
		return nil, xerrors.Errorf("unsupported URI scheme %q", parsedURL.Scheme)
	}

	// auth
	authValue, err := common.GetAuthValue(ctx)
	if err != nil {
		return nil, xerrors.Errorf("failed to get auth value: %w", err)
	}

	// make a irods filesystem client
	fs, err := r.mcpServer.GetIRODSFSClientFromAuthValue(&authValue)
	if err != nil {
		return nil, xerrors.Errorf("failed to create a irods fs client: %w", err)
	}

	irodsPath := irods_common.MakeIRODSPath(r.config, parsedURL.Path)

	// check permission
	permissionMgr := r.mcpServer.GetPermissionManager()
	if !permissionMgr.IsAPIAllowed(irodsPath, r.GetName()) {
		return nil, xerrors.Errorf("request is not permitted for path %q", irodsPath)
	}

	// Get file info
	sourceEntry, err := fs.Stat(irodsPath)
	if err != nil {
		return nil, err
	}

	if sourceEntry.IsDir() {
		// If it's a directory, list its contents
		listOutput, err := r.listCollection(fs, sourceEntry)
		if err != nil {
			return nil, xerrors.Errorf("failed to list directory (collection) %q: %w", irodsPath, err)
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: "application/json",
				Text:     listOutput,
			},
		}, nil
	}

	// file
	webdavURL := irods_common.MakeWebdavURL(r.config, sourceEntry.Path)
	if sourceEntry.Size > irods_common.MaxInlineSize {
		// file is too large to inline, return a reference to WebDAV URL
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: "text/plain",
				Text:     fmt.Sprintf("File is too large to display inline (%d bytes). Access it via WebDAV URI: %q.", sourceEntry.Size, webdavURL),
			},
		}, nil
	}

	// read the file content
	content, err := r.readDataObject(fs, irodsPath, irods_common.MaxInlineSize)
	if err != nil {
		return nil, xerrors.Errorf("failed to read file (data-object) %q: %w", irodsPath, err)
	}

	mimeType := irods_common.DetectMimeType(irodsPath, content)
	if irods_common.IsTextFile(mimeType) {
		// text file
		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      uri,
				MIMEType: mimeType,
				Text:     string(content),
			},
		}, nil
	} else {
		// binary file
		if sourceEntry.Size <= irods_common.MaxBase64Size {
			// file is small enough to base64 encode
			return []mcp.ResourceContents{
				mcp.BlobResourceContents{
					URI:      uri,
					MIMEType: mimeType,
					Blob:     base64.StdEncoding.EncodeToString(content),
				},
			}, nil
		} else {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      uri,
					MIMEType: "text/plain",
					Text:     fmt.Sprintf("Binary file (%q, %d bytes) is too large to encode to base64 format. Access it via WebDAV URI: %q.", mimeType, sourceEntry.Size, webdavURL),
				},
			}, nil
		}
	}
}

func (r *Filesystem) listCollection(fs *irodsclient_fs.FileSystem, sourceEntry *irodsclient_fs.Entry) (string, error) {
	outputEntries := []model.EntryWithAccess{}

	dirEntries, err := fs.List(sourceEntry.Path)
	if err != nil {
		return "", xerrors.Errorf("failed to list directory (collection) %q: %w", sourceEntry.Path, err)
	}

	for _, dirEntry := range dirEntries {
		objStruct := model.EntryWithAccess{
			Entry:       dirEntry,
			ResourceURI: irods_common.MakeResourceURI(dirEntry.Path),
			WebDAVURI:   irods_common.MakeWebdavURL(r.config, dirEntry.Path),
		}

		outputEntries = append(outputEntries, objStruct)
	}

	listDirectoryOutput := model.ListDirectoryOutput{
		Directory:            sourceEntry,
		DirectoryResourceURI: irods_common.MakeResourceURI(sourceEntry.Path),
		DirectoryWebDAVURI:   irods_common.MakeWebdavURL(r.config, sourceEntry.Path),
		DirectoryEntries:     outputEntries,
	}

	jsonBytes, err := json.Marshal(listDirectoryOutput)
	if err != nil {
		return "", xerrors.Errorf("failed to marshal JSON: %w", err)
	}

	return string(jsonBytes), nil
}

func (r *Filesystem) readDataObject(fs *irodsclient_fs.FileSystem, sourcePath string, maxReadLen int64) ([]byte, error) {
	handle, err := fs.OpenFile(sourcePath, "", "r")
	if err != nil {
		return nil, xerrors.Errorf("failed to open file %q: %w", sourcePath, err)
	}
	defer handle.Close()

	// read the file content
	buffer := make([]byte, maxReadLen)
	n, err := handle.Read(buffer)
	if err != nil {
		if err == io.EOF {
			// EOF is not an error
			return buffer[:n], nil
		}

		return nil, xerrors.Errorf("failed to read file %q: %w", sourcePath, err)
	}

	return buffer[:n], nil
}
