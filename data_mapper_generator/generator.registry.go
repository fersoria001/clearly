package data_mapper_generator

import (
	"fmt"
)

func (g *DataMapperGenerator) generateDataMapperRegistryImports() {
	requiredImports := []string{
		"clearly-not-a-secret-project/registry",
		"fmt",
		"reflect",
		"sync",
	}
	g.wln("import (")
	for _, v := range requiredImports {
		g.wln(fmt.Sprintf("\"%s\"", v))
	}
	g.wln(")")
}

func (g *DataMapperGenerator) generateDataMapperRegistry() error {
	g.buff.Reset()
	pkg := g.generateNewPkg(generatedRegistryPkg, generatedRegistryPkg)
	g.generateDataMapperRegistryImports()
	instances := make(map[string]bool, 0)
	for _, v := range g.config.Objects {
		index := -1
		for i := range v.ValidatedFields {
			if *v.ValidatedFields[i].name == "id" {
				index = i
			}
		}
		if index < 0 {
			return fmt.Errorf("could not find id field in the validated fields %v", v)
		}
		idField := v.ValidatedFields[index]
		if _, ok := instances[*idField.dataType]; !ok {
			instances[*idField.dataType] = true
			g.wln(fmt.Sprintf(
				`var %sIdRegistry *registry.Registry[%s]`, *idField.dataType,
				*idField.dataType))
			g.wln(fmt.Sprintf(
				`var %sIdRegistryOnce sync.Once`, *idField.dataType,
			))
			g.wln(fmt.Sprintf(
				`func %sInstance() *registry.Registry[%s] {
					%sIdRegistryOnce.Do(func() {
						%sIdRegistry = registry.New[%s]()
			})
						return %sIdRegistry
				}`, *idField.dataType, *idField.dataType,
				*idField.dataType, *idField.dataType, *idField.dataType,
				*idField.dataType,
			))
		}
	}
	g.wln("var registries = registry.Registries{")
	for k := range instances {
		switch k {
		case "string":
			{
				g.wln(fmt.Sprintf("reflect.TypeOf(\"%v\"): %sInstance(),", randString(1), k))
			}
		}
	}
	g.wln("}")

	g.wln(`
	func Instance[K comparable]() (*registry.Registry[K],error) {
	 var zero[0]K
	 t:= reflect.TypeOf(zero).Elem()
	 r, ok := registries[t]
	 if !ok {
	 	return nil, fmt.Errorf("the registry for id type %s is not registered", t.Name())
	 }
	 instance, ok := r.(*registry.Registry[K])
	 if !ok {
	 	return nil, fmt.Errorf("could not cast the recovered registry to the given type")
	 	}
	return instance, nil
	}
	`)
	err := g.writeFile(pkg, "generated", "", "registry")
	if err != nil {
		return err
	}
	return nil
}
