package smbfs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"
)

// MockSMBBackend provides an in-memory SMB server simulation for testing.
// It maintains a virtual filesystem that can be populated with test data
// and tracks all operations for verification.
type MockSMBBackend struct {
	mu sync.RWMutex

	// files maps paths to mock file data
	files map[string]*mockFileData

	// shares available on this mock server
	shares map[string]bool

	// errors to inject for specific operations
	errorOnPath map[string]error
	errorOnOp   map[string]error

	// operation tracking for verification (separate mutex to avoid lock contention)
	opMu       sync.Mutex
	operations []MockOperation
}

// mockFileData represents a file or directory in the mock filesystem.
type mockFileData struct {
	name    string
	content []byte
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

// MockOperation records an operation performed on the mock backend.
type MockOperation struct {
	Op   string
	Path string
	Args []interface{}
	Time time.Time
}

// NewMockSMBBackend creates a new mock SMB backend with default shares.
func NewMockSMBBackend() *MockSMBBackend {
	m := &MockSMBBackend{
		files:       make(map[string]*mockFileData),
		shares:      make(map[string]bool),
		errorOnPath: make(map[string]error),
		errorOnOp:   make(map[string]error),
		operations:  make([]MockOperation, 0),
	}

	// Create root directory
	m.files["/"] = &mockFileData{
		name:    "/",
		isDir:   true,
		mode:    fs.ModeDir | 0755,
		modTime: time.Now(),
	}

	// Add default share
	m.shares["testshare"] = true

	return m
}

// AddShare adds a share to the mock backend.
// Note: This method acquires the lock internally.
func (m *MockSMBBackend) AddShare(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addShareLocked(name)
}

// addShareLocked adds a share without acquiring lock (caller must hold lock).
func (m *MockSMBBackend) addShareLocked(name string) {
	m.shares[name] = true
}

// AddFile adds a file to the mock filesystem.
func (m *MockSMBBackend) AddFile(path string, content []byte, mode fs.FileMode) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path = normalizeMockPath(path)
	m.files[path] = &mockFileData{
		name:    pathBase(path),
		content: content,
		mode:    mode,
		modTime: time.Now(),
		isDir:   false,
	}

	// Ensure parent directories exist
	m.ensureParentDirs(path)
}

// AddDir adds a directory to the mock filesystem.
func (m *MockSMBBackend) AddDir(path string, mode fs.FileMode) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path = normalizeMockPath(path)
	m.files[path] = &mockFileData{
		name:    pathBase(path),
		isDir:   true,
		mode:    fs.ModeDir | mode,
		modTime: time.Now(),
	}

	// Ensure parent directories exist
	m.ensureParentDirs(path)
}

// SetError sets an error to return for a specific path.
func (m *MockSMBBackend) SetError(path string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorOnPath[normalizeMockPath(path)] = err
}

// SetOperationError sets an error to return for a specific operation type.
func (m *MockSMBBackend) SetOperationError(op string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorOnOp[op] = err
}

// ClearErrors clears all injected errors.
func (m *MockSMBBackend) ClearErrors() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorOnPath = make(map[string]error)
	m.errorOnOp = make(map[string]error)
}

// GetOperations returns all recorded operations.
func (m *MockSMBBackend) GetOperations() []MockOperation {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	ops := make([]MockOperation, len(m.operations))
	copy(ops, m.operations)
	return ops
}

// ClearOperations clears the operation history.
func (m *MockSMBBackend) ClearOperations() {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.operations = make([]MockOperation, 0)
}

// GetFile returns the content of a file (for test verification).
func (m *MockSMBBackend) GetFile(path string) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	path = normalizeMockPath(path)
	if f, ok := m.files[path]; ok && !f.isDir {
		content := make([]byte, len(f.content))
		copy(content, f.content)
		return content, true
	}
	return nil, false
}

// FileExists returns true if the file exists.
func (m *MockSMBBackend) FileExists(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.files[normalizeMockPath(path)]
	return ok
}

// recordOp records an operation for later verification.
// This is thread-safe and uses its own mutex to avoid lock contention.
func (m *MockSMBBackend) recordOp(op, path string, args ...interface{}) {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.operations = append(m.operations, MockOperation{
		Op:   op,
		Path: path,
		Args: args,
		Time: time.Now(),
	})
}

// checkError checks for injected errors.
func (m *MockSMBBackend) checkError(op, path string) error {
	if err, ok := m.errorOnOp[op]; ok {
		return err
	}
	if err, ok := m.errorOnPath[path]; ok {
		return err
	}
	return nil
}

// ensureParentDirs ensures all parent directories exist.
func (m *MockSMBBackend) ensureParentDirs(p string) {
	dir := pathDir(p)
	if dir == p || dir == "/" {
		return
	}

	if _, ok := m.files[dir]; !ok {
		m.files[dir] = &mockFileData{
			name:    pathBase(dir),
			isDir:   true,
			mode:    fs.ModeDir | 0755,
			modTime: time.Now(),
		}
		m.ensureParentDirs(dir)
	}
}

// normalizeMockPath normalizes a path for the mock filesystem.
func normalizeMockPath(p string) string {
	// Convert backslashes to forward slashes
	p = strings.ReplaceAll(p, "\\", "/")

	// Ensure leading slash
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	// Clean the path
	p = path.Clean(p)

	return p
}

// pathBase returns the last element of path.
func pathBase(p string) string {
	return path.Base(p)
}

// pathDir returns all but the last element of path.
func pathDir(p string) string {
	return path.Dir(p)
}

// MockSMBSession implements SMBSession for testing.
type MockSMBSession struct {
	backend *MockSMBBackend
	loggedOff bool
	mu        sync.Mutex
}

// NewMockSMBSession creates a new mock SMB session.
func NewMockSMBSession(backend *MockSMBBackend) *MockSMBSession {
	return &MockSMBSession{
		backend: backend,
	}
}

// Mount mounts a share and returns an SMBShare interface.
func (s *MockSMBSession) Mount(shareName string) (SMBShare, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loggedOff {
		return nil, errors.New("session logged off")
	}

	s.backend.mu.RLock()
	defer s.backend.mu.RUnlock()

	if err := s.backend.checkError("mount", shareName); err != nil {
		return nil, err
	}

	if !s.backend.shares[shareName] {
		return nil, errors.New("share not found: " + shareName)
	}

	s.backend.recordOp("mount", shareName)
	return &MockSMBShare{backend: s.backend, shareName: shareName}, nil
}

// Logoff ends the session.
func (s *MockSMBSession) Logoff() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loggedOff {
		return nil
	}

	s.loggedOff = true
	s.backend.mu.Lock()
	defer s.backend.mu.Unlock()
	s.backend.recordOp("logoff", "")
	return nil
}

// MockSMBShare implements SMBShare for testing.
type MockSMBShare struct {
	backend   *MockSMBBackend
	shareName string
	unmounted bool
	mu        sync.Mutex
}

// OpenFile opens a file with the specified flags and permissions.
func (sh *MockSMBShare) OpenFile(name string, flag int, perm fs.FileMode) (SMBFile, error) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	if sh.unmounted {
		return nil, errors.New("share unmounted")
	}

	sh.backend.mu.Lock()
	defer sh.backend.mu.Unlock()

	name = normalizeMockPath(name)

	if err := sh.backend.checkError("open", name); err != nil {
		return nil, err
	}

	sh.backend.recordOp("open", name, flag, perm)

	// Check if file exists
	data, exists := sh.backend.files[name]

	// Handle creation flags
	create := flag&os.O_CREATE != 0
	excl := flag&os.O_EXCL != 0
	trunc := flag&os.O_TRUNC != 0

	if excl && exists {
		return nil, fs.ErrExist
	}

	if !exists {
		if !create {
			return nil, fs.ErrNotExist
		}

		// Create new file
		data = &mockFileData{
			name:    pathBase(name),
			content: []byte{},
			mode:    perm,
			modTime: time.Now(),
			isDir:   false,
		}
		sh.backend.files[name] = data
		sh.backend.ensureParentDirs(name)
	}

	// Can't open a directory for writing
	if data.isDir && (flag&(os.O_WRONLY|os.O_RDWR) != 0) {
		return nil, errors.New("is a directory")
	}

	if trunc && !data.isDir {
		data.content = []byte{}
		data.modTime = time.Now()
	}

	return &MockSMBFile{
		backend: sh.backend,
		path:    name,
		data:    data,
		flag:    flag,
	}, nil
}

// Stat returns file info for the specified path.
func (sh *MockSMBShare) Stat(name string) (fs.FileInfo, error) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	if sh.unmounted {
		return nil, errors.New("share unmounted")
	}

	sh.backend.mu.RLock()
	defer sh.backend.mu.RUnlock()

	name = normalizeMockPath(name)

	if err := sh.backend.checkError("stat", name); err != nil {
		return nil, err
	}

	sh.backend.recordOp("stat", name)

	data, exists := sh.backend.files[name]
	if !exists {
		return nil, fs.ErrNotExist
	}

	return &mockFileInfo{data: data}, nil
}

// Mkdir creates a directory.
func (sh *MockSMBShare) Mkdir(name string, perm fs.FileMode) error {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	if sh.unmounted {
		return errors.New("share unmounted")
	}

	sh.backend.mu.Lock()
	defer sh.backend.mu.Unlock()

	name = normalizeMockPath(name)

	if err := sh.backend.checkError("mkdir", name); err != nil {
		return err
	}

	sh.backend.recordOp("mkdir", name, perm)

	// Check if already exists
	if _, exists := sh.backend.files[name]; exists {
		return fs.ErrExist
	}

	// Check if parent exists
	parent := pathDir(name)
	parentData, parentExists := sh.backend.files[parent]
	if !parentExists {
		return fs.ErrNotExist
	}
	if !parentData.isDir {
		return errors.New("parent is not a directory")
	}

	// Create directory
	sh.backend.files[name] = &mockFileData{
		name:    pathBase(name),
		isDir:   true,
		mode:    fs.ModeDir | perm,
		modTime: time.Now(),
	}

	return nil
}

// Remove removes a file or empty directory.
func (sh *MockSMBShare) Remove(name string) error {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	if sh.unmounted {
		return errors.New("share unmounted")
	}

	sh.backend.mu.Lock()
	defer sh.backend.mu.Unlock()

	name = normalizeMockPath(name)

	if err := sh.backend.checkError("remove", name); err != nil {
		return err
	}

	sh.backend.recordOp("remove", name)

	data, exists := sh.backend.files[name]
	if !exists {
		return fs.ErrNotExist
	}

	// If directory, check if empty
	if data.isDir {
		for p := range sh.backend.files {
			if p != name && strings.HasPrefix(p, name+"/") {
				return errors.New("directory not empty")
			}
		}
	}

	delete(sh.backend.files, name)
	return nil
}

// Rename renames a file or directory.
func (sh *MockSMBShare) Rename(oldname, newname string) error {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	if sh.unmounted {
		return errors.New("share unmounted")
	}

	sh.backend.mu.Lock()
	defer sh.backend.mu.Unlock()

	oldname = normalizeMockPath(oldname)
	newname = normalizeMockPath(newname)

	if err := sh.backend.checkError("rename", oldname); err != nil {
		return err
	}

	sh.backend.recordOp("rename", oldname, newname)

	data, exists := sh.backend.files[oldname]
	if !exists {
		return fs.ErrNotExist
	}

	// Check if new name already exists
	if _, exists := sh.backend.files[newname]; exists {
		return fs.ErrExist
	}

	// Rename (move the data to new path)
	delete(sh.backend.files, oldname)
	data.name = pathBase(newname)
	sh.backend.files[newname] = data

	// If directory, also rename all children
	if data.isDir {
		for p, d := range sh.backend.files {
			if strings.HasPrefix(p, oldname+"/") {
				newPath := newname + strings.TrimPrefix(p, oldname)
				delete(sh.backend.files, p)
				d.name = pathBase(newPath)
				sh.backend.files[newPath] = d
			}
		}
	}

	return nil
}

// Chmod changes the mode of a file.
func (sh *MockSMBShare) Chmod(name string, mode fs.FileMode) error {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	if sh.unmounted {
		return errors.New("share unmounted")
	}

	sh.backend.mu.Lock()
	defer sh.backend.mu.Unlock()

	name = normalizeMockPath(name)

	if err := sh.backend.checkError("chmod", name); err != nil {
		return err
	}

	sh.backend.recordOp("chmod", name, mode)

	data, exists := sh.backend.files[name]
	if !exists {
		return fs.ErrNotExist
	}

	// Preserve directory bit
	if data.isDir {
		data.mode = fs.ModeDir | mode
	} else {
		data.mode = mode
	}

	return nil
}

// Chtimes changes the access and modification times of a file.
func (sh *MockSMBShare) Chtimes(name string, atime, mtime time.Time) error {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	if sh.unmounted {
		return errors.New("share unmounted")
	}

	sh.backend.mu.Lock()
	defer sh.backend.mu.Unlock()

	name = normalizeMockPath(name)

	if err := sh.backend.checkError("chtimes", name); err != nil {
		return err
	}

	sh.backend.recordOp("chtimes", name, atime, mtime)

	data, exists := sh.backend.files[name]
	if !exists {
		return fs.ErrNotExist
	}

	data.modTime = mtime
	return nil
}

// Umount unmounts the share.
func (sh *MockSMBShare) Umount() error {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	if sh.unmounted {
		return nil
	}

	sh.unmounted = true
	sh.backend.mu.Lock()
	defer sh.backend.mu.Unlock()
	sh.backend.recordOp("umount", sh.shareName)
	return nil
}

// MockSMBFile implements SMBFile for testing.
type MockSMBFile struct {
	backend *MockSMBBackend
	path    string
	data    *mockFileData
	flag    int
	offset  int64
	closed  bool
	mu      sync.Mutex
}

// Read reads up to len(p) bytes into p.
func (f *MockSMBFile) Read(p []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return 0, fs.ErrClosed
	}

	f.backend.mu.RLock()
	defer f.backend.mu.RUnlock()

	if err := f.backend.checkError("read", f.path); err != nil {
		return 0, err
	}

	if f.data.isDir {
		return 0, errors.New("is a directory")
	}

	if f.offset >= int64(len(f.data.content)) {
		return 0, io.EOF
	}

	n = copy(p, f.data.content[f.offset:])
	f.offset += int64(n)

	if f.offset >= int64(len(f.data.content)) {
		return n, io.EOF
	}

	return n, nil
}

// Write writes len(p) bytes from p to the file.
func (f *MockSMBFile) Write(p []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return 0, fs.ErrClosed
	}

	// Check if opened for writing
	if f.flag&(os.O_WRONLY|os.O_RDWR) == 0 {
		return 0, errors.New("file not opened for writing")
	}

	f.backend.mu.Lock()
	defer f.backend.mu.Unlock()

	if err := f.backend.checkError("write", f.path); err != nil {
		return 0, err
	}

	if f.data.isDir {
		return 0, errors.New("is a directory")
	}

	// Special case: zero-length write at an offset truncates the file
	if len(p) == 0 {
		if f.offset < int64(len(f.data.content)) {
			// Truncate to current offset
			f.data.content = f.data.content[:f.offset]
			f.data.modTime = time.Now()
		}
		return 0, nil
	}

	// Expand content if needed
	endOffset := f.offset + int64(len(p))
	if endOffset > int64(len(f.data.content)) {
		newContent := make([]byte, endOffset)
		copy(newContent, f.data.content)
		f.data.content = newContent
	}

	n = copy(f.data.content[f.offset:], p)
	f.offset += int64(n)
	f.data.modTime = time.Now()

	return n, nil
}

// Seek sets the offset for the next Read or Write.
func (f *MockSMBFile) Seek(offset int64, whence int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return 0, fs.ErrClosed
	}

	f.backend.mu.RLock()
	size := int64(len(f.data.content))
	f.backend.mu.RUnlock()

	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = f.offset + offset
	case io.SeekEnd:
		newOffset = size + offset
	default:
		return 0, errors.New("invalid whence")
	}

	if newOffset < 0 {
		return 0, errors.New("negative offset")
	}

	f.offset = newOffset
	return newOffset, nil
}

// Close closes the file.
func (f *MockSMBFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return nil
	}

	f.closed = true
	f.backend.mu.Lock()
	defer f.backend.mu.Unlock()
	f.backend.recordOp("close", f.path)
	return nil
}

// Stat returns file information.
func (f *MockSMBFile) Stat() (fs.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return nil, fs.ErrClosed
	}

	f.backend.mu.RLock()
	defer f.backend.mu.RUnlock()

	if err := f.backend.checkError("stat", f.path); err != nil {
		return nil, err
	}

	return &mockFileInfo{data: f.data}, nil
}

// Readdir reads the directory contents.
func (f *MockSMBFile) Readdir(n int) ([]fs.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return nil, fs.ErrClosed
	}

	f.backend.mu.RLock()
	defer f.backend.mu.RUnlock()

	if !f.data.isDir {
		return nil, errors.New("not a directory")
	}

	if err := f.backend.checkError("readdir", f.path); err != nil {
		return nil, err
	}

	// Find all direct children
	var infos []fs.FileInfo
	prefix := f.path
	if prefix != "/" {
		prefix += "/"
	}

	for p, data := range f.backend.files {
		if p == f.path {
			continue
		}

		if !strings.HasPrefix(p, prefix) {
			continue
		}

		// Check if it's a direct child (no additional slashes)
		remainder := strings.TrimPrefix(p, prefix)
		if strings.Contains(remainder, "/") {
			continue
		}

		infos = append(infos, &mockFileInfo{data: data})
	}

	// Sort by name for consistent results
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name() < infos[j].Name()
	})

	if n <= 0 {
		return infos, nil
	}

	if n > len(infos) {
		n = len(infos)
	}

	return infos[:n], nil
}

// mockFileInfo implements fs.FileInfo for mock files.
type mockFileInfo struct {
	data *mockFileData
}

func (fi *mockFileInfo) Name() string       { return fi.data.name }
func (fi *mockFileInfo) Size() int64        { return int64(len(fi.data.content)) }
func (fi *mockFileInfo) Mode() fs.FileMode  { return fi.data.mode }
func (fi *mockFileInfo) ModTime() time.Time { return fi.data.modTime }
func (fi *mockFileInfo) IsDir() bool        { return fi.data.isDir }
func (fi *mockFileInfo) Sys() interface{}   { return nil }

// MockConnectionFactory implements ConnectionFactory for testing.
type MockConnectionFactory struct {
	Backend *MockSMBBackend

	// Error to return on CreateConnection
	ConnectError error

	// Track connection attempts
	mu               sync.Mutex
	connectionsMade  int
	connectAttempts  int
}

// NewMockConnectionFactory creates a new mock connection factory.
func NewMockConnectionFactory(backend *MockSMBBackend) *MockConnectionFactory {
	return &MockConnectionFactory{
		Backend: backend,
	}
}

// CreateConnection creates a mock SMB connection.
func (f *MockConnectionFactory) CreateConnection(config *Config) (SMBSession, SMBShare, error) {
	f.mu.Lock()
	f.connectAttempts++
	f.mu.Unlock()

	if f.ConnectError != nil {
		return nil, nil, f.ConnectError
	}

	// Add share to backend (method handles its own locking)
	f.Backend.AddShare(config.Share)

	session := NewMockSMBSession(f.Backend)
	share, err := session.Mount(config.Share)
	if err != nil {
		return nil, nil, err
	}

	f.mu.Lock()
	f.connectionsMade++
	f.mu.Unlock()

	return session, share, nil
}

// ConnectionsMade returns the number of successful connections.
func (f *MockConnectionFactory) ConnectionsMade() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connectionsMade
}

// ConnectAttempts returns the total connection attempts.
func (f *MockConnectionFactory) ConnectAttempts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connectAttempts
}

// Reset resets the connection tracking.
func (f *MockConnectionFactory) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connectionsMade = 0
	f.connectAttempts = 0
}
