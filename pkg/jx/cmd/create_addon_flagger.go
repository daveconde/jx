package cmd

import (
	"fmt"
	"github.com/jenkins-x/jx/pkg/log"
	"io"
	"strings"

	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/jx/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	"gopkg.in/AlecAivazis/survey.v1/terminal"
	"k8s.io/apimachinery/pkg/types"
)

const (
	defaultFlaggerNamespace             = "istio-system"
	defaultFlaggerReleaseName           = kube.DefaultFlaggerReleaseName
	defaultFlaggerVersion               = ""
	defaultFlaggerRepo                  = "https://flagger.app"
	optionGrafanaChart                  = "grafana-chart"
	defaultFlaggerProductionEnvironment = "production"
)

var (
	createAddonFlaggerLong = templates.LongDesc(`
		Creates the Flagger addon
`)

	createAddonFlaggerExample = templates.Examples(`
		# Create the Flagger addon
		jx create addon flagger
	`)
)

type CreateAddonFlaggerOptions struct {
	CreateAddonOptions
	Chart                 string
	GrafanaChart          string
	ProductionEnvironment string
}

func NewCmdCreateAddonFlagger(f Factory, in terminal.FileReader, out terminal.FileWriter, errOut io.Writer) *cobra.Command {
	options := &CreateAddonFlaggerOptions{
		CreateAddonOptions: CreateAddonOptions{
			CreateOptions: CreateOptions{
				CommonOptions: CommonOptions{
					Factory: f,
					In:      in,
					Out:     out,
					Err:     errOut,
				},
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "flagger",
		Short:   "Create the Flagger addon for Canary deployments",
		Long:    createAddonFlaggerLong,
		Example: createAddonFlaggerExample,
		Run: func(cmd *cobra.Command, args []string) {
			err := options.Run()
			CheckErr(err)
		},
	}

	options.addCommonFlags(cmd)
	options.addFlags(cmd, defaultFlaggerNamespace, defaultFlaggerReleaseName, defaultFlaggerVersion)

	cmd.Flags().StringVarP(&options.Chart, optionChart, "c", kube.ChartFlagger, "The name of the chart to use")
	cmd.Flags().StringVarP(&options.GrafanaChart, optionGrafanaChart, "", kube.ChartFlaggerGrafana, "The name of the Flagger Grafana chart to use")
	cmd.Flags().StringVarP(&options.ProductionEnvironment, "environment", "e", defaultFlaggerProductionEnvironment, "The name of the production environment where Istio will be enabled")
	return cmd
}

// Create the addon
func (o *CreateAddonFlaggerOptions) Run() error {
	if o.ReleaseName == "" {
		return util.MissingOption(optionRelease)
	}
	if o.Chart == "" {
		return util.MissingOption(optionChart)
	}
	if o.GrafanaChart == "" {
		return util.MissingOption(optionGrafanaChart)
	}
	err := o.ensureHelm()
	if err != nil {
		return errors.Wrap(err, "failed to ensure that Helm is present")
	}

	values := []string{}
	setValues := strings.Split(o.SetValues, ",")
	values = append(values, setValues...)
	err = o.addHelmRepoIfMissing(defaultFlaggerRepo, "flagger", "", "")
	if err != nil {
		return errors.Wrap(err, "Flagger deployment failed")
	}
	err = o.installChart(o.ReleaseName, o.Chart, o.Version, o.Namespace, true, values, nil, "")
	if err != nil {
		return errors.Wrap(err, "Flagger deployment failed")
	}
	err = o.installChart(o.ReleaseName+"-grafana", o.GrafanaChart, o.Version, o.Namespace, true, values, nil, "")
	if err != nil {
		return errors.Wrap(err, "Flagger Grafana deployment failed")
	}

	// Enable Istio in production namespace
	if o.ProductionEnvironment != "" {
		client, err := o.KubeClient()
		if err != nil {
			return errors.Wrap(err, "error enabling Istio in production namespace")
		}
		var ns string
		ns, err = o.findEnvironmentNamespace(o.ProductionEnvironment)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("error enabling Istio for environment %s", o.ProductionEnvironment))
		}
		log.Infof("Enabling Istio in namespace %s\n", ns)
		patch := []byte(`{"metadata":{"labels":{"istio-injection":"enabled"}}}`)
		_, err = client.CoreV1().Namespaces().Patch(ns, types.MergePatchType, patch)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("error enabling Istio in namespace %s", ns))
		}
	}
	return nil
}
