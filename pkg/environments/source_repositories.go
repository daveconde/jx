package environments

import (
	"fmt"
	"github.com/ghodss/yaml"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"
	"strings"
)

func WriteSourceRepoTemplates(dir string, srList *v1.SourceRepositoryList) ([]string, error) {
	outDir := filepath.Join(dir, "repositories", "templates")
	exists, err := util.DirExists(outDir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to check if output dir exists %s", outDir)
	}
	if !exists {
		return nil, fmt.Errorf("output dir %s does not exist", outDir)
	}

	for _, sr := range srList.Items {
		labels := sr.Labels
		if labels != nil {
			if strings.ToLower(labels[kube.LabelGitSync]) == "false" {
				continue
			}
		}
		sr.ObjectMeta = emptyObjectMeta(&sr.ObjectMeta)

		data, err := yaml.Marshal(&sr)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to marshal SourceRepository %s to YAML", sr.Name)
		}

		fileName := filepath.Join(outDir, sr.Name+".yaml")
		err = ioutil.WriteFile(fileName, data, util.DefaultWritePermissions)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to write file %s for SourceRepository %s to YAML", fileName, sr.Name)
		}
	}
	return nil, nil
}

// emptyObjectMeta lets return a clean ObjectMeta without any cluster or transient specific values
func emptyObjectMeta(md *metav1.ObjectMeta) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name: md.Name,
	}
}
