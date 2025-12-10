package ctxmgr

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shehryarbajwa/browserbase-mini/pkg/models"
)

// Manager handles context persistence
type Manager struct {
	contexts  sync.Map // contextID -> *models.Context
	storePath string   // Base path for storing contexts
	mu        sync.RWMutex
}

// NewManager creates a new context manager
func NewManager(storePath string) (*Manager, error) {
	// Create storage directory if it doesn't exist
	if err := os.MkdirAll(storePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &Manager{
		storePath: storePath,
	}, nil
}

// CreateContext creates a new empty context
func (m *Manager) CreateContext(projectID string) (*models.Context, error) {
	if projectID == "" {
		return nil, fmt.Errorf("projectId is required")
	}

	ctx := &models.Context{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		DataPath:  "", // Will be set when data is saved
	}

	m.contexts.Store(ctx.ID, ctx)

	return ctx, nil
}

// GetContext retrieves a context by ID
func (m *Manager) GetContext(id string) (*models.Context, error) {
	value, ok := m.contexts.Load(id)
	if !ok {
		return nil, fmt.Errorf("context not found")
	}
	return value.(*models.Context), nil
}

// UpdateContext updates the context timestamp
func (m *Manager) UpdateContext(id string) error {
	ctx, err := m.GetContext(id)
	if err != nil {
		return err
	}

	ctx.UpdatedAt = time.Now()
	m.contexts.Store(id, ctx)

	return nil
}

// DeleteContext removes a context and its data
func (m *Manager) DeleteContext(id string) error {
	ctx, err := m.GetContext(id)
	if err != nil {
		return err
	}

	// Delete stored data if exists
	if ctx.DataPath != "" {
		if err := os.Remove(ctx.DataPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete context data: %w", err)
		}
	}

	m.contexts.Delete(id)

	return nil
}

// SaveContextData compresses and saves browser user-data directory
func (m *Manager) SaveContextData(contextID, userDataDir string) error {
	ctx, err := m.GetContext(contextID)
	if err != nil {
		return err
	}

	// Create archive path
	archivePath := filepath.Join(m.storePath, fmt.Sprintf("%s.tar.gz", contextID))

	// Compress the directory
	if err := m.compressDirectory(userDataDir, archivePath); err != nil {
		return fmt.Errorf("failed to compress context data: %w", err)
	}

	// Update context
	ctx.DataPath = archivePath
	ctx.UpdatedAt = time.Now()
	m.contexts.Store(contextID, ctx)

	return nil
}

// LoadContextData extracts context data to a temporary directory
func (m *Manager) LoadContextData(contextID string) (string, error) {
	ctx, err := m.GetContext(contextID)
	if err != nil {
		return "", err
	}

	if ctx.DataPath == "" {
		return "", fmt.Errorf("context has no saved data")
	}

	// Create temporary directory for extraction
	extractPath := filepath.Join(os.TempDir(), fmt.Sprintf("browser-context-%s", contextID))
	if err := os.MkdirAll(extractPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Extract the archive
	if err := m.extractDirectory(ctx.DataPath, extractPath); err != nil {
		return "", fmt.Errorf("failed to extract context data: %w", err)
	}

	return extractPath, nil
}

// compressDirectory creates a tar.gz archive of a directory
func (m *Manager) compressDirectory(source, target string) error {
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		// Update name to be relative to source
		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// If not a directory, write file content
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(tarWriter, file)
			return err
		}

		return nil
	})
}

// extractDirectory extracts a tar.gz archive to a directory
func (m *Manager) extractDirectory(source, target string) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Join(target, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}

			outFile, err := os.Create(targetPath)
			if err != nil {
				return err
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}
