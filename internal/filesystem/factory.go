// ABOUTME: Filesystem factory for creating Afero filesystems from URIs
// ABOUTME: Supports local, S3, SFTP/SSH, and HTTP filesystems with automatic detection

package filesystem

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	s3fs "github.com/fclairamb/afero-s3"
	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"golang.org/x/crypto/ssh"
)

// Config holds configuration for filesystem creation
type Config struct {
	// AWS credentials for S3
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	AWSSessionToken    string
	AWSRegion          string

	// SSH credentials for SFTP
	SSHUser           string
	SSHPassword       string
	SSHPrivateKey     string
	SSHPrivateKeyPath string
	SSHKnownHostsPath string
}

// FSInfo contains information about a parsed filesystem path
type FSInfo struct {
	Scheme   string // file, s3, sftp, ssh, http, https
	Host     string
	Port     string
	Bucket   string // For S3
	Path     string
	Original string
}

// ParsePath parses a path/URI and extracts filesystem information
func ParsePath(path string) (*FSInfo, error) {
	info := &FSInfo{
		Original: path,
	}

	// Try to parse as URI
	if strings.Contains(path, "://") {
		u, err := url.Parse(path)
		if err != nil {
			return nil, fmt.Errorf("invalid URI: %w", err)
		}

		info.Scheme = u.Scheme
		info.Host = u.Hostname()
		info.Port = u.Port()
		info.Path = u.Path

		// For S3, extract bucket from host
		if info.Scheme == "s3" {
			info.Bucket = info.Host
			// Remove leading slash from path
			info.Path = strings.TrimPrefix(info.Path, "/")
		}

		return info, nil
	}

	// Default to local file system
	info.Scheme = "file"
	info.Path = path
	return info, nil
}

// GetFilesystem creates an appropriate Afero filesystem based on the path
func GetFilesystem(path string, config *Config) (afero.Fs, error) {
	info, err := ParsePath(path)
	if err != nil {
		return nil, err
	}

	if config == nil {
		config = &Config{}
	}

	switch info.Scheme {
	case "file", "":
		return afero.NewOsFs(), nil

	case "s3":
		return createS3Filesystem(info, config)

	case "sftp", "ssh", "scp":
		return createSFTPFilesystem(info, config)

	case "http", "https":
		// HTTP filesystems are not directly supported yet
		return nil, fmt.Errorf("HTTP filesystem not yet supported")

	default:
		return nil, fmt.Errorf("unsupported filesystem scheme: %s", info.Scheme)
	}
}

// createS3Filesystem creates an S3-backed Afero filesystem
func createS3Filesystem(info *FSInfo, config *Config) (afero.Fs, error) {
	if info.Bucket == "" {
		return nil, fmt.Errorf("S3 URI must specify bucket: s3://bucket/path")
	}

	// Create AWS config
	awsConfig := &aws.Config{}

	// Set region
	region := config.AWSRegion
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if region == "" {
		region = "us-east-1" // Default region
	}
	awsConfig.Region = aws.String(region)

	// Set credentials if provided
	if config.AWSAccessKeyID != "" && config.AWSSecretAccessKey != "" {
		awsConfig.Credentials = credentials.NewStaticCredentials(
			config.AWSAccessKeyID,
			config.AWSSecretAccessKey,
			config.AWSSessionToken,
		)
	}

	// Create AWS session
	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	// Create S3 filesystem
	s3Fs := s3fs.NewFs(info.Bucket, sess)

	return s3Fs, nil
}

// createSFTPFilesystem creates an SFTP-backed Afero filesystem
func createSFTPFilesystem(info *FSInfo, config *Config) (afero.Fs, error) {
	if info.Host == "" {
		return nil, fmt.Errorf("SFTP URI must specify host: sftp://host/path or ssh://user@host/path")
	}

	// Determine username
	username := config.SSHUser
	if username == "" {
		username = os.Getenv("USER")
	}

	// Build SSH client config
	sshConfig := &ssh.ClientConfig{
		User:            username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Implement proper host key verification
	}

	// Add authentication methods
	if config.SSHPassword != "" {
		sshConfig.Auth = append(sshConfig.Auth, ssh.Password(config.SSHPassword))
	}

	if config.SSHPrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(config.SSHPrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH private key: %w", err)
		}
		sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(signer))
	}

	if config.SSHPrivateKeyPath != "" {
		keyBytes, err := os.ReadFile(config.SSHPrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read SSH private key file: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse SSH private key from file: %w", err)
		}
		sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(signer))
	}

	// If no auth methods provided, try default SSH agent and key files
	if len(sshConfig.Auth) == 0 {
		// Try default key locations
		defaultKeys := []string{
			os.Getenv("HOME") + "/.ssh/id_rsa",
			os.Getenv("HOME") + "/.ssh/id_ed25519",
			os.Getenv("HOME") + "/.ssh/id_ecdsa",
		}

		for _, keyPath := range defaultKeys {
			if keyBytes, err := os.ReadFile(keyPath); err == nil {
				if signer, err := ssh.ParsePrivateKey(keyBytes); err == nil {
					sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(signer))
					break
				}
			}
		}
	}

	if len(sshConfig.Auth) == 0 {
		return nil, fmt.Errorf("no SSH authentication method available")
	}

	// Determine port
	port := info.Port
	if port == "" {
		port = "22"
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%s", info.Host, port)
	conn, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH server: %w", err)
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	// Wrap SFTP client in Afero filesystem
	// Note: We need to create a custom Afero FS wrapper for SFTP
	return NewSFTPFs(sftpClient), nil
}

// SFTPFs is an Afero filesystem implementation backed by SFTP
type SFTPFs struct {
	client *sftp.Client
}

// NewSFTPFs creates a new SFTP-backed Afero filesystem
func NewSFTPFs(client *sftp.Client) afero.Fs {
	return &SFTPFs{client: client}
}

// SFTPFile wraps sftp.File to implement afero.File
type SFTPFile struct {
	*sftp.File
	client *sftp.Client
	name   string
}

func (f *SFTPFile) Readdir(count int) ([]os.FileInfo, error) {
	// Use the client to read directory
	return f.client.ReadDir(f.name)
}

func (f *SFTPFile) Readdirnames(n int) ([]string, error) {
	// Read directory and extract names
	entries, err := f.client.ReadDir(f.name)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}

	if n > 0 && len(names) > n {
		names = names[:n]
	}

	return names, nil
}

func (f *SFTPFile) WriteString(s string) (int, error) {
	return f.Write([]byte(s))
}

// Implement Afero Fs interface for SFTP
func (fs *SFTPFs) Create(name string) (afero.File, error) {
	f, err := fs.client.Create(name)
	if err != nil {
		return nil, err
	}
	return &SFTPFile{File: f, client: fs.client, name: name}, nil
}

func (fs *SFTPFs) Mkdir(name string, perm os.FileMode) error {
	return fs.client.Mkdir(name)
}

func (fs *SFTPFs) MkdirAll(path string, perm os.FileMode) error {
	return fs.client.MkdirAll(path)
}

func (fs *SFTPFs) Open(name string) (afero.File, error) {
	f, err := fs.client.Open(name)
	if err != nil {
		return nil, err
	}
	return &SFTPFile{File: f, client: fs.client, name: name}, nil
}

func (fs *SFTPFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	f, err := fs.client.OpenFile(name, flag)
	if err != nil {
		return nil, err
	}
	return &SFTPFile{File: f, client: fs.client, name: name}, nil
}

func (fs *SFTPFs) Remove(name string) error {
	return fs.client.Remove(name)
}

func (fs *SFTPFs) RemoveAll(path string) error {
	return fs.client.RemoveAll(path)
}

func (fs *SFTPFs) Rename(oldname, newname string) error {
	return fs.client.Rename(oldname, newname)
}

func (fs *SFTPFs) Stat(name string) (os.FileInfo, error) {
	return fs.client.Stat(name)
}

func (fs *SFTPFs) Name() string {
	return "SFTPFs"
}

func (fs *SFTPFs) Chmod(name string, mode os.FileMode) error {
	return fs.client.Chmod(name, mode)
}

func (fs *SFTPFs) Chown(name string, uid, gid int) error {
	return fs.client.Chown(name, uid, gid)
}

func (fs *SFTPFs) Chtimes(name string, atime, mtime time.Time) error {
	return fs.client.Chtimes(name, atime, mtime)
}
