package cmd

import (
	"context"
	"fmt"
	"gopkg.in/AlecAivazis/survey.v1/terminal"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/services"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/version"
	"github.com/pkg/errors"
	"gopkg.in/AlecAivazis/survey.v1"
	"gopkg.in/src-d/go-git.v4"
	gitconfig "gopkg.in/src-d/go-git.v4/config"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DefaultChartRepo default URL for charts repository
	DefaultChartRepo = "http://jenkins-x-chartmuseum:8080"
	// DefaultTillerNamesapce default namespace for helm tiller server
	DefaultTillerNamesapce = "kube-system"
	// DefaultTillerRole default cluster role for service account of helm tiller server
	DefaultTillerRole = "cluster-admin"
	// DefaultOnlyHelmClient indicates if only the client is initialized
	DefaultOnlyHelmClient = false
	// DefaultHelm3 indicates if helm 3 is used
	DefaultHelm3 = false
	// DefaultSkipTiller skips the tiller server initialization
	DefaultSkipTiller = false
	// DefaultGlobalTiller indicates if a global tiller server is used
	DefaultGlobalTiller = true
	// DefaultRemoteTiller indicates that a remote tiller server is used
	DefaultRemoteTiller = true
)

// InitHelmConfig configuration for helm initialization
type InitHelmConfig struct {
	Namespace       string
	OnlyHelmClient  bool
	Helm3           bool
	SkipTiller      bool
	GlobalTiller    bool
	TillerNamespace string
	TillerRole      string
}

// defaultInitHelmConfig builds the default configuration for init helm
func (o *CommonOptions) defaultInitHelmConfig() InitHelmConfig {
	return InitHelmConfig{
		Namespace:       kube.DefaultNamespace,
		OnlyHelmClient:  DefaultOnlyHelmClient,
		Helm3:           DefaultHelm3,
		SkipTiller:      DefaultSkipTiller,
		GlobalTiller:    DefaultGlobalTiller,
		TillerNamespace: DefaultTillerNamesapce,
		TillerRole:      DefaultTillerRole,
	}
}

// InitHelm initializes hlem client and server (tillter)
func (o *CommonOptions) InitHelm(config InitHelmConfig) error {
	var err error

	skipTiller := config.SkipTiller
	if config.Helm3 {
		log.Infof("Using %s\n", util.ColorInfo("helm3"))
		skipTiller = true
	} else {
		log.Infof("Using %s\n", util.ColorInfo("helm2"))
	}
	if !skipTiller {
		log.Infof("Configuring %s\n", util.ColorInfo("tiller"))
		client, curNs, err := o.KubeClientAndNamespace()
		if err != nil {
			return err
		}

		tillerNamespace := config.TillerNamespace
		serviceAccountName := "tiller"
		if config.GlobalTiller {
			if tillerNamespace == "" {
				return errors.New("tiller namespace is empty: glboal tiller requires a namesapce")
			}
		} else {
			if config.Namespace == "" {
				config.Namespace = curNs
			}
			if config.Namespace == "" {
				return errors.New("empty namespace")
			}
			tillerNamespace = config.Namespace
		}

		err = o.ensureServiceAccount(tillerNamespace, serviceAccountName)
		if err != nil {
			return err
		}

		if config.GlobalTiller {
			clusterRoleBindingName := serviceAccountName
			err = o.ensureClusterRoleBinding(clusterRoleBindingName, config.TillerRole, tillerNamespace, serviceAccountName)
			if err != nil {
				return err
			}
		} else {
			// lets create a tiller service account
			roleName := "tiller-manager"
			roleBindingName := "tiller-binding"

			_, err = client.RbacV1().Roles(tillerNamespace).Get(roleName, metav1.GetOptions{})
			if err != nil {
				// lets create a Role for tiller
				role := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{
						Name:      roleName,
						Namespace: tillerNamespace,
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{"", "extensions", "apps"},
							Resources: []string{"*"},
							Verbs:     []string{"*"},
						},
					},
				}
				_, err = client.RbacV1().Roles(tillerNamespace).Create(role)
				if err != nil {
					return fmt.Errorf("Failed to create Role %s in namespace %s: %s", roleName, tillerNamespace, err)
				}
				log.Infof("Created Role %s in namespace %s\n", util.ColorInfo(roleName), util.ColorInfo(tillerNamespace))
			}
			_, err = client.RbacV1().RoleBindings(tillerNamespace).Get(roleBindingName, metav1.GetOptions{})
			if err != nil {
				// lets create a RoleBinding for tiller
				roleBinding := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      roleBindingName,
						Namespace: tillerNamespace,
					},
					Subjects: []rbacv1.Subject{
						{
							Kind:      "ServiceAccount",
							Name:      serviceAccountName,
							Namespace: tillerNamespace,
						},
					},
					RoleRef: rbacv1.RoleRef{
						Kind:     "Role",
						Name:     roleName,
						APIGroup: "rbac.authorization.k8s.io",
					},
				}
				_, err = client.RbacV1().RoleBindings(tillerNamespace).Create(roleBinding)
				if err != nil {
					return fmt.Errorf("Failed to create RoleBinding %s in namespace %s: %s", roleName, tillerNamespace, err)
				}
				log.Infof("Created RoleBinding %s in namespace %s\n", util.ColorInfo(roleName), util.ColorInfo(tillerNamespace))
			}
		}

		running, err := kube.IsDeploymentRunning(client, "tiller-deploy", tillerNamespace)
		if running {
			log.Infof("Tiller Deployment is running in namespace %s\n", util.ColorInfo(tillerNamespace))
			return nil
		}
		if err == nil && !running {
			return fmt.Errorf("existing tiller deployment found but not running, please check the %s namespace and resolve any issues", tillerNamespace)
		}

		if !running {
			log.Infof("Initialising helm using ServiceAccount %s in namespace %s\n", util.ColorInfo(serviceAccountName), util.ColorInfo(tillerNamespace))

			err = o.Helm().Init(false, serviceAccountName, tillerNamespace, false)
			if err != nil {
				return err
			}
			err = kube.WaitForDeploymentToBeReady(client, "tiller-deploy", tillerNamespace, 10*time.Minute)
			if err != nil {
				return err
			}

			err = o.Helm().Init(false, serviceAccountName, tillerNamespace, true)
			if err != nil {
				return err
			}
		}

		log.Infof("Waiting for tiller-deploy to be ready in tiller namespace %s\n", tillerNamespace)
		err = kube.WaitForDeploymentToBeReady(client, "tiller-deploy", tillerNamespace, 10*time.Minute)
		if err != nil {
			return err
		}
	} else {
		log.Infof("Skipping %s\n", util.ColorInfo("tiller"))
	}

	if config.Helm3 {
		err = o.Helm().Init(false, "", "", false)
		if err != nil {
			return err
		}
	} else if config.OnlyHelmClient || config.SkipTiller {
		err = o.Helm().Init(true, "", "", false)
		if err != nil {
			return err
		}
	}

	err = o.Helm().AddRepo("jenkins-x", kube.DefaultChartMuseumURL, "", "")
	if err != nil {
		return err
	}
	log.Success("helm installed and configured")

	return nil
}

func (o *CommonOptions) registerLocalHelmRepo(repoName, ns string) error {
	if repoName == "" {
		repoName = kube.LocalHelmRepoName
	}
	// TODO we should use the auth package to keep a list of server login/pwds
	// TODO we have a chartmuseumAuth.yaml now but sure yet if that's the best thing to do
	username := "admin"
	password := "admin"

	// lets check if we have a local helm repository
	client, err := o.KubeClient()
	if err != nil {
		return errors.Wrap(err, "failed to create the kube client")
	}
	u, err := services.FindServiceURL(client, ns, kube.ServiceChartMuseum)
	if err != nil {
		return errors.Wrapf(err, "failed to find the service URL of the ChartMuseum")
	}
	u2, err := url.Parse(u)
	if err != nil {
		return errors.Wrap(err, "failed to parse the ChartMuseum URL")
	}
	if u2.User == nil {
		u2.User = url.UserPassword(username, password)
	}
	helmUrl := u2.String()
	// lets check if we already have the helm repo installed or if we need to add it or remove + add it
	remove := false
	repos, err := o.Helm().ListRepos()
	if err != nil {
		return errors.Wrap(err, "failed to list the repositories")
	}
	for repo, repoURL := range repos {
		if repo == repoName {
			if repoURL == helmUrl {
				return nil
			} else {
				remove = true
			}
		}
	}
	if remove {
		err = o.Helm().RemoveRepo(repoName)
		if err != nil {
			return errors.Wrapf(err, "failed to remove the repository '%s'", repoName)
		}
	}
	return o.Helm().AddRepo(repoName, helmUrl, "", "")
}

// addHelmRepoIfMissing adds the given helm repo if its not already added
func (o *CommonOptions) addHelmRepoIfMissing(helmUrl, repoName, username, password string) error {
	return o.addHelmBinaryRepoIfMissing(helmUrl, repoName, username, password)
}

func (o *CommonOptions) addHelmBinaryRepoIfMissing(helmUrl, repoName, username, password string) error {
	missing, err := o.Helm().IsRepoMissing(helmUrl)
	if err != nil {
		return errors.Wrapf(err, "failed to check if the repository with URL '%s' is missing", helmUrl)
	}
	if missing {
		log.Infof("Adding missing Helm repo: %s %s\n", util.ColorInfo(repoName), util.ColorInfo(helmUrl))
		err = o.Helm().AddRepo(repoName, helmUrl, username, password)
		if err == nil {
			log.Infof("Successfully added Helm repository %s.\n", repoName)
		}
		return errors.Wrapf(err, "failed to add the repository '%s' with URL '%s'", repoName, helmUrl)
	}
	return nil
}

// installChart installs the given chart
func (o *CommonOptions) installChart(releaseName string, chart string, version string, ns string, helmUpdate bool,
	setValues []string, valueFiles []string, repo string) error {
	return o.installChartOptions(helm.InstallChartOptions{ReleaseName: releaseName, Chart: chart, Version: version,
		Ns: ns, HelmUpdate: helmUpdate, SetValues: setValues, ValueFiles: valueFiles, Repository: repo})
}

// installChartAt installs the given chart
func (o *CommonOptions) installChartAt(dir string, releaseName string, chart string, version string, ns string,
	helmUpdate bool, setValues []string, valueFiles []string, repo string) error {
	return o.installChartOptions(helm.InstallChartOptions{Dir: dir, ReleaseName: releaseName, Chart: chart,
		Version: version, Ns: ns, HelmUpdate: helmUpdate, SetValues: setValues, ValueFiles: valueFiles, Repository: repo})
}

func (o *CommonOptions) installChartOptions(options helm.InstallChartOptions) error {
	client, err := o.KubeClient()
	if err != nil {
		return err
	}
	if options.VersionsDir == "" {
		options.VersionsDir, err = o.cloneJXVersionsRepo("")
	}
	return helm.InstallFromChartOptions(options, o.Helm(), client, defaultInstallTimeout)
}

// clones the jenkins-x versions repo to a local working dir
func (o *CommonOptions) cloneJXVersionsRepo(versionRepository string) (string, error) {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	configDir, err := util.ConfigDir()
	if err != nil {
		return "", fmt.Errorf("error determining config dir %v", err)
	}
	wrkDir := filepath.Join(configDir, "jenkins-x-versions")

	o.Debugf("Current configuration dir: %s\n", configDir)
	o.Debugf("versionRepository: %s\n", versionRepository)

	if versionRepository == "" {
		versionRepository = DefaultVersionsURL
	}

	// If the repo already exists let's try to fetch the latest version
	if exists, err := util.DirExists(wrkDir); err == nil && exists {
		repo, err := git.PlainOpen(wrkDir)
		if err != nil {
			log.Errorf("Error opening %s", wrkDir)
			return deleteAndReClone(wrkDir, versionRepository, o.Out)
		}
		remote, err := repo.Remote("origin")
		if err != nil {
			log.Errorf("Error getting remote origin")
			return deleteAndReClone(wrkDir, versionRepository, o.Out)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()
		err = remote.FetchContext(ctx, &git.FetchOptions{
			RefSpecs: []gitconfig.RefSpec{
				gitconfig.RefSpec("+refs/heads/master:refs/remotes/origin/master"),
			},
		})
		// The repository is up to date
		if err == git.NoErrAlreadyUpToDate {
			return wrkDir, nil
		} else if err != nil {
			log.Errorf("Error fetching latest from remote")
			return deleteAndReClone(wrkDir, versionRepository, o.Out)
		} else {
			pullLatest := false
			if o.BatchMode {
				pullLatest = true
			} else {
				confirm := &survey.Confirm{
					Message: "A local Jenkins X versions repository already exists, pull the latest?",
					Default: true,
				}
				survey.AskOne(confirm, &pullLatest, nil, surveyOpts)
				if err != nil {
					log.Errorf("Error confirming if we should pull latest, skipping %s\n", wrkDir)
				}
			}

			if pullLatest {
				w, err := repo.Worktree()
				if err == nil {
					err := w.Pull(&git.PullOptions{RemoteName: "origin"})
					if err != nil {
						return "", errors.Wrap(err, "pulling the latest")
					}
				}
			}
			return wrkDir, err
		}
	} else {
		return deleteAndReClone(wrkDir, versionRepository, o.Out)
	}
}

func deleteAndReClone(wrkDir string, versionRepository string, fw terminal.FileWriter) (string, error) {
	log.Info("Deleting and cloning the Jenkins X versions repo")
	err := deleteDirectory(wrkDir)
	if err != nil {
		return "", err
	}
	err = clone(wrkDir, versionRepository, fw)
	if err != nil {
		return "", err
	}
	return wrkDir, err
}

func clone(wrkDir string, versionRepository string, fw terminal.FileWriter) error {
	log.Infof("Cloning the Jenkins X versions repo to %s\n", wrkDir)
	_, err := git.PlainClone(wrkDir, false, &git.CloneOptions{
		URL:           versionRepository,
		ReferenceName: "refs/heads/master",
		SingleBranch:  true,
		Progress:      fw,
	})
	return err
}

func deleteDirectory(wrkDir string) error {
	log.Infof("Delete previous Jenkins X version repo from %s\n", wrkDir)
	// If it exists a this stage most likely its content is not consistent
	if exists, err := util.DirExists(wrkDir); err == nil && exists {
		err := util.DeleteDirContents(wrkDir)
		if err != nil {
			return errors.Wrapf(err, "cleaning the content of %q dir", wrkDir)
		}
	}
	return nil
}

// getVersionNumber returns the version number for the given kind and name or blank string if there is no locked version
func (o *CommonOptions) getVersionNumber(kind version.VersionKind, name string) (string, error) {
	versionsDir, err := o.cloneJXVersionsRepo("")
	if err != nil {
		return "", err
	}
	return version.LoadStableVersionNumber(versionsDir, kind, name)
}

// deleteChart deletes the given chart
func (o *CommonOptions) deleteChart(releaseName string, purge bool) error {
	_, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}

	return o.Helm().DeleteRelease(ns, releaseName, purge)
}

func (o *CommonOptions) FindHelmChart() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", errors.Wrap(err, "failed to get the current working directory")
	}
	o.Helm().SetCWD(dir)
	return o.Helm().FindChart()
}

func (o *CommonOptions) DiscoverAppName() (string, error) {
	answer := ""
	chartFile, err := o.FindHelmChart()
	if err != nil {
		return answer, err
	}
	if chartFile != "" {
		return helm.LoadChartName(chartFile)
	}

	gitInfo, err := o.Git().Info("")
	if err != nil {
		return answer, err
	}

	if gitInfo == nil {
		return answer, fmt.Errorf("no git info found to discover app name from")
	}
	answer = gitInfo.Name

	if answer == "" {
	}
	return answer, nil
}

func (o *CommonOptions) isHelmRepoMissing(helmUrlString string) (bool, error) {
	return o.Helm().IsRepoMissing(helmUrlString)
}

func (o *CommonOptions) addChartRepos(dir string, helmBinary string, chartRepos map[string]string) error {
	installedChartRepos, err := o.getInstalledChartRepos(helmBinary)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve the install charts")
	}
	repoCounter := len(installedChartRepos)
	if chartRepos != nil {
		for name, url := range chartRepos {
			if !util.StringMapHasValue(installedChartRepos, url) {
				repoCounter++
				err = o.addHelmBinaryRepoIfMissing(url, name, "", "")
				if err != nil {
					return errors.Wrapf(err, "failed to add the Helm repository with name '%s' and URL '%s'", name, url)
				}
			}
		}
	}

	reqfile := filepath.Join(dir, "requirements.yaml")
	exists, err := util.FileExists(reqfile)
	if err != nil {
		return errors.Wrapf(err, "requirements.yaml file not found in the chart directory '%s'", dir)
	}
	if exists {
		requirements, err := helm.LoadRequirementsFile(reqfile)
		if err != nil {
			return errors.Wrap(err, "failed to load the Helm requirements file")
		}
		if requirements != nil {
			for _, dep := range requirements.Dependencies {
				repo := dep.Repository
				if repo != "" && !util.StringMapHasValue(installedChartRepos, repo) && repo != DefaultChartRepo && !strings.HasPrefix(repo, "file:") && !strings.HasPrefix(repo, "alias:") {
					repoCounter++
					// TODO we could provide some mechanism to customise the names of repos somehow?
					err = o.addHelmBinaryRepoIfMissing(repo, "repo"+strconv.Itoa(repoCounter), "", "")
					if err != nil {
						return errors.Wrapf(err, "failed to add Helm repository '%s'", repo)
					}
				}
			}
		}
	}
	return nil
}

func (o *CommonOptions) getInstalledChartRepos(helmBinary string) (map[string]string, error) {
	return o.Helm().ListRepos()
}

func (o *CommonOptions) helmInit(dir string) error {
	o.Helm().SetCWD(dir)
	if o.Helm().HelmBinary() == "helm" {
		// need to check the tiller settings at this point
		_, noTiller, helmTemplate, err := o.TeamHelmBin()
		if err != nil {
			return errors.Wrap(err, "failed to access team settings")
		}

		if noTiller || helmTemplate {
			return o.Helm().Init(true, "", "", false)
		} else {
			return o.Helm().Init(false, "", "", true)
		}
	} else {
		return o.Helm().Init(false, "", "", false)
	}
}

func (o *CommonOptions) helmInitDependency(dir string, chartRepos map[string]string) (string, error) {
	o.Helm().SetCWD(dir)
	err := o.Helm().RemoveRequirementsLock()
	if err != nil {
		return o.Helm().HelmBinary(),
			errors.Wrapf(err, "failed to remove requirements.lock file from chart '%s'", dir)
	}

	if o.Helm().HelmBinary() == "helm" {
		// need to check the tiller settings at this point
		_, noTiller, helmTemplate, err := o.TeamHelmBin()
		if err != nil {
			return o.Helm().HelmBinary(),
				errors.Wrap(err, "failed to access team settings")
		}

		if noTiller || helmTemplate {
			err = o.Helm().Init(true, "", "", false)
		} else {
			err = o.Helm().Init(false, "", "", true)
		}
	} else {
		err = o.Helm().Init(false, "", "", false)
	}

	if err != nil {
		return o.Helm().HelmBinary(),
			errors.Wrap(err, "failed to initialize Helm")
	}
	err = o.addChartRepos(dir, o.Helm().HelmBinary(), chartRepos)
	if err != nil {
		return o.Helm().HelmBinary(),
			errors.Wrap(err, "failed to add chart repositories")
	}

	return o.Helm().HelmBinary(), nil
}

func (o *CommonOptions) helmInitDependencyBuild(dir string, chartRepos map[string]string) (string, error) {
	helmBin, err := o.helmInitDependency(dir, chartRepos)
	if err != nil {
		return helmBin, err
	}
	// TODO due to this issue: https://github.com/kubernetes/helm/issues/4230
	// lets stick with helm2 for this step
	//
	helmBinary := o.Helm().HelmBinary()
	o.Helm().SetHelmBinary("helm")
	o.Helm().SetCWD(dir)
	err = o.Helm().BuildDependency()
	if err != nil {
		return helmBinary, errors.Wrapf(err, "failed to build the dependencies of chart '%s'", dir)
	}

	o.Helm().SetHelmBinary(helmBinary)
	_, err = o.Helm().Lint()
	if err != nil {
		return helmBinary, errors.Wrapf(err, "failed to lint the chart '%s'", dir)
	}
	return helmBinary, nil
}

func (o *CommonOptions) helmInitRecursiveDependencyBuild(dir string, chartRepos map[string]string) error {
	_, err := o.helmInitDependency(dir, chartRepos)
	if err != nil {
		return errors.Wrap(err, "initializing Helm")
	}

	helmBinary := o.Helm().HelmBinary()
	o.Helm().SetHelmBinary("helm")
	o.Helm().SetCWD(dir)
	err = o.Helm().BuildDependency()
	if err != nil {
		return errors.Wrapf(err, "failed to build the dependencies of chart '%s'", dir)
	}

	reqFilePath := filepath.Join(dir, "requirements.yaml")
	reqs, err := helm.LoadRequirementsFile(reqFilePath)
	if err != nil {
		return errors.Wrap(err, "loading the requirements file")
	}

	type chartDep struct {
		path string
		deps []*helm.Dependency
	}

	baseChartPath := filepath.Join(dir, "charts")
	depQueue := []chartDep{{
		path: baseChartPath,
		deps: reqs.Dependencies,
	}}

	for {
		if len(depQueue) == 0 {
			break
		}
		currChartDep := depQueue[0]
		depQueue = depQueue[1:]
		for _, dep := range currChartDep.deps {
			chartArchive := filepath.Join(currChartDep.path, fmt.Sprintf("%s-%s.tgz", dep.Name, dep.Version))
			chartPath := filepath.Join(currChartDep.path, dep.Name)
			err := os.MkdirAll(chartPath, os.ModePerm)
			if err != nil {
				return errors.Wrap(err, "creating directory")
			}
			err = util.UnTargz(chartArchive, chartPath, []string{})
			if err != nil {
				return errors.Wrap(err, "extracting chart")
			}
			o.Helm().SetCWD(chartPath)
			err = o.Helm().BuildDependency()
			if err != nil {
				return errors.Wrap(err, "building Helm dependency")
			}
			chartReqFile := filepath.Join(chartPath, "requirements.yaml")
			reqs, err := helm.LoadRequirementsFile(chartReqFile)
			if err != nil {
				return errors.Wrap(err, "loading the requirements file")
			}
			if len(reqs.Dependencies) > 0 {
				depQueue = append(depQueue, chartDep{
					path: filepath.Join(chartPath, "charts"),
					deps: reqs.Dependencies,
				})
			}
		}
	}

	o.Helm().SetHelmBinary(helmBinary)
	_, err = o.Helm().Lint()
	if err != nil {
		return errors.Wrapf(err, "linting the chart '%s'", dir)
	}

	return nil
}

func (o *CommonOptions) defaultReleaseCharts() map[string]string {
	releasesURL := o.releaseChartMuseumUrl()
	answer := map[string]string{
		"jenkins-x": kube.DefaultChartMuseumURL,
	}
	if releasesURL != "" {
		answer["releases"] = releasesURL
	}
	return answer
}

func (o *CommonOptions) releaseChartMuseumUrl() string {
	chartRepo := os.Getenv("CHART_REPOSITORY")
	if chartRepo == "" {
		if o.IsInCDPipeline() {
			chartRepo = DefaultChartRepo
			log.Warnf("No $CHART_REPOSITORY defined so using the default value of: %s\n", DefaultChartRepo)
		} else {
			return ""
		}
	}
	return chartRepo
}

func (o *CommonOptions) ensureHelm() error {
	_, err := o.Helm().Version(false)
	if err == nil {
		return nil
	}
	err = o.installHelm()
	if err != nil {
		return errors.Wrap(err, "failed to install Helm")
	}
	cfg := o.defaultInitHelmConfig()
	err = o.InitHelm(cfg)
	if err != nil {
		return errors.Wrapf(err, "initializing helm with config: %v", cfg)
	}
	return nil
}
