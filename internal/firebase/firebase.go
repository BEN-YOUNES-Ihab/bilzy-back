package firebase

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// Client wraps the Firebase Auth admin client.
type Client struct {
	auth *auth.Client
}

// New initializes the Firebase admin SDK. If credentialsPath is empty, falls
// back to Application Default Credentials (useful when deployed on GCP).
func New(ctx context.Context, projectID, credentialsPath string) (*Client, error) {
	if projectID == "" {
		return nil, fmt.Errorf("firebase: projectID is required")
	}
	cfg := &firebase.Config{ProjectID: projectID}

	var opts []option.ClientOption
	if credentialsPath != "" {
		opts = append(opts, option.WithCredentialsFile(credentialsPath))
	}

	app, err := firebase.NewApp(ctx, cfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("firebase: init app: %w", err)
	}
	a, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("firebase: init auth: %w", err)
	}
	return &Client{auth: a}, nil
}

// Token is the verified subset we care about.
type Token struct {
	UID         string
	Email       string
	DisplayName string
}

// Verify validates an ID token and returns the canonical claims. Expired or
// malformed tokens return an error; callers should treat any error as a 401.
func (c *Client) Verify(ctx context.Context, idToken string) (*Token, error) {
	t, err := c.auth.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, err
	}
	tok := &Token{UID: t.UID}
	if v, ok := t.Claims["email"].(string); ok {
		tok.Email = v
	}
	if v, ok := t.Claims["name"].(string); ok {
		tok.DisplayName = v
	}
	return tok, nil
}
