package email

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

const sendTimeout = 10 * time.Second

type smtpSender struct {
	host        string
	port        int
	fromAddress string
	fromName    string
}

// NewSMTP creates an unauthenticated SMTP sender for local mail servers such as Mailpit.
func NewSMTP(host string, port int, fromAddress, fromName string) Sender {
	return smtpSender{host: host, port: port, fromAddress: fromAddress, fromName: fromName}
}

func (s smtpSender) Send(ctx context.Context, message Message) error {
	if s.host == "" || s.port < 1 || s.port > 65535 {
		return fmt.Errorf("invalid SMTP server address")
	}
	from, err := parseAddress(s.fromAddress)
	if err != nil {
		return fmt.Errorf("invalid SMTP sender address: %w", err)
	}
	to, err := parseAddress(message.To)
	if err != nil {
		return fmt.Errorf("invalid email recipient: %w", err)
	}
	if err := validateMessage(message); err != nil {
		return err
	}
	body, err := smtpMessage(message, from, s.fromName, to)
	if err != nil {
		return err
	}

	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(sendCtx, "tcp", net.JoinHostPort(s.host, strconv.Itoa(s.port)))
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}
	defer conn.Close()
	deadline, ok := sendCtx.Deadline()
	if ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return fmt.Errorf("set SMTP deadline: %w", err)
		}
	}
	stopCancellation := context.AfterFunc(sendCtx, func() {
		_ = conn.SetDeadline(time.Now())
	})
	defer stopCancellation()

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("start SMTP session: %w", err)
	}
	defer client.Close()
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("start SMTP message: %w", err)
	}
	if _, err := writer.Write(body); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("close SMTP session: %w", err)
	}
	return nil
}

func parseAddress(value string) (*mail.Address, error) {
	address, err := mail.ParseAddress(value)
	if err != nil {
		return nil, err
	}
	return address, nil
}

func validateMessage(message Message) error {
	if strings.TrimSpace(message.Subject) == "" {
		return fmt.Errorf("email subject is required")
	}
	if message.TextBody == "" && message.HTMLBody == "" {
		return fmt.Errorf("email body is required")
	}
	return nil
}

func encodedAddress(address *mail.Address) string {
	if address.Name == "" {
		return address.Address
	}
	return mime.QEncoding.Encode("UTF-8", address.Name) + " <" + address.Address + ">"
}

func smtpMessage(message Message, from *mail.Address, fromName string, to *mail.Address) ([]byte, error) {
	var out bytes.Buffer
	fmt.Fprintf(&out, "From: %s\r\n", encodedAddress(&mail.Address{Name: fromName, Address: from.Address}))
	fmt.Fprintf(&out, "To: %s\r\n", encodedAddress(to))
	fmt.Fprintf(&out, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", message.Subject))
	out.WriteString("MIME-Version: 1.0\r\n")
	writer := multipart.NewWriter(&out)
	boundary := writer.Boundary()
	fmt.Fprintf(&out, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary)
	for _, body := range []struct {
		contentType string
		content     string
	}{{"text/plain", message.TextBody}, {"text/html", message.HTMLBody}} {
		if body.content == "" {
			continue
		}
		header := make(textproto.MIMEHeader)
		header.Set("Content-Type", body.contentType+"; charset=UTF-8")
		header.Set("Content-Transfer-Encoding", "quoted-printable")
		part, err := writer.CreatePart(header)
		if err != nil {
			return nil, fmt.Errorf("create email body part: %w", err)
		}
		encoded := quotedprintable.NewWriter(part)
		if _, err := encoded.Write([]byte(body.content)); err != nil {
			return nil, fmt.Errorf("encode email body part: %w", err)
		}
		if err := encoded.Close(); err != nil {
			return nil, fmt.Errorf("finish email body part: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("finish email body: %w", err)
	}
	return out.Bytes(), nil
}
