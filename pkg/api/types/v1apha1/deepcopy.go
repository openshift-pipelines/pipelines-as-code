package v1alpha1

import "k8s.io/apimachinery/pkg/runtime"

// DeepCopyInto copies all properties of this object into another object of the
// same type that is provided as a pointer.
func (in *Repository) DeepCopyInto(out *Repository) {
	out.TypeMeta = in.TypeMeta
	out.ObjectMeta = in.ObjectMeta
	out.Spec = RepositorySpec{
		Namespace: in.Spec.Namespace,
		URL:       in.Spec.URL,
		EventType: in.Spec.EventType,
		Branch:    in.Spec.Branch,
	}
}

// DeepCopyObject returns a generically typed copy of an object
func (in *Repository) DeepCopyObject() runtime.Object {
	out := Repository{}
	in.DeepCopyInto(&out)

	return &out
}

// DeepCopyObject returns a generically typed copy of an object
func (in *RepositoryList) DeepCopyObject() runtime.Object {
	out := RepositoryList{}
	out.TypeMeta = in.TypeMeta
	out.ListMeta = in.ListMeta

	if in.Items != nil {
		out.Items = make([]Repository, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}

	return &out
}
