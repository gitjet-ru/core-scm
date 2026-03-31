package repository

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	repo_model "github.com/gitjet-ru/core-scm/models/repo"
	user_model "github.com/gitjet-ru/core-scm/models/user"
	"github.com/gitjet-ru/core-scm/modules/git/gitcmd"
	"github.com/gitjet-ru/core-scm/modules/gitrepo"
	"github.com/gitjet-ru/core-scm/modules/log"
	"github.com/gitjet-ru/core-scm/modules/options"
	repo_module "github.com/gitjet-ru/core-scm/modules/repository"
	"github.com/gitjet-ru/core-scm/modules/setting"
	"github.com/gitjet-ru/core-scm/modules/templates/vars"
)

func isRemoteCreateFlowEnabled() bool {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("GIT_STORAGE_BACKEND")))
	return mode == "remote" || mode == "shadow"
}

func initRepoCommitRemote(ctx context.Context, repo *repo_model.Repository, u *user_model.User, opts CreateRepoOptions) error {
	// Create flow may have duplicate/canceled browser requests; commit bootstrap should
	// complete independently of client disconnect once initialization has started.
	ctx = context.WithoutCancel(ctx)
	files, err := renderInitialFilesForRemote(ctx, repo, opts)
	if err != nil {
		return err
	}
	defaultBranch := opts.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = setting.Repository.DefaultBranch
	}

	commitTS := time.Now().Unix()
	sig := u.NewGitSig()
	commitEnv := append(os.Environ(),
		"GIT_AUTHOR_NAME="+sig.Name,
		"GIT_AUTHOR_EMAIL="+sig.Email,
		"GIT_AUTHOR_DATE="+time.Unix(commitTS, 0).Format(time.RFC3339),
		"GIT_COMMITTER_NAME="+sig.Name,
		"GIT_COMMITTER_EMAIL="+sig.Email,
		"GIT_COMMITTER_DATE="+time.Unix(commitTS, 0).Format(time.RFC3339),
	)
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var script bytes.Buffer
	markID := 1
	for _, rel := range paths {
		content := files[rel]
		script.WriteString("blob\n")
		script.WriteString(fmt.Sprintf("mark :%d\n", markID))
		script.WriteString(fmt.Sprintf("data %d\n", len(content)))
		script.Write(content)
		script.WriteByte('\n')
		markID++
	}

	script.WriteString("commit refs/heads/" + defaultBranch + "\n")
	script.WriteString(fmt.Sprintf("author %s <%s> %d +0000\n", sig.Name, sig.Email, commitTS))
	script.WriteString(fmt.Sprintf("committer %s <%s> %d +0000\n", sig.Name, sig.Email, commitTS))
	script.WriteString("data 15\nInitial commit\n")
	for i, rel := range paths {
		script.WriteString(fmt.Sprintf("M 100644 :%d %s\n", i+1, rel))
	}
	if _, _, runErr := gitrepo.RunCmdString(ctx, repo, gitcmd.NewCommand("fast-import", "--date-format=raw").WithStdinBytes(script.Bytes()).WithEnv(commitEnv)); runErr != nil {
		return fmt.Errorf("git fast-import: %w", runErr)
	}
	return nil
}

func renderInitialFilesForRemote(ctx context.Context, repo *repo_model.Repository, opts CreateRepoOptions) (map[string][]byte, error) {
	files := map[string][]byte{}

	data, err := options.Readme(opts.Readme)
	if err != nil {
		return nil, fmt.Errorf("GetRepoInitFile[%s]: %w", opts.Readme, err)
	}
	cloneLink := repo.CloneLink(ctx, nil)
	match := map[string]string{
		"Name":           repo.Name,
		"Description":    repo.Description,
		"CloneURL.SSH":   cloneLink.SSH,
		"CloneURL.HTTPS": cloneLink.HTTPS,
		"OwnerName":      repo.OwnerName,
	}
	res, err := vars.Expand(string(data), match)
	if err != nil {
		log.Error("unable to expand template vars for repo README: %s, err: %v", opts.Readme, err)
	}
	files["README.md"] = []byte(res)

	if len(opts.Gitignores) > 0 {
		var buf bytes.Buffer
		names := strings.SplitSeq(opts.Gitignores, ",")
		for name := range names {
			data, err = options.Gitignore(name)
			if err != nil {
				return nil, fmt.Errorf("GetRepoInitFile[%s]: %w", name, err)
			}
			buf.WriteString("# ---> " + name + "\n")
			buf.Write(data)
			buf.WriteString("\n")
		}
		if buf.Len() > 0 {
			files[".gitignore"] = buf.Bytes()
		}
	}

	if len(opts.License) > 0 {
		data, err = repo_module.GetLicense(opts.License, &repo_module.LicenseValues{
			Owner: repo.OwnerName,
			Email: repo.Owner.NewGitSig().Email,
			Repo:  repo.Name,
			Year:  time.Now().Format("2006"),
		})
		if err != nil {
			return nil, fmt.Errorf("getLicense[%s]: %w", opts.License, err)
		}
		files["LICENSE"] = data
	}
	return files, nil
}
