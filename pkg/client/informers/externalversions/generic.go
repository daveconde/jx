// Code generated by informer-gen. DO NOT EDIT.

package externalversions

import (
	"fmt"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	cache "k8s.io/client-go/tools/cache"
)

// GenericInformer is type of SharedIndexInformer which will locate and delegate to other
// sharedInformers based on type
type GenericInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() cache.GenericLister
}

type genericInformer struct {
	informer cache.SharedIndexInformer
	resource schema.GroupResource
}

// Informer returns the SharedIndexInformer.
func (f *genericInformer) Informer() cache.SharedIndexInformer {
	return f.informer
}

// Lister returns the GenericLister.
func (f *genericInformer) Lister() cache.GenericLister {
	return cache.NewGenericLister(f.Informer().GetIndexer(), f.resource)
}

// ForResource gives generic access to a shared informer of the matching type
// TODO extend this to unknown resources with a client pool
func (f *sharedInformerFactory) ForResource(resource schema.GroupVersionResource) (GenericInformer, error) {
	switch resource {
	// Group=jenkins.io, Version=v1
	case v1.SchemeGroupVersion.WithResource("apps"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().Apps().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("buildpacks"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().BuildPacks().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("commitstatuses"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().CommitStatuses().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("environments"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().Environments().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("environmentrolebindings"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().EnvironmentRoleBindings().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("extensions"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().Extensions().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("facts"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().Facts().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("gitservices"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().GitServices().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("pipelineactivities"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().PipelineActivities().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("pipelinestructures"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().PipelineStructures().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("plugins"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().Plugins().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("releases"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().Releases().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("sourcerepositories"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().SourceRepositories().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("teams"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().Teams().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("users"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().Users().Informer()}, nil
	case v1.SchemeGroupVersion.WithResource("workflows"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Jenkins().V1().Workflows().Informer()}, nil

	}

	return nil, fmt.Errorf("no informer found for %v", resource)
}
