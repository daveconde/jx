package cmd_test

import (
	"github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/jenkins-x/jx/pkg/jx/cmd/cmd_test_helpers"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/petergtz/pegomock"
	"github.com/satori/go.uuid"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"testing"

	"github.com/jenkins-x/jx/pkg/jx/cmd"
	"github.com/stretchr/testify/assert"
)

func TestGetApp(t *testing.T) {
	name := uuid.NewV4().String()
	version := "0.0.1"
	namespace := "jx-testing"
	pegomock.RegisterMockTestingT(t)
	testOptions := cmd_test_helpers.CreateAppTestOptions(false, t)

	defer func() {
		err := testOptions.Cleanup()
		assert.NoError(t, err)
	}()

	_, _, _, err := testOptions.AddApp()
	assert.NoError(t, err)
	helm_test.StubFetchChart(name, version, kube.DefaultChartMuseumURL, &chart.Chart{
		Metadata: &chart.Metadata{
			Name:        name,
			Version:     version,
			Description: "My test chart description",
		},
	}, testOptions.MockHelmer)

	o := &cmd.AddAppOptions{
		AddOptions: cmd.AddOptions{
			CommonOptions: *testOptions.CommonOptions,
		},
		Version:              version,
		Repo:                 kube.DefaultChartMuseumURL,
		GitOps:               false,
		DevEnv:               testOptions.DevEnv,
		HelmUpdate:           true, // Flag default when run on CLI
		ConfigureGitCallback: testOptions.ConfigureGitFn,
		Namespace:            namespace,
	}
	o.Args = []string{name}
	err = o.Run()
	assert.NoError(t, err)

	getAppOptions := &cmd.GetAppsOptions{
		GetOptions: cmd.GetOptions{
			CommonOptions: *testOptions.CommonOptions,
		},
		Namespace: namespace,
	}
	console := tests.NewTerminal(t)
	getAppOptions.CommonOptions.In = console.In
	getAppOptions.CommonOptions.Out = console.Out
	getAppOptions.CommonOptions.Err = console.Err

	//getAppOptions.Args = []string{name}

	err = getAppOptions.Run()
	assert.NoError(t, err)
}
