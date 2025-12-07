// Copyright (c) HashiCorp, Inc.

package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	datasourcevalidator "github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"gopkg.in/yaml.v3"
)

var _ datasource.DataSource = &LPKBuildDataSource{}

type LPKBuildDataSource struct {
	client *LcmdClient
}

type LPKBuildModel struct {
	ID        types.String          `tfsdk:"id"`
	Source    *LPKBuildSourceModel  `tfsdk:"source"`
	Build     *LPKBuildBuildModel   `tfsdk:"build"`
	Publish   *LPKBuildPublishModel `tfsdk:"publish"`
	Env       *LPKBuildEnvModel     `tfsdk:"env"`
	LPKURL    types.String          `tfsdk:"lpk_url"`
	SHA256    types.String          `tfsdk:"sha256"`
	AppID     types.String          `tfsdk:"appid"`
	Version   types.String          `tfsdk:"version"`
	LocalPath types.String          `tfsdk:"local_path"`
	UploadID  types.String          `tfsdk:"upload_id"`
}

type LPKBuildSourceModel struct {
	Local *LPKBuildSourceLocalModel `tfsdk:"local"`
	Git   *LPKBuildSourceGitModel   `tfsdk:"git"`
}

type LPKBuildSourceLocalModel struct {
	Path types.String `tfsdk:"path"`
}

type LPKBuildSourceGitModel struct {
	URL     types.String `tfsdk:"url"`
	Ref     types.String `tfsdk:"ref"`
	Subpath types.String `tfsdk:"subpath"`
}

type LPKBuildBuildModel struct {
	Command types.String `tfsdk:"command"`
}

type LPKBuildPublishModel struct {
	Enabled types.Bool   `tfsdk:"enabled"`
	Name    types.String `tfsdk:"name"`
	Version types.String `tfsdk:"version"`
}

type LPKBuildEnvModel struct {
	Variables         map[string]types.String `tfsdk:"variables"`
	TemplateExtension types.String            `tfsdk:"template_extension"`
}

const defaultTemplateExtension = ".tmpl"

func NewLPKBuildDataSource() datasource.DataSource {
	return &LPKBuildDataSource{}
}

func (d *LPKBuildDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lpk_build"
}

func (d *LPKBuildDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		// At least one of local/git must be set
		datasourcevalidator.AtLeastOneOf(
			path.MatchRoot("source").AtName("local"),
			path.MatchRoot("source").AtName("git"),
		),
		// They must not both be set at the same time
		datasourcevalidator.Conflicting(
			path.MatchRoot("source").AtName("local"),
			path.MatchRoot("source").AtName("git"),
		),
	}
}

func (d *LPKBuildDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Builds an LPK from source and optionally uploads it to the NAS registry.",
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Computed: true},
			"lpk_url":    schema.StringAttribute{Computed: true, Description: "Download URL returned by NAS registry."},
			"sha256":     schema.StringAttribute{Computed: true},
			"appid":      schema.StringAttribute{Computed: true},
			"version":    schema.StringAttribute{Computed: true},
			"local_path": schema.StringAttribute{Computed: true},
			"upload_id":  schema.StringAttribute{Computed: true},
			"source": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"local": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"path": schema.StringAttribute{
								Required: true,
							},
						},
					},
					"git": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"url": schema.StringAttribute{
								Required: true,
							},
							"ref": schema.StringAttribute{
								Optional: true,
							},
							"subpath": schema.StringAttribute{
								Optional: true,
							},
						},
					},
				},
				Validators: []validator.Object{},
			},
		},

		Blocks: map[string]schema.Block{
			"build": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"command": schema.StringAttribute{
						Optional: true,
					},
				},
			},
			"publish": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Optional: true,
					},
					"name": schema.StringAttribute{
						Optional: true,
					},
					"version": schema.StringAttribute{
						Optional: true,
					},
				},
			},
			"env": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"variables": schema.MapAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Key-value pairs exposed to template rendering and build commands.",
					},
					"template_extension": schema.StringAttribute{
						Optional:    true,
						Description: "File extension (e.g., .tmpl or .j2) considered a template. Defaults to .tmpl.",
					},
				},
			},
		},
	}
}

func (d *LPKBuildDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*LcmdClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected DataSource Configure Type", fmt.Sprintf("Expected *LcmdClient, got %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *LPKBuildDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "")
		return
	}
	var data LPKBuildModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	workdir, cleanup, err := d.prepareSource(ctx, data.Source)
	if err != nil {
		resp.Diagnostics.AddError("Source error", err.Error())
		return
	}
	if cleanup != nil {
		defer cleanup()
	}
	envVars := collectEnvVars(data.Env)
	ext := resolveTemplateExtension(data.Env)
	if err := renderTemplateFiles(workdir, ext, envVars); err != nil {
		resp.Diagnostics.AddError("Template error", err.Error())
		return
	}
	lpkPath, meta, err := d.runBuild(ctx, workdir, data.Build, data.Publish, envVars)
	if err != nil {
		resp.Diagnostics.AddError("Build error", err.Error())
		return
	}
	data.LocalPath = types.StringValue(lpkPath)
	data.AppID = types.StringValue(meta.AppID)
	data.Version = types.StringValue(meta.Version)
	data.SHA256 = types.StringValue(meta.SHA256)
	data.LPKURL = types.StringNull()
	data.UploadID = types.StringNull()
	if shouldPublish(data.Publish) {
		uploadName := meta.Name
		if data.Publish != nil && !data.Publish.Name.IsNull() && data.Publish.Name.ValueString() != "" {
			uploadName = data.Publish.Name.ValueString()
		}
		uploadVersion := meta.Version
		if data.Publish != nil && !data.Publish.Version.IsNull() && data.Publish.Version.ValueString() != "" {
			uploadVersion = data.Publish.Version.ValueString()
		}
		upload, err := d.client.UploadLPK(ctx, d.client.User, uploadName, uploadVersion, lpkPath)
		if err != nil {
			resp.Diagnostics.AddError("Upload error", err.Error())
			return
		}
		data.LPKURL = types.StringValue(upload.DownloadURL)
		data.UploadID = types.StringValue(upload.ID)
		if upload.SHA256 != "" {
			data.SHA256 = types.StringValue(upload.SHA256)
		}
		if upload.Version != "" {
			data.Version = types.StringValue(upload.Version)
		}
	}
	data.ID = types.StringValue(fmt.Sprintf("%s-%s-%s", meta.AppID, meta.Version, meta.SHA256))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *LPKBuildDataSource) prepareSource(ctx context.Context, source *LPKBuildSourceModel) (string, func(), error) {
	if source == nil {
		return "", nil, errors.New("source block is required")
	}
	if source.Local != nil {
		if source.Local.Path.IsNull() || source.Local.Path.ValueString() == "" {
			return "", nil, errors.New("local.path must be set")
		}
		return source.Local.Path.ValueString(), nil, nil
	}
	if source.Git != nil {
		if source.Git.URL.IsNull() || source.Git.URL.ValueString() == "" {
			return "", nil, errors.New("git.url must be set")
		}
		tmp, err := os.MkdirTemp("", "lpk-build-*")
		if err != nil {
			return "", nil, err
		}
		cleanup := func() { _ = os.RemoveAll(tmp) }
		clone := exec.CommandContext(ctx, "git", "clone", source.Git.URL.ValueString(), "repo")
		clone.Dir = tmp
		clone.Stdout = os.Stdout
		clone.Stderr = os.Stderr
		if err := clone.Run(); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("git clone failed: %w", err)
		}
		repoPath := filepath.Join(tmp, "repo")
		if !source.Git.Ref.IsNull() && source.Git.Ref.ValueString() != "" {
			checkout := exec.CommandContext(ctx, "git", "checkout", source.Git.Ref.ValueString())
			checkout.Dir = repoPath
			checkout.Stdout = os.Stdout
			checkout.Stderr = os.Stderr
			if err := checkout.Run(); err != nil {
				cleanup()
				return "", nil, fmt.Errorf("git checkout failed: %w", err)
			}
		}
		sub := repoPath
		if !source.Git.Subpath.IsNull() && source.Git.Subpath.ValueString() != "" {
			sub = filepath.Join(repoPath, source.Git.Subpath.ValueString())
		}
		return sub, cleanup, nil
	}
	return "", nil, errors.New("either source.local or source.git must be provided")
}

type lpkMetadata struct {
	AppID   string
	Version string
	SHA256  string
	Name    string
}

func (d *LPKBuildDataSource) runBuild(ctx context.Context, path string, build *LPKBuildBuildModel, pub *LPKBuildPublishModel, envVars map[string]string) (string, *lpkMetadata, error) {
	manifestPath := filepath.Join(path, "lzc-manifest.yml")
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return "", nil, fmt.Errorf("read manifest: %w", err)
	}
	if manifest.Name == "" {
		return "", nil, errors.New("manifest name must be set")
	}
	if manifest.Version == "" {
		return "", nil, errors.New("manifest version must be set")
	}
	manifestHash, err := computeSHA(manifestPath)
	if err != nil {
		return "", nil, fmt.Errorf("compute manifest hash: %w", err)
	}
	artifactBase := fmt.Sprintf("%s-%s-%s", manifest.Name, manifest.Version, manifestHash)
	artifactPath := filepath.Join(path, artifactBase+".lpk")
	if _, statErr := os.Stat(artifactPath); errors.Is(statErr, os.ErrNotExist) {
		command := "npx lzc-cli project build ."
		if build != nil && !build.Command.IsNull() && build.Command.ValueString() != "" {
			command = build.Command.ValueString()
		}
		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		cmd.Dir = path
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if len(envVars) > 0 {
			cmd.Env = commandEnvironment(envVars)
		}
		if err := cmd.Run(); err != nil {
			return "", nil, err
		}
		out, err := findLatestLPK(path)
		if err != nil {
			return "", nil, err
		}
		if out != artifactPath {
			if err := os.Rename(out, artifactPath); err != nil {
				return "", nil, fmt.Errorf("rename artifact: %w", err)
			}
		}
	} else if statErr != nil {
		return "", nil, fmt.Errorf("check artifact: %w", statErr)
	}
	sha, err := computeSHA(artifactPath)
	if err != nil {
		return "", nil, err
	}
	meta := &lpkMetadata{
		AppID:   manifest.AppID,
		Version: manifest.Version,
		SHA256:  sha,
		Name:    artifactBase,
	}
	if pub != nil && !pub.Version.IsNull() && pub.Version.ValueString() != "" {
		meta.Version = pub.Version.ValueString()
	}
	if pub != nil && !pub.Name.IsNull() && pub.Name.ValueString() != "" {
		meta.Name = pub.Name.ValueString()
	}
	return artifactPath, meta, nil
}

func collectEnvVars(env *LPKBuildEnvModel) map[string]string {
	if env == nil || len(env.Variables) == 0 {
		return nil
	}
	values := make(map[string]string, len(env.Variables))
	for key, value := range env.Variables {
		if value.IsNull() || value.IsUnknown() {
			continue
		}
		values[key] = value.ValueString()
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func resolveTemplateExtension(env *LPKBuildEnvModel) string {
	if env == nil || env.TemplateExtension.IsNull() || env.TemplateExtension.IsUnknown() {
		return defaultTemplateExtension
	}
	ext := strings.TrimSpace(env.TemplateExtension.ValueString())
	if ext == "" {
		return defaultTemplateExtension
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}

func renderTemplateFiles(baseDir, extension string, envVars map[string]string) error {
	ext := extension
	if ext == "" {
		ext = defaultTemplateExtension
	}
	return filepath.WalkDir(baseDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ext) {
			return nil
		}
		return renderTemplateFile(path, ext, envVars)
	})
}

func renderTemplateFile(path, extension string, envVars map[string]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read template %s: %w", path, err)
	}
	tmpl, err := template.New(filepath.Base(path)).Option("missingkey=error").Parse(string(data))
	if err != nil {
		return fmt.Errorf("parse template %s: %w", path, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, envVars); err != nil {
		return fmt.Errorf("render template %s: %w", path, formatTemplateError(err))
	}
	dest := strings.TrimSuffix(path, extension)
	perm := fs.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}
	if err := os.WriteFile(dest, buf.Bytes(), perm); err != nil {
		return fmt.Errorf("write rendered template %s: %w", dest, err)
	}
	return nil
}

func formatTemplateError(err error) error {
	var execErr *template.ExecError
	if errors.As(err, &execErr) {
		if missing := extractMissingKey(execErr.Err); missing != "" {
			return fmt.Errorf("environment variable %s not provided", missing)
		}
		return execErr.Err
	}
	return err
}

func extractMissingKey(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	const prefix = "map has no entry for key "
	if !strings.HasPrefix(msg, prefix) {
		return ""
	}
	return strings.Trim(msg[len(prefix):], "\"")
}

func commandEnvironment(custom map[string]string) []string {
	if len(custom) == 0 {
		return nil
	}
	values := make(map[string]string)
	for _, pair := range os.Environ() {
		if idx := strings.Index(pair, "="); idx > 0 {
			values[pair[:idx]] = pair[idx+1:]
		}
	}
	for key, value := range custom {
		values[key] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return env
}

func shouldPublish(pub *LPKBuildPublishModel) bool {
	if pub == nil || pub.Enabled.IsNull() {
		return true
	}
	return pub.Enabled.ValueBool()
}

func findLatestLPK(dir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.lpk"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", errors.New("no .lpk artifact produced")
	}
	sort.Slice(matches, func(i, j int) bool {
		iInfo, _ := os.Stat(matches[i])
		jInfo, _ := os.Stat(matches[j])
		return iInfo.ModTime().After(jInfo.ModTime())
	})
	return matches[0], nil
}

type manifestYAML struct {
	AppID   string `yaml:"appid"`
	Version string `yaml:"version"`
	Name    string `yaml:"name"`
}

func readManifest(path string) (*manifestYAML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &manifestYAML{}, err
	}
	var m manifestYAML
	if err := yaml.Unmarshal(data, &m); err != nil {
		return &manifestYAML{}, err
	}
	return &m, nil
}

func computeSHA(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
