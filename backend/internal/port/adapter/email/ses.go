package email

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

type sesClient interface {
	SendEmail(context.Context, *sesv2.SendEmailInput, ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

type sesSender struct {
	client      sesClient
	fromAddress *mail.Address
	fromName    string
}

// NewSES creates a sender using AWS SDK default credential discovery and SESv2.
func NewSES(ctx context.Context, region, fromAddress, fromName string) (Sender, error) {
	if strings.TrimSpace(region) == "" {
		return nil, fmt.Errorf("AWS region is required")
	}
	from, err := parseAddress(fromAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid SES sender address: %w", err)
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS configuration: %w", err)
	}
	return sesSender{
		client:      sesv2.NewFromConfig(cfg),
		fromAddress: from,
		fromName:    fromName,
	}, nil
}

func (s sesSender) Send(ctx context.Context, message Message) error {
	to, err := parseAddress(message.To)
	if err != nil {
		return fmt.Errorf("invalid email recipient: %w", err)
	}
	if err := validateMessage(message); err != nil {
		return err
	}
	sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
	defer cancel()
	from := *s.fromAddress
	if s.fromName != "" {
		from.Name = s.fromName
	}
	_, err = s.client.SendEmail(sendCtx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(encodedAddress(&from)),
		Destination:      &types.Destination{ToAddresses: []string{to.Address}},
		Content: &types.EmailContent{Simple: &types.Message{
			Subject: &types.Content{Data: aws.String(message.Subject), Charset: aws.String("UTF-8")},
			Body: &types.Body{
				Text: contentIfPresent(message.TextBody),
				Html: contentIfPresent(message.HTMLBody),
			},
		}},
	})
	if err != nil {
		return fmt.Errorf("send email with SES: %w", err)
	}
	return nil
}

func contentIfPresent(value string) *types.Content {
	if value == "" {
		return nil
	}
	return &types.Content{Data: aws.String(value), Charset: aws.String("UTF-8")}
}
