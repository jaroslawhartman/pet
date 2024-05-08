package sync

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/briandowns/spinner"
	"github.com/knqyf263/pet/config"
	"github.com/pkg/errors"
	"github.com/xanzy/go-gitlab"
)

const (
	gitlabTokenEnvVariable = "PET_GITLAB_ACCESS_TOKEN"
)

// GitLabClient manages communication with GitLab Snippets
type GitLabClient struct {
	Client *gitlab.Client
	ID     int
}

// NewGitLabClient returns GitLabClient
func NewGitLabClient() (Client, error) {
	accessToken, err := getGitlabAccessToken()
	if err != nil {
		return nil, fmt.Errorf(`access_token is empty.
Go https://gitlab.com/profile/personal_access_tokens and create access_token.
Write access_token in config file (pet configure) or export $%v.
		`, gitlabTokenEnvVariable)
	}

	u := "https://git.mydomain.com/api/v4"
	id := 0

	h := &http.Client{}

	if config.Conf.GitLab.Url != "" {
		fmt.Println(config.Conf.GitLab.Url)
		u = config.Conf.GitLab.Url
	}

	if config.Conf.GitLab.SkipSsl == true {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		h = &http.Client{Transport: tr}
	}

	c, err := gitlab.NewClient(accessToken, gitlab.WithBaseURL(u), gitlab.WithHTTPClient(h))
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create GitLab client: %d", id)
	}

	if config.Conf.GitLab.ID == "" {
		client := GitLabClient{
			Client: c,
			ID:     id,
		}

		return client, nil
	}

	id, err = strconv.Atoi(config.Conf.GitLab.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "Invalid GitLab Snippet ID: %d", id)
	}

	client := GitLabClient{
		Client: c,
		ID:     id,
	}

	return client, nil
}

func getGitlabAccessToken() (string, error) {
	if config.Conf.GitLab.AccessToken != "" {
		return config.Conf.GitLab.AccessToken, nil
	} else if os.Getenv(gitlabTokenEnvVariable) != "" {
		return os.Getenv(gitlabTokenEnvVariable), nil
	}
	return "", errors.New("GitLab AccessToken not found in any source")
}

// GetSnippet returns the remote snippet
func (g GitLabClient) GetSnippet() (*Snippet, error) {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Start()
	s.Suffix = " Getting GitLab Snippet..."
	defer s.Stop()

	if g.ID == 0 {
		return &Snippet{}, nil
	}

	snippet, res, err := g.Client.Snippets.GetSnippet(g.ID)
	if err != nil {
		if res.StatusCode == 404 {
			return nil, errors.Wrapf(err, "No GitLab Snippet ID (%d)", g.ID)
		}
		return nil, errors.Wrapf(err, "Failed to get GitLab Snippet (ID: %d)", g.ID)
	}

	filename := config.Conf.GitLab.FileName
	if snippet.FileName != filename {
		return nil, fmt.Errorf("No snippet file in GitLab Snippet (ID: %d)", g.ID)
	}

	contentByte, _, err := g.Client.Snippets.SnippetContent(g.ID)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get GitLab Snippet content (ID: %d)", g.ID)
	}

	content := string(contentByte)
	if content == "" {
		return nil, fmt.Errorf("%s is empty", filename)
	}

	return &Snippet{
		Content:   content,
		UpdatedAt: *snippet.UpdatedAt,
	}, nil
}

// UploadSnippet uploads local snippets to GitLab Snippet
func (g GitLabClient) UploadSnippet(content string) error {
	if g.ID == 0 {
		id, err := g.createSnippet(context.Background(), content)
		if err != nil {
			return errors.Wrap(err, "Failed to create GitLab Snippet")
		}
		fmt.Printf("GitLab Snippet ID: %d\n", id)
	} else {
		if err := g.updateSnippet(context.Background(), content); err != nil {
			return errors.Wrap(err, "Failed to update GitLab Snippet")
		}
	}
	return nil
}

func (g GitLabClient) createSnippet(ctx context.Context, content string) (id int, err error) {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Start()
	s.Suffix = " Creating GitLab Snippet..."
	defer s.Stop()

	opt := &gitlab.CreateSnippetOptions{
		Title:       gitlab.String("pet-snippet"),
		FileName:    gitlab.String(config.Conf.GitLab.FileName),
		Description: gitlab.String("Snippet file generated by pet"),
		Content:     gitlab.String(content),
		Visibility:  gitlab.Visibility(gitlab.VisibilityValue(config.Conf.GitLab.Visibility)),
	}

	ret, _, err := g.Client.Snippets.CreateSnippet(opt)
	if err != nil {
		return -1, errors.Wrap(err, "Failed to create GitLab Snippet")
	}
	return ret.ID, nil
}

func (g GitLabClient) updateSnippet(ctx context.Context, content string) (err error) {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Start()
	s.Suffix = " Updating GitLab Snippet..."
	defer s.Stop()

	opt := &gitlab.UpdateSnippetOptions{
		Title:       gitlab.String("pet-snippet"),
		FileName:    gitlab.String(config.Conf.GitLab.FileName),
		Description: gitlab.String("Snippet file generated by pet"),
		Content:     gitlab.String(content),
		Visibility:  gitlab.Visibility(gitlab.VisibilityValue(config.Conf.GitLab.Visibility)),
	}

	_, _, err = g.Client.Snippets.UpdateSnippet(g.ID, opt)
	if err != nil {
		return errors.Wrap(err, "Failed to update GitLab Snippet")
	}
	return nil
}
