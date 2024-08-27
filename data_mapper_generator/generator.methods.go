package data_mapper_generator

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (g *DataMapperGenerator) generateObjectMethodsImports(o *ObjectType) {
	requiredImports := []string{
		"clearly-not-a-secret-project/interfaces",
		"clearly-not-a-secret-project/lazy_loading",
		"fmt",
		"reflect",
	}
	if o.Lazy && o.Pkg != g.config.RootPkg {
		dsPkgPath := fmt.Sprintf("%s/%s", filepath.Base(g.caller), g.config.RootDir)
		requiredImports = append(requiredImports, dsPkgPath)
	}
	g.wln("import (")
	for _, v := range requiredImports {
		g.wln(fmt.Sprintf("\"%s\"", v))
	}
	g.wln(")")
}

func (g *DataMapperGenerator) generateGhostImpl(o *ObjectType) {
	index := -1
	for i := range o.ValidatedFields {
		if *o.ValidatedFields[i].name == "id" {
			index = i
		}
	}
	if index < 0 {
		panic(fmt.Errorf("could not find id field in the validated fields"))
	}
	idField := o.ValidatedFields[index]
	g.wln(fmt.Sprintf(`
		func Create%sGhost(id %s) interfaces.DomainObject[%s] {
			return &%s{
				id: id,
				loadStatus: lazy_loading.GHOST,
			}
		}
		`, o.Name, *idField.dataType, *idField.dataType, o.Name))
	g.wln(fmt.Sprintf(`
		func (o *%s) load() {
			if o.IsGhost() {
				err := %s.Load(o)
				panic(fmt.Errorf("error at domain load %%w",err))
			}
		}
		`, o.Name, g.config.RootPkg))
	g.wln(fmt.Sprintf(`
		func (o *%s) Type() reflect.Type {
			return reflect.TypeOf(o)
		}
		`, o.Name))
	g.wln(fmt.Sprintf(`
		func (o %s) IsGhost() bool {
			return o.loadStatus == lazy_loading.GHOST
		}
		`, o.Name))
	g.wln(fmt.Sprintf(`
		func (o %s) IsLoaded() bool {
			return o.loadStatus == lazy_loading.LOADED
		}
		`, o.Name))
	g.wln(fmt.Sprintf(`
		func (o *%s) MarkLoading() error {
			if !o.IsGhost() {
				return fmt.Errorf("assertion error: to change the status to loading it has to be in status ghost")
			}
			o.loadStatus = lazy_loading.LOADING
			return nil
		}
		`, o.Name))
	g.wln(fmt.Sprintf(`
		func (o *%s) MarkLoaded() error {
			if o.loadStatus != lazy_loading.LOADING {
				return fmt.Errorf("assertion error: to change the status to loaded it has to be in status loading")
			}
			o.loadStatus = lazy_loading.LOADED
			return nil
		}
		`, o.Name))

}

func (g *DataMapperGenerator) generateObjectMethods(o *ObjectType) error {
	g.buff.Reset()
	pkg := g.generateNewPkg(o.Dir, o.Pkg)
	g.generateObjectMethodsImports(o)
	if o.Lazy {
		g.generateGhostImpl(o)
	}
	for _, v := range o.ValidatedFields {
		n := matchFirstCh.ReplaceAllStringFunc(*v.name, strings.ToUpper)
		g.wln(fmt.Sprintf(`
			func (o %s) %s()%s {
		`, o.Name, n, *v.dataType))
		if *v.name != "id" && o.Lazy {
			g.wln("o.load()")
		}
		g.wln(fmt.Sprintf(`
			return o.%s
		}`, *v.name))
		if *v.name != "id" {
			g.wln(fmt.Sprintf(`
			func (o *%s) Set%s(%s %s) {
				o.%s = %s
			}
		`, o.Name, n, *v.name, *v.dataType, *v.name, *v.name))
		}
	}
	err := g.writeFile(pkg, o.Name, "", "generated")
	if err != nil {
		return err
	}
	return nil
}
