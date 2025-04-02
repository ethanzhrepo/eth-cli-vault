package util

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Storage interface defines methods that any storage provider must implement
type Storage interface {
	Put(data []byte, filePath string) (string, error)
	Get(filePath string) ([]byte, error)
	List(dir string) ([]string, error)
}

// StorageFactory creates storage implementations based on provided string
type StorageFactory struct{}

// NewStorage creates a new storage implementation based on the provider
func (f *StorageFactory) NewStorage(provider string) (Storage, error) {
	switch provider {
	case "google":
		return &GoogleDriveStorage{}, nil
	case "dropbox":
		return &DropboxStorage{}, nil
	case "s3":
		return &S3Storage{}, nil
	case "box":
		return &BoxStorage{}, nil
	case "local":
		return &LocalStorage{}, nil
	default:
		// If the provider is not one of the cloud providers, treat it as a local path
		if isLocalPath(provider) {
			return &LocalStorage{}, nil
		}
		return nil, fmt.Errorf("unsupported storage provider: %s", provider)
	}
}

// Put is a convenience method to put data using a specific provider
func Put(provider string, data []byte, filePath string) (string, error) {
	factory := &StorageFactory{}
	storage, err := factory.NewStorage(provider)
	if err != nil {
		return "", err
	}
	return storage.Put(data, filePath)
}

// Get is a convenience method to get data using a specific provider
func Get(provider string, filePath string) ([]byte, error) {
	factory := &StorageFactory{}
	storage, err := factory.NewStorage(provider)
	if err != nil {
		return nil, err
	}
	return storage.Get(filePath)
}

// List is a convenience method to list files using a specific provider
func List(provider string, dir string) ([]string, error) {
	factory := &StorageFactory{}
	storage, err := factory.NewStorage(provider)
	if err != nil {
		return nil, err
	}

	// Get list of wallet files
	files, err := storage.List(dir)
	if err != nil {
		return nil, err
	}

	// Strip file extensions to get wallet names
	var walletNames []string
	for _, file := range files {
		name := filepath.Base(file)
		walletNames = append(walletNames, strings.TrimSuffix(name, filepath.Ext(name)))
	}

	return walletNames, nil
}

// isLocalPath checks if the given path is a local file system path
func isLocalPath(path string) bool {
	// Check if path is a cloud provider
	for _, provider := range CLOUD_PROVIDERS {
		if path == provider {
			return false
		}
	}
	return true
}

// GoogleDriveStorage implements Storage interface for Google Drive
type GoogleDriveStorage struct{}

func (g *GoogleDriveStorage) Put(data []byte, filePath string) (string, error) {
	return UploadToGoogleDrive(data, filePath)
}

func (g *GoogleDriveStorage) Get(filePath string) ([]byte, error) {
	return DownloadFromGoogleDrive(filePath)
}

func (g *GoogleDriveStorage) List(dir string) ([]string, error) {
	return ListGoogleDriveFiles(dir)
}

// DropboxStorage implements Storage interface for Dropbox
type DropboxStorage struct{}

func (d *DropboxStorage) Put(data []byte, filePath string) (string, error) {
	return UploadToDropbox(data, filePath)
}

func (d *DropboxStorage) Get(filePath string) ([]byte, error) {
	return DownloadFromDropbox(filePath)
}

func (d *DropboxStorage) List(dir string) ([]string, error) {
	return ListDropboxFiles(dir)
}

// LocalStorage implements Storage interface for local file system
type LocalStorage struct{}

func (l *LocalStorage) Put(data []byte, filePath string) (string, error) {
	err := SaveToFileSystem(data, filePath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("File saved to local file system: %s", filePath), nil
}

func (l *LocalStorage) Get(filePath string) ([]byte, error) {
	return LoadFromFileSystem(filePath)
}

func (l *LocalStorage) List(dir string) ([]string, error) {
	return ListFilesFromFileSystem(dir)
}

// S3Storage implements Storage interface for AWS S3
type S3Storage struct{}

func (s *S3Storage) Put(data []byte, filePath string) (string, error) {
	return UploadToS3(data, filePath)
}

func (s *S3Storage) Get(filePath string) ([]byte, error) {
	return DownloadFromS3(filePath)
}

func (s *S3Storage) List(dir string) ([]string, error) {
	return ListS3Files(dir)
}

// BoxStorage implements Storage interface for Box
type BoxStorage struct{}

func (b *BoxStorage) Put(data []byte, filePath string) (string, error) {
	return UploadToBox(data, filePath)
}

func (b *BoxStorage) Get(filePath string) ([]byte, error) {
	return DownloadFromBox(filePath)
}

func (b *BoxStorage) List(dir string) ([]string, error) {
	return ListBoxFiles(dir)
}
