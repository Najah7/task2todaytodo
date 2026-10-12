package email

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestSMTPSendWritesAlternativeMessage(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	messageReceived := make(chan string, 1)
	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		reader := bufio.NewReader(conn)
		if _, err := fmt.Fprint(conn, "220 localhost ESMTP\r\n"); err != nil {
			serverErr <- err
			return
		}
		var body strings.Builder
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				serverErr <- err
				return
			}
			command := strings.TrimRight(line, "\r\n")
			switch {
			case strings.HasPrefix(command, "EHLO "), strings.HasPrefix(command, "HELO "):
				_, err = fmt.Fprint(conn, "250 localhost\r\n")
			case strings.HasPrefix(command, "MAIL FROM:"):
				_, err = fmt.Fprint(conn, "250 sender accepted\r\n")
			case strings.HasPrefix(command, "RCPT TO:"):
				_, err = fmt.Fprint(conn, "250 recipient accepted\r\n")
			case command == "DATA":
				_, err = fmt.Fprint(conn, "354 continue\r\n")
				if err == nil {
					for {
						var dataLine string
						dataLine, err = reader.ReadString('\n')
						if err != nil {
							break
						}
						if dataLine == ".\r\n" {
							break
						}
						body.WriteString(strings.TrimPrefix(dataLine, "."))
					}
				}
				if err == nil {
					_, err = fmt.Fprint(conn, "250 message accepted\r\n")
				}
			case command == "QUIT":
				_, err = fmt.Fprint(conn, "221 bye\r\n")
				messageReceived <- body.String()
				serverErr <- err
				return
			default:
				_, err = fmt.Fprint(conn, "250 ok\r\n")
			}
			if err != nil {
				serverErr <- err
				return
			}
		}
	}()

	address := listener.Addr().(*net.TCPAddr)
	sender := NewSMTP("127.0.0.1", address.Port, "noreply@example.test", "Task2TodayToDo")
	message := Message{
		To:       "user@example.test",
		Subject:  "Subject",
		TextBody: "Plain text content",
		HTMLBody: "<p>HTML content</p>",
	}
	if err := sender.Send(context.Background(), message); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	select {
	case err := <-serverErr:
		if err != nil {
			t.Fatalf("SMTP test server error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("SMTP test server did not finish")
	}
	got := <-messageReceived
	for _, want := range []string{"multipart/alternative", "text/plain; charset=UTF-8", "text/html; charset=UTF-8", "Plain text content", "HTML content"} {
		if !strings.Contains(got, want) {
			t.Errorf("SMTP message missing %q", want)
		}
	}
}
