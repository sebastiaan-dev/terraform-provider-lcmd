// Copyright (c) HashiCorp, Inc.

package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

var errNotFound = errors.New("resource not found")

type apiAppInfo struct {
	AppID    string `json:"appid"`
	DeployID string `json:"deploy_id"`
	LpkID    string `json:"lpk_id"`
	Title    string `json:"title"`
	Version  string `json:"version"`
	Domain   string `json:"domain"`
	Owner    string `json:"owner"`
}

type apiInstallRequest struct {
	UID       string `json:"uid"`
	LPKURL    string `json:"lpk_url"`
	Wait      bool   `json:"wait"`
	Ephemeral bool   `json:"ephemeral"`
}

type apiUser struct {
	UID      string `json:"uid"`
	Nickname string `json:"nickname"`
}

type LcmdClient struct {
	baseURL    *url.URL
	httpClient *http.Client
	authHeader string
	User       string
}

func newAPIClient(endpoint, username, password string) (*LcmdClient, error) {
	if endpoint == "" {
		return nil, errors.New("endpoint is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	return &LcmdClient{
		baseURL: parsed,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		authHeader: auth,
	}, nil
}

func (c *LcmdClient) InstallApp(ctx context.Context, lpkURL string, wait bool, ephemeral bool) (*apiAppInfo, error) {
	if c.User == "" {
		return nil, errors.New("user uid is not configured")
	}
	payload := &apiInstallRequest{UID: c.User, LPKURL: lpkURL, Wait: wait, Ephemeral: ephemeral}
	var app apiAppInfo
	if err := c.do(ctx, http.MethodPost, "/v1/apps", nil, payload, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

func (c *LcmdClient) GetApp(ctx context.Context, appID string) (*apiAppInfo, error) {
	if c.User == "" {
		return nil, errors.New("user uid is not configured")
	}
	params := map[string]string{"uid": c.User}
	var app apiAppInfo
	err := c.do(ctx, http.MethodGet, path.Join("/v1/apps", appID), params, nil, &app)
	if errors.Is(err, errNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &app, nil
}

func (c *LcmdClient) DeleteApp(ctx context.Context, appID string, clearData bool) error {
	if c.User == "" {
		return errors.New("user uid is not configured")
	}
	params := map[string]string{
		"uid":        c.User,
		"clear_data": fmt.Sprintf("%t", clearData),
	}
	return c.do(ctx, http.MethodDelete, path.Join("/v1/apps", appID), params, nil, nil)
}

func (c *LcmdClient) ListUsers(ctx context.Context) ([]apiUser, error) {
	data, err := c.doRaw(ctx, http.MethodGet, "/v1/users", nil, nil)
	if err != nil {
		return nil, err
	}
	var detailed []apiUser
	if err := json.Unmarshal(data, &detailed); err == nil {
		return detailed, nil
	}
	var simple []string
	if err := json.Unmarshal(data, &simple); err == nil {
		users := make([]apiUser, len(simple))
		for i, uid := range simple {
			users[i] = apiUser{UID: uid}
		}
		return users, nil
	}
	return nil, fmt.Errorf("unexpected users payload: %s", string(data))
}

func (c *LcmdClient) do(ctx context.Context, method string, p string, query map[string]string, body interface{}, out interface{}) error {
	data, err := c.doRaw(ctx, method, p, query, body)
	if err != nil {
		return err
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

func (c *LcmdClient) buildURL(p string, query map[string]string) string {
	u := *c.baseURL
	cleanPath := strings.TrimSuffix(u.Path, "/") + p
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}
	u.Path = cleanPath
	q := u.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (c *LcmdClient) doRaw(ctx context.Context, method string, p string, query map[string]string, body interface{}) ([]byte, error) {
	endpoint := c.buildURL(p, query)
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api %s %s: %s", method, p, strings.TrimSpace(string(msg)))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}
