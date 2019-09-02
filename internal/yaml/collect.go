package yaml

import (
	"io"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"gopkg.in/yaml.v2"
	"strings"
)


// TypeMeta is a pun of v1.TypeMeta with yaml annotations.
// The alternative is to re-invent a bunch of the JSON->YAML transformations which all seem to be typed.
type TypeMeta struct {
	Kind string
	APIVersion string `yaml:"apiVersion"`
}

// GenericObject is a copy of v1.PartialObjectMetadata
type GenericObject struct {
	TypeMeta `yaml:",inline"`
	v1.ObjectMeta `yaml:"metadata"`
}

// GetConfigs reads a set of k8s PartialObjectMetadata from a set of
// yaml files in the specified directory.
func GetConfigs(dir string, fileSuffix string) ([]*v1.PartialObjectMetadata, error) {
	ret := []*v1.PartialObjectMetadata{}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name()[0] == '.' && info.Name() != "." {
			if info.IsDir() { // Don't traverse into "." directories.
				return filepath.SkipDir
			}
			return nil // Otherwise, just skip the file
		}
		if info.IsDir() || info.Size() <= 0 {
			// Keep going (including into subdirs), but don't need to open this and process.
			return nil
		}
		if info.Name()[0] != '.' && strings.HasSuffix(info.Name(), fileSuffix) {
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			d := yaml.NewDecoder(f)
			for {
				o := GenericObject{}
				err = d.Decode(&o)
				if err == io.EOF {
					return nil
				}
				if err != nil {
					return err
				}
				o2 := v1.PartialObjectMetadata{}
				o2.Kind = o.Kind
				o2.APIVersion = o.APIVersion
				o2.Name = o.Name
				o2.Namespace = o.Namespace

				ret = append(ret, &o2)
			}
		}
		return nil
	})

	return ret, err
}
