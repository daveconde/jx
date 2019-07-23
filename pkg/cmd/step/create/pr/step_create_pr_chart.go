package pr

import (
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	createPullRequestChartLong = templates.LongDesc(`
		Creates a Pull Request on a git repository updating the requirements.yaml and values.yaml with the new version
`)

	createPullRequestChartExample = templates.Examples(`
					`)
)

// StepCreatePullRequestChartsOptions contains the command line flags
type StepCreatePullRequestChartsOptions struct {
	StepCreatePrOptions

	Names []string
}

// NewCmdStepCreatePullRequestChart Creates a new Command object
func NewCmdStepCreatePullRequestChart(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepCreatePullRequestChartsOptions{
		StepCreatePrOptions: StepCreatePrOptions{
			StepCreateOptions: opts.StepCreateOptions{
				StepOptions: opts.StepOptions{
					CommonOptions: commonOpts,
				},
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "chart",
		Short:   "Creates a Pull Request on a git repository updating the Chart",
		Long:    createPullRequestChartLong,
		Example: createPullRequestChartExample,
		Aliases: []string{"version pullrequest"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	AddStepCreatePrFlags(cmd, &options.StepCreatePrOptions)
	cmd.Flags().StringArrayVarP(&options.Names, "name", "n", make([]string, 0), "The name of the property to update")
	return cmd
}

// ValidateChartsOptions validates the common options for chart pr steps
func (o *StepCreatePullRequestChartsOptions) ValidateChartsOptions() error {
	if err := o.ValidateOptions(); err != nil {
		return errors.WithStack(err)
	}
	if len(o.Names) == 0 {
		return util.MissingOption("name")
	}
	if o.SrcGitURL == "" {
		log.Logger().Warnf("srcRepo is not provided so generated PR will not be correctly linked in release notesPR")
	}

	return nil
}

// Run implements this command
func (o *StepCreatePullRequestChartsOptions) Run() error {
	if err := o.ValidateChartsOptions(); err != nil {
		return errors.WithStack(err)
	}
	err := o.CreatePullRequest("chart",
		func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
			return helm.UpdateVersions(o.Version, dir, o.Names)
		})
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}
