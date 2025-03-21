/*
Copyright Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// RepositoryLister helps list Repositories.
// All objects returned here must be treated as read-only.
type RepositoryLister interface {
	// List lists all Repositories in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.Repository, err error)
	// Repositories returns an object that can list and get Repositories.
	Repositories(namespace string) RepositoryNamespaceLister
	RepositoryListerExpansion
}

// repositoryLister implements the RepositoryLister interface.
type repositoryLister struct {
	indexer cache.Indexer
}

// NewRepositoryLister returns a new RepositoryLister.
func NewRepositoryLister(indexer cache.Indexer) RepositoryLister {
	return &repositoryLister{indexer: indexer}
}

// List lists all Repositories in the indexer.
func (s *repositoryLister) List(selector labels.Selector) (ret []*v1alpha1.Repository, err error) {
	err = cache.ListAll(s.indexer, selector, func(m any) {
		ret = append(ret, m.(*v1alpha1.Repository))
	})
	return ret, err
}

// Repositories returns an object that can list and get Repositories.
func (s *repositoryLister) Repositories(namespace string) RepositoryNamespaceLister {
	return repositoryNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// RepositoryNamespaceLister helps list and get Repositories.
// All objects returned here must be treated as read-only.
type RepositoryNamespaceLister interface {
	// List lists all Repositories in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.Repository, err error)
	// Get retrieves the Repository from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.Repository, error)
	RepositoryNamespaceListerExpansion
}

// repositoryNamespaceLister implements the RepositoryNamespaceLister
// interface.
type repositoryNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all Repositories in the indexer for a given namespace.
func (s repositoryNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.Repository, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m any) {
		ret = append(ret, m.(*v1alpha1.Repository))
	})
	return ret, err
}

// Get retrieves the Repository from the indexer for a given namespace and name.
func (s repositoryNamespaceLister) Get(name string) (*v1alpha1.Repository, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("repository"), name)
	}
	return obj.(*v1alpha1.Repository), nil
}
