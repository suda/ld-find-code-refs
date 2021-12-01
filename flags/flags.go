package flags

import (
	"fmt"
	"os"

	"github.com/launchdarkly/ld-find-code-refs/internal/helpers"
	"github.com/launchdarkly/ld-find-code-refs/internal/ld"
	"github.com/launchdarkly/ld-find-code-refs/internal/log"
	"github.com/launchdarkly/ld-find-code-refs/internal/version"
	"github.com/launchdarkly/ld-find-code-refs/options"
)

const (
	minFlagKeyLen = 3 // Minimum flag key length helps reduce the number of false positives
)

func GetFlagKeys(opts options.Options, repoParams ld.RepoParams) map[string][]string {
	var projects []string
	flagKeys := make(map[string][]string)
	if len(opts.Projects) > 0 {
		for _, proj := range opts.Projects {
			projects = append(projects, proj.ProjectKey)
		}
	} else {
		projects = append(projects, opts.AccessToken)
	}

	for _, proj := range projects {
		ldApi := ld.InitApiClient(ld.ApiOptions{ApiKey: opts.AccessToken, BaseUri: opts.BaseUri, ProjKey: proj, UserAgent: "LDFindCodeRefs/" + version.Version})
		isDryRun := opts.DryRun

		ignoreServiceErrors := opts.IgnoreServiceErrors
		if !isDryRun {
			err := ldApi.MaybeUpsertCodeReferenceRepository(repoParams)
			if err != nil {
				helpers.FatalServiceError(err, ignoreServiceErrors)
			}
		}

		flags, err := getFlags(ldApi)
		if err != nil {
			helpers.FatalServiceError(fmt.Errorf("could not retrieve flag keys from LaunchDarkly: %w", err), ignoreServiceErrors)
		}

		filteredFlags, omittedFlags := filterShortFlagKeys(flags)
		if len(filteredFlags) == 0 {
			log.Info.Printf("no flag keys longer than the minimum flag key length (%v) were found for project: %s, exiting early",
				minFlagKeyLen, proj)
			os.Exit(0)
		} else if len(omittedFlags) > 0 {
			log.Warning.Printf("omitting %d flags with keys less than minimum (%d)", len(omittedFlags), minFlagKeyLen)
		}
		flagKeys[proj] = filteredFlags
	}

	return flagKeys
}

// Very short flag keys lead to many false positives when searching in code,
// so we filter them out.
func filterShortFlagKeys(flags []string) (filtered []string, omitted []string) {
	filteredFlags := []string{}
	omittedFlags := []string{}
	for _, flag := range flags {
		if len(flag) >= minFlagKeyLen {
			filteredFlags = append(filteredFlags, flag)
		} else {
			omittedFlags = append(omittedFlags, flag)
		}
	}
	return filteredFlags, omittedFlags
}

func getFlags(ldApi ld.ApiClient) ([]string, error) {
	flags, err := ldApi.GetFlagKeyList()
	if err != nil {
		return nil, err
	}
	return flags, nil
}
