// Package privateaccess provides a command-scoped, TLS-transparent EKS route.
package privateaccess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"k8s.io/client-go/tools/clientcmd"
)

const ProxyEnv = "QUARTZ_KUBE_PROXY_URL"
const ConfigEnv = "QUARTZ_PRIVATE_KUBECONFIG"

type Options struct {
	Cluster, Region, Endpoint, Directory string
	Kubeconfig                           []byte
}

type Session struct {
	ProxyURL, KubeconfigPath string
	server                   *http.Server
	forward                  *exec.Cmd
	wait                     <-chan error
	mu                       sync.Mutex
	connections              map[net.Conn]bool
	closed                   bool
	restore                  func()
	metadata                 *sessionMetadata
	region                   string
	closeOnce                sync.Once
	closeError               error
}

type sessionMetadata struct {
	mu     sync.Mutex
	buffer string
	id     string
}

var sessionMarker = regexp.MustCompile(`Starting session with SessionId:[ \t]+([a-zA-Z0-9_+=,.@:-]+)\r?\n`)

func (m *sessionMetadata) Write(data []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.id == "" {
		m.buffer += string(data)
		if len(m.buffer) > 4096 {
			m.buffer = m.buffer[len(m.buffer)-4096:]
		}
		if match := sessionMarker.FindStringSubmatch(m.buffer); len(match) == 2 {
			m.id = match[1]
			m.buffer = ""
		}
	}
	return len(data), nil
}

func (m *sessionMetadata) ready() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.id != ""
}

func Start(ctx context.Context, options Options) (*Session, error) {
	if _, err := exec.LookPath("session-manager-plugin"); err != nil {
		return nil, fmt.Errorf("private EKS access requires session-manager-plugin: %w", err)
	}
	if _, err := exec.LookPath("aws"); err != nil {
		return nil, fmt.Errorf("private EKS access requires AWS CLI: %w", err)
	}
	endpoint, err := url.Parse(options.Endpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || strings.Trim(endpoint.Path, "/") != "" || (endpoint.Port() != "" && endpoint.Port() != "443") {
		return nil, errors.New("private access requires the unmodified HTTPS EKS endpoint")
	}
	var ids []string
	output, err := exec.CommandContext(ctx, "aws", "ec2", "describe-instances", "--region", options.Region,
		"--filters", "Name=tag:quartz:access-relay,Values="+options.Cluster, "Name=instance-state-name,Values=running",
		"--query", "Reservations[].Instances[].InstanceId", "--output", "json").Output()
	if err != nil {
		return nil, fmt.Errorf("discover private relay: %w", err)
	}
	if err := json.Unmarshal(output, &ids); err != nil || len(ids) != 1 {
		return nil, errors.New("exactly one running cluster-owned private SSM relay is required")
	}
	if err := waitManagedRelay(ctx, options.Region, ids[0]); err != nil {
		return nil, err
	}
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	_, port, _ := net.SplitHostPort(reservation.Addr().String())
	reservation.Close()
	parameters, _ := json.Marshal(map[string][]string{"host": {endpoint.Hostname()}, "portNumber": {"443"}, "localPortNumber": {port}})
	command := exec.Command("aws", "ssm", "start-session", "--region", options.Region, "--target", ids[0],
		"--document-name", "AWS-StartPortForwardingSessionToRemoteHost", "--parameters", string(parameters))
	configureProcess(command)
	// Session output can contain identity/session metadata; never send it to logs.
	metadata := &sessionMetadata{}
	command.Stdout, command.Stderr = metadata, io.Discard
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start SSM forwarding: %w", err)
	}
	wait := make(chan error, 1)
	go func() { wait <- command.Wait(); close(wait) }()
	local := net.JoinHostPort("127.0.0.1", port)
	started := &Session{forward: command, wait: wait, metadata: metadata, region: options.Region}
	if err := waitForward(ctx, local, wait, metadata); err != nil {
		started.Close()
		return nil, err
	}
	session, err := NewTunnel(net.JoinHostPort(endpoint.Hostname(), "443"), local)
	if err != nil {
		started.Close()
		return nil, err
	}
	session.forward, session.wait = command, wait
	session.metadata, session.region = metadata, options.Region
	if err := session.WriteKubeconfig(options.Directory, options.Kubeconfig); err != nil {
		session.Close()
		return nil, err
	}
	session.restore = bindEnvironment(session.ProxyURL, session.KubeconfigPath)
	go func() {
		select {
		case <-ctx.Done():
		case <-wait:
		}
		session.Close()
	}()
	return session, nil
}

func waitManagedRelay(ctx context.Context, region, id string) error {
	deadline := time.NewTimer(3 * time.Minute)
	defer deadline.Stop()
	for {
		output, err := exec.CommandContext(ctx, "aws", "ssm", "describe-instance-information", "--region", region,
			"--filters", "Key=InstanceIds,Values="+id, "--query", "InstanceInformationList[].PingStatus", "--output", "json").Output()
		var statuses []string
		if err == nil && json.Unmarshal(output, &statuses) == nil && len(statuses) == 1 && statuses[0] == "Online" {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("private relay did not become SSM Online; check endpoint DNS/SGs/agent and caller permissions")
		case <-time.After(2 * time.Second):
		}
	}
}

func waitForward(ctx context.Context, local string, wait <-chan error, metadata *sessionMetadata) error {
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	for {
		connection, err := net.DialTimeout("tcp", local, 200*time.Millisecond)
		if err == nil {
			connection.Close()
			if metadata.ready() {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-wait:
			return errors.New("SSM session exited before forwarding became ready")
		case <-deadline.C:
			return errors.New("SSM forwarding/session identity did not become ready")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// NewTunnel only permits CONNECT to one EKS authority; TLS remains end-to-end.
func NewTunnel(authority, local string) (*Session, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	session := &Session{ProxyURL: "http://" + listener.Addr().String(), connections: make(map[net.Conn]bool)}
	session.server = &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect || r.Host != authority || r.URL.Host != authority {
			http.Error(w, "only the configured EKS CONNECT authority is permitted", http.StatusForbidden)
			return
		}
		upstream, err := net.DialTimeout("tcp", local, 5*time.Second)
		if err != nil {
			http.Error(w, "private forwarding unavailable", http.StatusBadGateway)
			return
		}
		client, buffered, err := w.(http.Hijacker).Hijack()
		if err != nil {
			upstream.Close()
			return
		}
		session.mu.Lock()
		if session.closed {
			session.mu.Unlock()
			client.Close()
			upstream.Close()
			return
		}
		session.connections[client], session.connections[upstream] = true, true
		session.mu.Unlock()
		defer func() {
			client.Close()
			upstream.Close()
			session.mu.Lock()
			delete(session.connections, client)
			delete(session.connections, upstream)
			session.mu.Unlock()
		}()
		if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			return
		}
		if err := buffered.Flush(); err != nil {
			return
		}
		done := make(chan struct{})
		go func() { io.Copy(client, upstream); client.Close(); close(done) }()
		io.Copy(upstream, buffered)
		upstream.Close()
		<-done
	})}
	go session.server.Serve(listener)
	return session, nil
}

func (s *Session) WriteKubeconfig(directory string, original []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("private session is already closed")
	}
	config, err := clientcmd.Load(original)
	if err != nil {
		return err
	}
	if len(config.Clusters) != 1 {
		return errors.New("private context requires exactly one EKS kubeconfig cluster")
	}
	for _, cluster := range config.Clusters {
		if cluster.InsecureSkipTLSVerify || len(cluster.CertificateAuthorityData) == 0 {
			return errors.New("private EKS kubeconfig must retain CA verification")
		}
		cluster.ProxyURL = s.ProxyURL
	}
	data, err := clientcmd.Write(*config)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, "private-session-*.yaml")
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	closeErr := file.Close()
	if err := errors.Join(err, closeErr); err != nil {
		os.Remove(file.Name())
		return err
	}
	if s.KubeconfigPath != "" {
		if err := os.Rename(file.Name(), s.KubeconfigPath); err != nil {
			os.Remove(file.Name())
			return err
		}
	} else {
		s.KubeconfigPath = file.Name()
	}
	return nil
}

func bindEnvironment(proxy, path string) func() {
	previous := make(map[string]*string)
	for name, value := range map[string]string{ProxyEnv: proxy, ConfigEnv: path} {
		if old, ok := os.LookupEnv(name); ok {
			copy := old
			previous[name] = &copy
		} else {
			previous[name] = nil
		}
		os.Setenv(name, value)
	}
	return func() {
		for name, value := range previous {
			if value == nil {
				os.Unsetenv(name)
			} else {
				os.Setenv(name, *value)
			}
		}
	}
}

func ProxyURL() (*url.URL, error) {
	value := os.Getenv(ProxyEnv)
	if value == "" {
		return nil, nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "http" || parsed.Hostname() != "127.0.0.1" || parsed.Port() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Trim(parsed.Path, "/") != "" {
		return nil, errors.New("private proxy must be a credential-free loopback HTTP CONNECT endpoint")
	}
	return parsed, nil
}

func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		if s.server != nil {
			s.server.Close()
		}
		s.mu.Lock()
		s.closed = true
		for connection := range s.connections {
			connection.Close()
		}
		s.mu.Unlock()
		if s.forward != nil {
			select {
			case <-s.wait:
			default:
				terminateProcess(s.forward)
				select {
				case <-s.wait:
				case <-time.After(2 * time.Second):
					forceProcess(s.forward)
					select {
					case <-s.wait:
					case <-time.After(2 * time.Second):
					}
				}
			}
			if s.metadata != nil {
				s.metadata.mu.Lock()
				id := s.metadata.id
				s.metadata.id, s.metadata.buffer = "", ""
				s.metadata.mu.Unlock()
				if id != "" {
					// Explicitly terminate even if the plugin was forcibly stopped.
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					command := exec.CommandContext(ctx, "aws", "ssm", "terminate-session", "--region", s.region, "--session-id", id)
					command.Stdout, command.Stderr = io.Discard, io.Discard
					if err := command.Run(); err != nil {
						s.closeError = fmt.Errorf("terminate private SSM session: %w", err)
					}
					cancel()
				}
			}
		}
		if s.KubeconfigPath != "" {
			if err := os.Remove(s.KubeconfigPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				s.closeError = errors.Join(s.closeError, errors.New("remove private session kubeconfig failed"))
			}
		}
		if s.restore != nil {
			s.restore()
		}
	})
	return s.closeError
}
