package services_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/am-miracle/evictor/internal/api/services"
)

func TestShutdownDrainsInflightRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = response.Write([]byte("completed"))
	})
	serverConnection, clientConnection := net.Pipe()
	listener := newSingleConnectionListener(serverConnection)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- services.Serve(ctx, listener, handler, time.Second)
	}()

	responseDone := make(chan string, 1)
	go func() {
		defer func() { _ = clientConnection.Close() }()
		if _, err := io.WriteString(clientConnection, "GET / HTTP/1.1\r\nHost: evictor.test\r\nConnection: close\r\n\r\n"); err != nil {
			responseDone <- err.Error()
			return
		}
		response, requestErr := http.ReadResponse(bufio.NewReader(clientConnection), nil)
		if requestErr != nil {
			responseDone <- requestErr.Error()
			return
		}
		defer func() { _ = response.Body.Close() }()
		body, _ := io.ReadAll(response.Body)
		responseDone <- string(body)
	}()
	<-started
	cancel()
	close(release)

	if got := <-responseDone; got != "completed" {
		t.Fatalf("in-flight response = %q", got)
	}
	if err := <-done; err != nil {
		t.Fatalf("serve returned %v", err)
	}
}

func TestSIGTERMDuringInflightRequestCompletesAndExitsZero(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestSIGTERMHelperProcess$")
	command.Env = append(os.Environ(), "EVICTOR_SIGTERM_HELPER=1")
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = command.Stdout
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(output)
	if !scanner.Scan() || !strings.HasPrefix(scanner.Text(), "ADDR ") {
		t.Fatalf("helper did not report address: %q", scanner.Text())
	}
	address := strings.TrimPrefix(scanner.Text(), "ADDR ")
	responseDone := make(chan string, 1)
	go func() {
		response, requestErr := http.Get("http://" + address + "/slow")
		if requestErr != nil {
			responseDone <- requestErr.Error()
			return
		}
		defer func() { _ = response.Body.Close() }()
		body, _ := io.ReadAll(response.Body)
		responseDone <- string(body)
	}()
	if !scanner.Scan() || scanner.Text() != "STARTED" {
		t.Fatalf("helper did not start request: %q", scanner.Text())
	}
	if err := command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if got := <-responseDone; got != "completed" {
		t.Fatalf("in-flight response = %q", got)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("helper did not exit zero: %v", err)
	}
}

func TestSIGTERMHelperProcess(t *testing.T) {
	if os.Getenv("EVICTOR_SIGTERM_HELPER") != "1" {
		t.Skip("helper process")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(os.Stdout, "STARTED")
		time.Sleep(100 * time.Millisecond)
		_, _ = response.Write([]byte("completed"))
	})
	_, _ = fmt.Fprintln(os.Stdout, "ADDR", listener.Addr().String())
	if err := services.Serve(ctx, listener, handler, time.Second); err != nil {
		t.Fatal(err)
	}
}

type singleConnectionListener struct {
	connection net.Conn
	closed     chan struct{}
	once       sync.Once
}

func newSingleConnectionListener(connection net.Conn) *singleConnectionListener {
	return &singleConnectionListener{connection: connection, closed: make(chan struct{})}
}

func (listener *singleConnectionListener) Accept() (net.Conn, error) {
	var connection net.Conn
	listener.once.Do(func() {
		connection = listener.connection
	})
	if connection != nil {
		return connection, nil
	}
	<-listener.closed
	return nil, net.ErrClosed
}

func (listener *singleConnectionListener) Close() error {
	select {
	case <-listener.closed:
	default:
		close(listener.closed)
	}
	return nil
}

func (*singleConnectionListener) Addr() net.Addr { return pipeAddress{} }

type pipeAddress struct{}

func (pipeAddress) Network() string { return "pipe" }
func (pipeAddress) String() string  { return "pipe" }
