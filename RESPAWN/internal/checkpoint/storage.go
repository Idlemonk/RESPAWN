package checkpoint

import (
    "crypto/sha256"
    "encoding/binary"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
    "time"
    "github.com/klauspost/compress/zstd"
	"RESPAWN/internal/checkpoint"
	"RESPAWN/internal/system"

)

type Storage struct {
	baseDir    string
	compressor     *zstd.Encoder
	decompressor    *zstd.Decoder
	compressionLevel    int 
}

type CheckpointMetadata struct {
	ID           string    `json:"id"`
    Timestamp    time.Time `json:"timestamp"`
    IsCompressed bool      `json:"is_compressed"`
    OriginalSize int64     `json:"original_size"`
    CompressedSize int64   `json:"compressed_size,omitempty"`
    Checksum     string    `json:"checksum"`
    AppCount     int       `json:"app_count"`
    AppNames     []string  `json:"app_names"`

}

// NewStorage creates a new storage manager
func NewStorage(baseDir string) (*Storage, error) {
	// Create zstd compressor with default level (user can change later)
	compressor, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, fmt.Errorf("failed to create compressor: %w", err)
	}

	decompressor, err := zstd.NewReader(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create decompressor: %w", err)
	}

	storage := &Storage{
        baseDir:          baseDir,
        compressor:       compressor,
        decompressor:     decompressor,
        compressionLevel: int(zstd.SpeedDefault),
	}

    // Create metadata directory
    metadataDir := filepath.Join(baseDir, "metadata")
    if err := os.MkdirAll(metadataDir, 0755); err != nil {
        return nil, fmt.Errorf("Failed to create metadata directory: %w", err)
    }

    return storage, nil 
}

// SetCompressionLevel allows user to manually set compression level
func (s *Storage) SetCompressionLevel(level int) error {
    // zstd levels: 1 (fastest) to 22 (best compression)
    if level < 1 || level > 22 {
        return fmt.Errorf("Invalid compression level %d, must be 1-22", level)
    }

    s.compressor.Close()
    s.compressor = compressor 
    s.compressionLevel = level

    system.Info("Compression level set to, level")
    return nil 
}

// This below is the function that saves a checkpoint to binary format.
func (s *Storage) SaveCheckpoint(checkpoint *Checkpoint) (string, int64, error) {
    system.Debug("Saving checkpoint", checkpoint.ID)

    // This is how the binary file is created 
    fileName := fmt.Sprint("%s.bin", checkpoint.ID)
    filePath := filepath.Join(s.baseDir, fileName)

    // Converts checkpoint to binary data
    data, err := s.serializeCheckpoint(checkpoint)
    if err != nil {
        return "", 0, fmt.Errorf("Failed to serialize checkpoint: %w", err)
    }

    // Write binary data to file
    file, err := os.Create(filePath)
    if err != nil {
        return "", 0, fmt.Errorf("Failed to create checkpoint file: %w", err)
    }
    defer file.Close()

    bytesWritten, err := file.Write(data)
    if err != nil {
        return "", 0, fmt.Errorf("Failed to write checkpoint data: %w", err)
    }

    // Calculate checksum for integrity
    checksum := s.calculateChecksum(data)

    // Saves metadata
    metadata := &CheckpointMetadata{
        ID:           checkpoint.ID,
        Timestamp:    checkpoint.Timestamp,
        IsCompressed: false,
        OriginalSize: int64(bytesWritten),
        Checksum:     checksum,
        AppCount:     len(checkpoint.Processes),
        AppNames:     checkpoint.AppNames,
    }

    if err := s.saveMetadata(metadata); err != nil {
        system.Warn("Failed to save metadata for", checkpoint.ID, ":", err)
    }

    system.Debug("Saved checkpoint", checkpoint.ID, "Size:", bytesWritten, "bytes")
    return filePath, int64(bytesWritten), nil 
}

// LoadCheckpoint loads a checkpoint from storage with streaming
func (s *Storage) LoadCheckpoint(checkpointID string) (*Checkpoint, error) {
    system.Debug("Loading checkpoint", checkpointID)

// Try compressed version first, then uncompressed
    filePath := s.getCheckpointPath(checkpointID)
    isCompressed := strings.HasSuffix(filePath, "_compressed.bin")

    // This makes sure the file is validated before loading
    if err := s.validateCheckpointFile(checkpointID); err != nil {
        return nil, fmt.Errorf("checkpoint validation failed: %w", err) 
    }

    // Stream data from file
    file, err := os.Open(filePath)
    if err != nil {
        return nil, fmt.Errorf("Failed to open checkpoint file: %w", err)
    }
    defer file.Close()

    var reader io.Reader = file 

    // Decompress if needed
    if isCompressed {
        decompressedData, err := s.decompressor.DecodeAll(nil, nil)
        if err != nil {
            return nil, fmt.Errorf("Failed to setup decpmpression: %w", err)
        }

        // Read compressed data
        compressedData, err := io.ReadAll(file)
        if err != nil {
            return nil, fmt.Errorf("Failed to read compressed data: %w", err)
        }

        // Decompress data
        decompressedData, err = s.decompressor.DecodeAll(decompressedData, compressedData)
        if err != nil {
            return nil, fmt.Errorf("Failed to decompress checkpoint: %w", err)
        }

        reader = strings.NewReader(string(decompressedData))
    }

    // Deserialize checkpoint data
    checkpoint, err := s.deserializeCheckpoint(reader)
    if err != nil {
        return nil, fmt.Errorf("Failed to deserialize checkpoint: %w", err)
    }

    checkpoint.FilePath = filePath
    checkpoint.IsCompressed = isCompressed

    system.Debug("Loaded checkpoint", checkpointID, "Apps:", len(checkpoint.Processes))
    return checkpoint, nil 
}

// LoadAllCheckpoints loads all available checkpoints with metadata
func (s *Storage) LoadAllCheckpoints() ([]Checkpoint, error) {
    system.Debug("Loading all available checkpoints")

    files, err := os.ReadDir(s.baseDir)
    if err != nil {
        return nil, fmt.Errorf("Failed to read checkpoint directory: %w", err)
    }

    var checkpoints []Checkpoint

    for _, file := range files {
        if file.IsDir() || (!strings.HasSuffix(file.Name(), ".bin")) {
            continue 
        }

        //Extract checkpoint ID from filename
        fileName := file.Name()
        checkpointID := strings.TrimSuffix(fileName, ".bin")
        checkpointID = strings.TrimSuffix(checkpointID, "_compressed")

        
        // Load metadata first (faster than full checkpoint)
        metadata, err := s.loadMetadata(checkpointID)
        if err != nil {
            system.Warn("Failed to load metadata for", checkpointID, "- loading full checkpoint")
            // Fallback to loading full checkpoint
            checkpoint, err := s.LoadCheckpoint(checkpointID)
            if err != nil {
                system.Warn("Failed to load checkpoint", checkpointID, ":", err)
                continue 
            }
            checkpoints = append(checkpoints, *checkpoint)
            continue
        }

        // Create checkpoint summary from metadata
        checkpoint := Checkpoint{
            ID:           metadata.ID,
            Timestamp:    metadata.Timestamp,
            AppNames:     metadata.AppNames,
            IsCompressed: metadata.IsCompressed,
            FilePath:     s.getCheckpointPath(checkpointID),
            FileSize:     metadata.OriginalSize,
        }

        if metadata.IsCompressed {
            checkpoint.FileSize = metadata.CompressedSize
        }

        checkpoints = append(checkpoints, checkpoint)
    }

    system.Debug("Loaded", len(checkpoints), "checkpoint summaries")
    return checkpoints, nil 
}

// CompressCheckpoint compress an existing checkpoint
func (s *Storage) CompressCheckpoint(checkpoint *Checkpoint) error {
    if checkpoint.IsCompressed {
        return nil // Already Compressed
    }

    system.Debug("Compressing checkpoint", checkpoint.ID)

    originalPath := s.getCheckpointPath(checkpoint.ID)
    compressedPath := filepath.Join(s.baseDir, fmt.Sprintf("%s_compressed.bin", checkpoint.ID))

    // Read original file
    originalData, err := os.ReadFile(originalPath)
    if err != nil {
        return fmt.Errorf("Failed to read original checkpoint: %w", err)
    }

    // This function compresses data
    compressedData := s.compressor.EncodeAll(originalData, nil)


    // This function writes compressed file
    if err := os.WriteFile(compressedPath, compressedData, 0644); err != nil {
        return fmt.Errorf("Failed to write compressed checkpoint: %w", err)
    }

    // Update Metadata
    metadata, _ := s.loadMetadata(checkpoint.ID)
    if metadata != nil {
        metadata.IsCompressed = true
        metadata.CompressedSize = int64(len(compressedData))
        metadata.Checksum = s.calculateChecksum(compressedData)
        s.saveMetadata(metadata)
    }

    //Remove original file
    if err := os.Remove(originalPath); err != nil {
        system.Warn("Failed to remove original file", originalPath, ":", err)
    }

    compressionRatio := float64(len(compressedData)) / float64(len(originalData)) * 100
    system.Info("Compressed", checkpoint.ID, "Size:", len(originalData), "→", len(compressedData), 
                fmt.Sprintf("(%.1f%%)", compressionRatio))

    checkpoint.IsCompressed = true
    checkpoint.FilePath = compressedPath
    checkpoint.FileSize = int64(len(compressedData))

    return nil
}

//This function validates checkpoint integrity using checksums
func (s *Storage) validateCheckpointFile(checkpointID string) error {
    filePath := s.getCheckpointPath(checkpointID)

    // Check if file exista and it's readable
    fileInfo, err := os.Stat(filePath)
    if err != nil {
        return fmt.Errorf("checkpoint file not accessible: %w", err)
    }

    //Basic size check
    if fileInfo.Size() == 0 {
        return fmt.Errorf("checkpoint file is empty")
    }

    // This loads metadata for checksum validation
    metadata, err := s.loadMetadata(checkpointID)
    if err != nil {
        system.Debug("No metadata found for", checkpointID, "-skipping checksum validation")
        return nil 
    }

    //Read file and calculate checksum
    data, err := os.ReadFile(filePath)
    if err != nil {
        return fmt.Errorf("Failed to read checkpoint file: %w", err)
    }

    actualChecksum := s.calculateChecksum(data)
    if actualChecksum != metadata.Checksum {
        return fmt.Errorf("Checksum mismatch - file may be corrupted (expected: %s, got: %s)", metadata.Checksum, actualChecksum)
    } 

    system.Debug("Checkpoint", checkpointID, "validation passed")
    return nil 
}

// CleanOldCheckpoints removes checkpoints older than the cuttoff time 
func (s *Storage) CleanOldCheckpoints(cutoffTime time.Time) error {
    system.Debug("Cleaning checkpoints older than", cutoffTime.Format("2006-01-02 15:04:05"))

    files, err := os.ReadDir(s.baseDir)
    if err != nil {
        return fmt.Errorf("Failed to read checkpoint directory: %w", err)
    }

    deletedCount := 0

    for _, file := range files {
        if file.IsDir() || !strings.HasSuffix(file.Name(), ".bin") {
            continue
        }

        filePath := filepath.Join(s.baseDir, file.Name())
        fileInfo, err := file.Info()
        if err != nil {
            continue
        }

        if fileInfo.ModTime().Before(cutoffTime) {
            if err := os.Remove(filePath); err != nil {
                system.Warn("Failed to delete old checkpoint", file.Name(), ";", err)
                continue
            }

            //Also remove metadata 
            checkpointID := strings.TrimSuffix(file.Name(), ".bin")
            checkpointID = strings.TrimSuffix(checkpointID, "_compressed")
            s.deleteMetadata(checkpointID)

            deletedCount++
            system.Debug("Deleted old checkpoint:", file.Name())
        }
    }

    if deletedCount > 0 {
        system.Info("Cleaned", deletedCount, "old checkpoints")
    }

    return nil 
}

// Helper functions

// serializeCheckpoints converts checkpoint to binary format
func (s *Storage) serializeCheckpoint(checkpoint *Checkpoint) ([]byte, error) {
    //  For now, use JSON serialization as binary format
    // In a more optimized version, You could use protocol buffers or custom binary format
    data, err := json.Marshal(checkpoint)
    if err != nil {
        return nil, err 
    }
    return data, nil 
}

// deserializeCheckpoint converts binary data back to checkpoint
func (s *Storage) deserializeCheckpoint(reader io.Reader) (*Checkpoint, error) {
    data, err := io.ReadAll(reader)
    if err != nil {
        return nil, err 
    }

    var checkpoint Checkpoint
    if err := json.Unmarshal(data, &checkpoint); err != nil {
        return nil, err
    }

    return &checkpoint, nil
}

// getCheckpointPath returns the file path for a checkpoint
func (s *Storage) getCheckpointPath(checkpointID string) string {
    // Check for compressed version first
    compressedPath := filepath.Join(s.baseDir, fmt.Sprintf("%s_compressed.bin", checkpointID))
    if _, err := os.Stat(compressedPath); err == nil {
        return compressedPath
    }

    //Return uncompressed path
    return filepath.Join(s.baseDir, fmt.Sprintf("%s.bin", checkpointID))
}

//This functions calculates SHA256 checksum for integrity validation ; [calculateChecksum]
func (s *Storage) calculateChecksum(data []byte) string {
    hash := sha256.Sum256(data)
    return fmt.Sprintf("%x", hash)
}

//This method saves checkpoint metadata
func (s *Storage) saveMetadata(metadata *CheckpointMetadata) error {
    metadataPath := filepath.Join(s.baseDir, "metadata", fmt.Sprintf("%s.json", metadata.ID))
    data, err := json.MarshalIndent(metadata, "", " ")
    if err != nil {
        return err
    }
    return os.WriteFile(metadataPath, data, 0644)
}

//This method loads checkpoint metadata
func (s *Storage) loadMetadata(checkpointID string) (*CheckpointMetadata, error) {
    metadataPath := filepath.Join(s.baseDir, "metadata", fmt.Sprintf("%s.json", checkpointID))
    data, err := os.ReadFile(metadataPath)
    if err != nil {
        return nil, err 
    }

    var metadata CheckpointMetadata
    if err := json.Unmarshal(data, &metadata); err != nil {
        return nil, err 
    }

    return &metadata, nil 
}

// deleteMetadata removes metadata for a checkpoint
func (s *Storage) deleteMetadata(checkpointID string) {
    metadataPath := filepath.Join(s.baseDir, "metadata", fmt.Sprintf("%s.json", checkpointID))
    os.Remove(metadataPath) // Ignore ERRORS
}

// This method cleans up storage resources
func (s *Storage) Close() {
    if s.compressor != nil {
        s.compressor.Close()
    }
    if s.decompressor != nil {
        s.decompressor.Close()
    }
}


























































