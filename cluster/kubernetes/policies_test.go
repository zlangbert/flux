package kubernetes

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/flux"
	kresource "github.com/weaveworks/flux/cluster/kubernetes/resource"
	"github.com/weaveworks/flux/policy"
)

type constNamespacer string

func (ns constNamespacer) EffectiveNamespace(manifest kresource.KubeManifest, _ ResourceScopes) (string, error) {
	return string(ns), nil
}

func TestUpdatePolicies(t *testing.T) {
	for _, c := range []struct {
		name    string
		in, out []string
		update  policy.Update
		wantErr bool
	}{
		{
			name: "adding annotation with others existing",
			in:   []string{"prometheus.io.scrape", "'false'"},
			out:  []string{"prometheus.io.scrape", "'false'", "flux.weave.works/automated", "'true'"},
			update: policy.Update{
				Add: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "adding annotation when already has annotation",
			in:   []string{"flux.weave.works/automated", "'true'"},
			out:  []string{"flux.weave.works/automated", "'true'"},
			update: policy.Update{
				Add: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "adding annotation when already has annotation and others",
			in:   []string{"flux.weave.works/automated", "'true'", "prometheus.io.scrape", "'false'"},
			out:  []string{"flux.weave.works/automated", "'true'", "prometheus.io.scrape", "'false'"},
			update: policy.Update{
				Add: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "adding first annotation",
			in:   nil,
			out:  []string{"flux.weave.works/automated", "'true'"},
			update: policy.Update{
				Add: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "add and remove different annotations at the same time",
			in:   []string{"flux.weave.works/automated", "'true'", "prometheus.io.scrape", "'false'"},
			out:  []string{"prometheus.io.scrape", "'false'", "flux.weave.works/locked", "'true'"},
			update: policy.Update{
				Add:    policy.Set{policy.Locked: "true"},
				Remove: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "remove overrides add for same key",
			in:   nil,
			out:  nil,
			update: policy.Update{
				Add:    policy.Set{policy.Locked: "true"},
				Remove: policy.Set{policy.Locked: "true"},
			},
		},
		{
			name: "remove annotation with others existing",
			in:   []string{"flux.weave.works/automated", "true", "prometheus.io.scrape", "false"},
			out:  []string{"prometheus.io.scrape", "false"},
			update: policy.Update{
				Remove: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "remove last annotation",
			in:   []string{"flux.weave.works/automated", "true"},
			out:  nil,
			update: policy.Update{
				Remove: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "remove annotation with no annotations",
			in:   nil,
			out:  nil,
			update: policy.Update{
				Remove: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "remove annotation with only others",
			in:   []string{"prometheus.io.scrape", "false"},
			out:  []string{"prometheus.io.scrape", "false"},
			update: policy.Update{
				Remove: policy.Set{policy.Automated: "true"},
			},
		},
		{
			name: "multiline",
			in:   []string{"flux.weave.works/locked_msg", "|-\n      first\n      second"},
			out:  nil,
			update: policy.Update{
				Remove: policy.Set{policy.LockedMsg: "foo"},
			},
		},
		{
			name: "multiline with empty line",
			in:   []string{"flux.weave.works/locked_msg", "|-\n      first\n\n      third"},
			out:  nil,
			update: policy.Update{
				Remove: policy.Set{policy.LockedMsg: "foo"},
			},
		},
		{
			name: "add tag policy",
			in:   nil,
			out:  []string{"flux.weave.works/tag.nginx", "glob:*"},
			update: policy.Update{
				Add: policy.Set{policy.TagPrefix("nginx"): "glob:*"},
			},
		},
		{
			name: "add non-glob tag policy",
			in:   nil,
			out:  []string{"flux.weave.works/tag.nginx", "foo"},
			update: policy.Update{
				Add: policy.Set{policy.TagPrefix("nginx"): "foo"},
			},
		},
		{
			name: "add semver tag policy",
			in:   nil,
			out:  []string{"flux.weave.works/tag.nginx", "semver:*"},
			update: policy.Update{
				Add: policy.Set{policy.TagPrefix("nginx"): "semver:*"},
			},
		},
		{
			name: "add invalid semver tag policy",
			in:   nil,
			out:  []string{"flux.weave.works/tag.nginx", "semver:*"},
			update: policy.Update{
				Add: policy.Set{policy.TagPrefix("nginx"): "semver:invalid"},
			},
			wantErr: true,
		},
		{
			name: "add regexp tag policy",
			in:   nil,
			out:  []string{"flux.weave.works/tag.nginx", "regexp:(.*?)"},
			update: policy.Update{
				Add: policy.Set{policy.TagPrefix("nginx"): "regexp:(.*?)"},
			},
		},
		{
			name: "add invalid regexp tag policy",
			in:   nil,
			out:  []string{"flux.weave.works/tag.nginx", "regexp:(.*?)"},
			update: policy.Update{
				Add: policy.Set{policy.TagPrefix("nginx"): "regexp:*"},
			},
			wantErr: true,
		},
		{
			name: "set tag to all containers",
			in:   nil,
			out:  []string{"flux.weave.works/tag.nginx", "semver:*"},
			update: policy.Update{
				Add: policy.Set{policy.TagAll: "semver:*"},
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			caseIn := templToString(t, annotationsTemplate, c.in)
			caseOut := templToString(t, annotationsTemplate, c.out)
			resourceID := flux.MustParseResourceID("default:deployment/nginx")
			out, err := (&Manifests{constNamespacer("default")}).UpdatePolicies([]byte(caseIn), resourceID, c.update)
			assert.Equal(t, c.wantErr, err != nil, "unexpected error value: %s", err)
			if !c.wantErr {
				assert.Equal(t, string(out), caseOut)
			}
		})
	}
}

func TestUpdatePolicies_invalidTagPattern(t *testing.T) {
	resourceID := flux.MustParseResourceID("default:deployment/nginx")
	update := policy.Update{
		Add: policy.Set{policy.TagPrefix("nginx"): "semver:invalid"},
	}
	_, err := (&Manifests{}).UpdatePolicies(nil, resourceID, update)
	assert.Error(t, err)
}

var annotationsTemplate = template.Must(template.New("").Parse(`---
apiVersion: extensions/v1beta1
kind: Deployment
metadata: # comment really close to the war zone
  name: nginx{{with .}}
  annotations:{{range .}}
    {{index . 0}}: {{printf "%s" (index . 1)}}{{end}}{{end}}
spec:
  replicas: 1
  template:
    metadata: # comment2
      labels:
        name: nginx
    spec:
      containers:
      - image: nginx  # These keys are purposefully un-sorted.
        name: nginx   # And these comments are testing comments.
        ports:
        - containerPort: 80
`))

func templToString(t *testing.T, templ *template.Template, data []string) string {
	var pairs [][]string
	for i := 0; i < len(data); i += 2 {
		pairs = append(pairs, []string{data[i], data[i+1]})
	}
	out := &bytes.Buffer{}
	err := templ.Execute(out, pairs)
	if err != nil {
		t.Fatal(err)
	}
	return out.String()
}